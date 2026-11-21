package telegram

import (
	"context"
	"strings"
	"testing"
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
