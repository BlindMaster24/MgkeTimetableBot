package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func recordedMethods(caller *recordingCaller) []string {
	caller.mu.Lock()
	defer caller.mu.Unlock()

	methods := make([]string, 0, len(caller.calls))
	for _, call := range caller.calls {
		methods = append(methods, call.Method)
	}
	return methods
}

func recordedCalls(caller *recordingCaller, method string) []map[string]any {
	caller.mu.Lock()
	defer caller.mu.Unlock()

	var found []map[string]any
	for _, call := range caller.calls {
		if call.Method != method {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(call.Raw), &payload); err != nil {
			continue
		}
		found = append(found, payload)
	}
	return found
}

func chatWithGroup(t *testing.T, repo *Repository, userID int64, group string) *Chat {
	t.Helper()

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatalf("find chat: %v", err)
	}
	chat.Mode = ModeStudent
	chat.Group = group
	chat.LastMsgID = 99
	if err := repo.Save(chat); err != nil {
		t.Fatalf("save chat: %v", err)
	}
	return chat
}

func TestE2E_DayCommandSendsANewMessage(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	cmd := &dayCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: "/day"}); err != nil {
		t.Fatalf("day command: %v", err)
	}

	methods := recordedMethods(caller)
	if len(methods) == 0 {
		t.Fatal("the day command delivered nothing")
	}
	for _, method := range methods {
		if method == "editMessageText" {
			t.Fatalf("the day command edited an old message instead of sending a new one: %v", methods)
		}
	}
}

func TestE2E_WeekCommandSendsANewMessage(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	cmd := &weekCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: "/week"}); err != nil {
		t.Fatalf("week command: %v", err)
	}

	methods := recordedMethods(caller)
	if len(methods) == 0 {
		t.Fatal("the week command delivered nothing")
	}
	for _, method := range methods {
		if method == "editMessageText" {
			t.Fatalf("the week command edited an old message instead of sending a new one: %v", methods)
		}
	}
}

func TestE2E_TimetableArrowEditsItsOwnMessage(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, b.chatRepo, 4242, "100")

	data := "timetable_g:100:0:0:1"
	prefix, handler := b.findCallback(data)
	if handler == nil || prefix == "" {
		t.Fatalf("no handler for %q", data)
	}

	u := &Update{
		Bot:       b,
		ChatID:    4242,
		UserID:    4242,
		MessageID: 55,
		Data:      data,
		Callback: &telego.CallbackQuery{
			ID:      "cb",
			From:    telego.User{ID: 4242},
			Message: &telego.Message{Chat: telego.Chat{ID: 4242}, MessageID: 55},
		},
	}

	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("timetable callback: %v", err)
	}

	edits := recordedCalls(caller, "editMessageText")
	if len(edits) != 1 {
		t.Fatalf("expected exactly one edit of the pressed message, got %v", recordedMethods(caller))
	}
	if id, ok := edits[0]["message_id"].(float64); !ok || int(id) != 55 {
		t.Fatalf("edit targeted %v, want message 55", edits[0]["message_id"])
	}
	if len(recordedCalls(caller, "sendMessage")) != 0 {
		t.Error("inline navigation must not send an extra message")
	}
}

func TestE2E_TeacherButtonLabelsAllOpenTheFlow(t *testing.T) {
	labels := []string{
		"👩‍🏫 Преподаватель",
		"👩‍🏫 Препод.",
		"Преподаватель",
		"Преподаватель День",
		"Препод.",
		"Препод",
		"Учитель День",
	}

	for _, label := range labels {
		caller := &recordingCaller{}
		b, repo := setupE2EBotWithCaller(t, caller, 4242)
		chatWithGroup(t, repo, 4242, "100")

		b.handleMessageText(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: label})

		chat, err := repo.FindOrCreate("telegram", 4242)
		if err != nil {
			t.Fatalf("find chat: %v", err)
		}
		if !strings.HasPrefix(chat.Scene, sceneGetTeacher) {
			t.Errorf("%q did not open the teacher flow, scene is %q", label, chat.Scene)
		}
		if text := caller.deliveredText(); !strings.Contains(text, "фамилию") {
			t.Errorf("%q replied with %q", label, text)
		}
	}
}

func TestE2E_GroupButtonLabelsAllOpenTheFlow(t *testing.T) {
	labels := []string{"👩‍🎓 Группа", "Группа", "Группа День"}

	for _, label := range labels {
		caller := &recordingCaller{}
		b, repo := setupE2EBotWithCaller(t, caller, 4242)
		chatWithGroup(t, repo, 4242, "100")

		b.handleMessageText(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: label})

		chat, err := repo.FindOrCreate("telegram", 4242)
		if err != nil {
			t.Fatalf("find chat: %v", err)
		}
		if !strings.HasPrefix(chat.Scene, sceneGetGroup) {
			t.Errorf("%q did not open the group flow, scene is %q", label, chat.Scene)
		}
		if text := caller.deliveredText(); !strings.Contains(text, "группы") {
			t.Errorf("%q replied with %q", label, text)
		}
	}
}

