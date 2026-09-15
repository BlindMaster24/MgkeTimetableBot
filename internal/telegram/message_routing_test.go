package telegram

import (
	"context"
	"encoding/json"
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
