package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
)

func TestTheFinderMapsTheTeacherFieldsForNotifications(t *testing.T) {
	repo, err := New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	target, err := repo.FindOrCreate("telegram", 5001)
	if err != nil {
		t.Fatal(err)
	}
	target.Mode = ModeTeacher
	target.Teacher = "Иванов И.И."
	target.NoticeChanges = true
	target.NoticeNextWeek = false
	target.CallsCampus = "Кн"
	target.ReminderLeadMinutes = 30
	target.Formatter = 2
	if err := repo.Save(target); err != nil {
		t.Fatal(err)
	}
	other, err := repo.FindOrCreate("telegram", 5002)
	if err != nil {
		t.Fatal(err)
	}
	other.Mode = ModeTeacher
	other.Teacher = "Петров П.П."
	if err := repo.Save(other); err != nil {
		t.Fatal(err)
	}

	finder := NewEventChatFinder(repo, nil)
	chats, err := finder.FindChatsByTeachers("telegram", []string{"Иванов И.И."}, true)
	if err != nil {
		t.Fatalf("FindChatsByTeachers: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("chats = %d, want only the teacher that matches", len(chats))
	}
	chat := chats[0]
	if chat.PeerID != 5001 || chat.Mode != "teacher" || chat.Teacher != "Иванов И.И." {
		t.Fatalf("chat = %+v, want the mapped teacher chat", chat)
	}
	if !chat.NoticeChanges || chat.NoticeNextWeek || chat.CallsCampus != "Кн" || chat.ReminderLeadMinutes != 30 || chat.Formatter != 2 {
		t.Fatalf("chat = %+v, the finder dropped a setting the notifier needs", chat)
	}
}

func TestTheFinderListsSecondarySubscriptions(t *testing.T) {
	repo, err := New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	owner, err := repo.FindOrCreate("telegram", 6001)
	if err != nil {
		t.Fatal(err)
	}
	owner.Mode = ModeStudent
	owner.Group = "100"
	if err := repo.Save(owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddSubscription(6001, "teacher", "Сидорова А.А."); err != nil {
		t.Fatal(err)
	}

	finder := NewEventChatFinder(repo, []int64{9001})
	subs, err := finder.FindSubscribedChatsByTeacher("telegram", "Сидорова А.А.", true)
	if err != nil {
		t.Fatalf("FindSubscribedChatsByTeacher: %v", err)
	}
	if len(subs) != 1 || subs[0].PeerID != 6001 {
		t.Fatalf("subs = %+v, want the chat that subscribed to the teacher", subs)
	}

	admins, err := finder.FindAdminChats("telegram")
	if err != nil {
		t.Fatalf("FindAdminChats: %v", err)
	}
	if len(admins) != 0 {
		t.Fatalf("admins = %+v, want none: 9001 has no chat row", admins)
	}
}

func TestIDCommandReportsTheIdentifiers(t *testing.T) {
	bot, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	cmd := &idCmd{bot: bot}
	u := &Update{UserID: 6409603928, ChatID: 6409603928, Bot: bot}
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("/id: %v", err)
	}
	text := caller.last()
	for _, want := range []string{"chat_id:", "peer_id: 6409603928", "user_id: 6409603928"} {
		if !strings.Contains(text, want) {
			t.Errorf("/id answer does not contain %q: %q", want, text)
		}
	}
}

// TestEveryFinderCarriesTheWholeEventChat guards the class of bug where a new
// column is added to the notification view but the SELECT list is not: each
// finder must return every field the notifier reads.
func TestEveryFinderCarriesTheWholeEventChat(t *testing.T) {
	repo, err := New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	fill := func(peer int64, mode ChatMode, group, teacher string) int64 {
		t.Helper()
		chat, err := repo.FindOrCreate("telegram", peer)
		if err != nil {
			t.Fatal(err)
		}
		chat.Mode = mode
		chat.Group = group
		chat.Teacher = teacher
		chat.NoticeChanges = true
		chat.NoticeNextWeek = true
		chat.NoticeCalls = true
		chat.NoticeParserErrors = true
		chat.AllowSendMess = true
		chat.ReminderLeadMinutes = 15
		chat.CallsCampus = "Кн"
		chat.Formatter = 3
		chat.HidePastDays = true
		chat.ShowHints = true
		chat.ShowParserTime = true
		if err := repo.Save(chat); err != nil {
			t.Fatal(err)
		}
		return chat.PeerID
	}
	student := fill(7001, ModeStudent, "100", "Иванов И.И.")
	teacher := fill(7002, ModeTeacher, "", "Иванов И.И.")
	if _, err := repo.AddSubscription(7001, "group", "100"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddSubscription(7001, "teacher", "Иванов И.И."); err != nil {
		t.Fatal(err)
	}

	finder := NewEventChatFinder(repo, []int64{7001})
	cases := []struct {
		name string
		want int64
		run  func() ([]*notification.EventChat, error)
	}{
		{"by groups", student, func() ([]*notification.EventChat, error) {
			return finder.FindChatsByGroups("telegram", []string{"100"}, true)
		}},
		{"by teachers", teacher, func() ([]*notification.EventChat, error) {
			return finder.FindChatsByTeachers("telegram", []string{"Иванов И.И."}, true)
		}},
		{"subscribed by group", student, func() ([]*notification.EventChat, error) {
			return finder.FindSubscribedChatsByGroup("telegram", "100", true)
		}},
		{"subscribed by teacher", student, func() ([]*notification.EventChat, error) {
			return finder.FindSubscribedChatsByTeacher("telegram", "Иванов И.И.", true)
		}},
		{"without the notice filter", student, func() ([]*notification.EventChat, error) {
			return finder.FindSubscribedChatsByTeacher("telegram", "Иванов И.И.", false)
		}},
		{"admins", student, func() ([]*notification.EventChat, error) {
			return finder.FindAdminChats("telegram")
		}},
	}
	for _, tc := range cases {
		found, err := tc.run()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(found) != 1 {
			t.Fatalf("%s returned %d chats, want 1", tc.name, len(found))
		}
		got := found[0]
		if got.PeerID != tc.want {
			t.Fatalf("%s returned peer %d, want %d", tc.name, got.PeerID, tc.want)
		}
		if got.CallsCampus != "Кн" || got.ReminderLeadMinutes != 15 || got.Formatter != 3 ||
			!got.NoticeCalls || !got.ShowParserTime || !got.HidePastDays || !got.AllowSendMess {
			t.Fatalf("%s dropped a field: %+v", tc.name, got)
		}
	}
}