func TestE2E_WeekAndImageLabelsCarryTheirKind(t *testing.T) {
	cases := []struct {
		label string
		scene string
		kind  string
	}{
		{"👩‍🏫 Препод. Неделя", sceneGetTeacher, "week"},
		{"Преподаватель Неделя", sceneGetTeacher, "week"},
		{"Учитель Неделя", sceneGetTeacher, "week"},
		{"👩‍🎓 Группа Неделя", sceneGetGroup, "week"},
		{"Группа Неделя", sceneGetGroup, "week"},
		{"👩‍🏫 Преподаватель таблица", sceneGetTeacher, "image"},
		{"Преподаватель фотография", sceneGetTeacher, "image"},
		{"Учительфотография", sceneGetTeacher, "image"},
		{"Группа таблица", sceneGetGroup, "image"},
		{"Группафото", sceneGetGroup, "image"},
	}

	for _, tc := range cases {
		caller := &recordingCaller{}
		b, repo := setupE2EBotWithCaller(t, caller, 4242)
		chatWithGroup(t, repo, 4242, "100")

		b.handleMessageText(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: tc.label})

		chat, err := repo.FindOrCreate("telegram", 4242)
		if err != nil {
			t.Fatalf("find chat: %v", err)
		}
		if chat.Scene != tc.scene+":"+tc.kind {
			t.Errorf("%q opened scene %q, want %q", tc.label, chat.Scene, tc.scene+":"+tc.kind)
		}
	}
}

func TestE2E_EveryMainMenuButtonHasAHandler(t *testing.T) {
	cases := []struct {
		name                                                 string
		mode                                                 ChatMode
		showAbout, showCalls, showFastGroup, showFastTeacher bool
	}{
		{"student with every button", ModeStudent, true, true, true, true},
		{"student with calls only", ModeStudent, true, false, false, false},
		{"student with defaults", ModeStudent, false, true, true, false},
		{"teacher", ModeTeacher, true, true, true, true},
		{"parent", ModeParent, false, false, false, false},
		{"guest", ModeGuest, true, true, true, true},
		{"not configured yet", "", true, true, true, true},
	}

	for _, tc := range cases {
		caller := &recordingCaller{}
		b, repo := setupE2EBotWithCaller(t, caller, 4242)

		chat, err := repo.FindOrCreate("telegram", 4242)
		if err != nil {
			t.Fatalf("find chat: %v", err)
		}
		chat.Mode = tc.mode
		chat.Group = "100"
		chat.Teacher = "Иванов И.И."
		chat.ShowAbout = tc.showAbout
		chat.ShowCalls = tc.showCalls
		chat.ShowFastGroup = tc.showFastGroup
		chat.ShowFastTeacher = tc.showFastTeacher
		if err := repo.Save(chat); err != nil {
			t.Fatalf("save chat: %v", err)
		}

		keyboard := replyMainMenu(b, chat)
		for _, row := range keyboard.Keyboard {
			for _, button := range row {
				chat.Scene = ""
				u := &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: button.Text}
				if !b.dispatchTextCommand(context.Background(), u, chat) {
					t.Errorf("%s: the main menu button %q has no handler", tc.name, button.Text)
				}
			}
		}
	}
}

func TestE2E_SlashCommandsIgnoreCase(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	checked := 0
	for _, cmd := range b.commandOrder {
		name := strings.TrimPrefix(cmd.Name(), "/")
		if name == "" {
			continue
		}
		checked++
		for _, variant := range []string{strings.ToUpper(name), name, strings.ToUpper(name[:1]) + name[1:]} {
			found := b.commandByName(variant)
			if found == nil || found.Name() != cmd.Name() {
				t.Errorf("/%s does not resolve to %s", variant, cmd.Name())
			}
		}
	}

	if checked == 0 {
		t.Fatal("no commands were checked")
	}

	chat, err := repo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatalf("find chat: %v", err)
	}
	for _, text := range []string{"/day", "/DAY", "/Day"} {
		chat.Scene = ""
		u := &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: text}
		if !b.dispatchTextCommand(context.Background(), u, chat) {
			t.Errorf("%q was not routed", text)
		}
	}
}

func TestE2E_HamburgerCallbackUsesTheCallbackMessage(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	b.handleCallback(context.Background(), &telego.CallbackQuery{
		ID:      "cb",
		From:    telego.User{ID: 4242},
		Data:    "calls_full",
		Message: &telego.Message{Chat: telego.Chat{ID: 4242}, MessageID: 71},
	})

	edits := recordedCalls(caller, "editMessageText")
	if len(edits) != 1 {
		t.Fatalf("expected the calls message to be edited in place, got %v", recordedMethods(caller))
	}
	if id, ok := edits[0]["message_id"].(float64); !ok || int(id) != 71 {
		t.Fatalf("edit targeted %v, want message 71", edits[0]["message_id"])
	}
}
