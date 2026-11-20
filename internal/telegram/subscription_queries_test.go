package telegram

import (
	"testing"
)

func openTestRepo(t *testing.T) *Repository {
	t.Helper()

	repo, err := New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

func subscribedChat(t *testing.T, repo *Repository, userID int64, subs ...[2]string) {
	t.Helper()

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Accepted = true
	chat.AllowSendMess = true
	chat.NoticeChanges = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}
	for _, sub := range subs {
		if _, err := repo.AddSubscription(userID, sub[0], sub[1]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindSubscribedChatsByTeacherFollowsTheFlags(t *testing.T) {
	repo := openTestRepo(t)

	subscribedChat(t, repo, 7001, [2]string{"teacher", "Иванов И.И."})
	subscribedChat(t, repo, 7002, [2]string{"teacher", "Иванов И.И."})
	subscribedChat(t, repo, 7003, [2]string{"teacher", "Петров П.П."})

	// 7002 turned change notices off.
	quiet, err := repo.FindOrCreate("telegram", 7002)
	if err != nil {
		t.Fatal(err)
	}
	quiet.NoticeChanges = false
	if err := repo.Save(quiet); err != nil {
		t.Fatal(err)
	}

	withNotice, err := repo.FindSubscribedChatsByTeacher("telegram", "Иванов И.И.", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withNotice) != 1 || withNotice[0].PeerID != 7001 {
		t.Fatalf("notice chats = %+v, want only 7001", withNotice)
	}

	everyone, err := repo.FindSubscribedChatsByTeacher("telegram", "Иванов И.И.", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(everyone) != 2 {
		t.Fatalf("chats = %d, want both subscribers", len(everyone))
	}

	if none, err := repo.FindSubscribedChatsByTeacher("telegram", "Сидоров С.С.", false); err != nil || len(none) != 0 {
		t.Fatalf("an unknown teacher must have no subscribers: %v, %v", none, err)
	}
}

func TestFindChatsByTeachersNeedsAList(t *testing.T) {
	repo := openTestRepo(t)

	if chats, err := repo.FindChatsByTeachers("telegram", nil, true); err != nil || chats != nil {
		t.Fatalf("an empty list must short-circuit: %v, %v", chats, err)
	}

	// A teacher-mode chat owns the timetable itself; a subscription is not enough.
	teacherChat, err := repo.FindOrCreate("telegram", 7101)
	if err != nil {
		t.Fatal(err)
	}
	teacherChat.Mode = ModeTeacher
	teacherChat.Teacher = "Иванов И.И."
	teacherChat.Accepted = true
	teacherChat.AllowSendMess = true
	teacherChat.NoticeChanges = true
	if err := repo.Save(teacherChat); err != nil {
		t.Fatal(err)
	}
	subscribedChat(t, repo, 7102, [2]string{"teacher", "Иванов И.И."})

	chats, err := repo.FindChatsByTeachers("telegram", []string{"Иванов И.И.", "Петров П.П."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].PeerID != 7101 {
		t.Fatalf("chats = %+v, want 7101 once", chats)
	}
}

func TestRemoveSubscriptionOnlyTouchesItsOwner(t *testing.T) {
	repo := openTestRepo(t)
	subscribedChat(t, repo, 7201, [2]string{"group", "100"})
	subscribedChat(t, repo, 7202, [2]string{"group", "200"})

	subs, err := repo.GetSubscriptions(7201)
	if err != nil || len(subs) != 1 {
		t.Fatalf("subscriptions = %+v, %v", subs, err)
	}

	// Another chat cannot delete it by id.
	if err := repo.RemoveSubscription(7202, subs[0].ID); err != nil {
		t.Fatal(err)
	}
	if left, _ := repo.GetSubscriptions(7201); len(left) != 1 {
		t.Fatalf("a stranger removed the subscription: %+v", left)
	}

	if err := repo.RemoveSubscription(7201, subs[0].ID); err != nil {
		t.Fatal(err)
	}
	if left, _ := repo.GetSubscriptions(7201); len(left) != 0 {
		t.Fatalf("the owner could not remove it: %+v", left)
	}
}

func TestSetTextFormatStoresTheChatChoice(t *testing.T) {
	repo := openTestRepo(t)
	chat, err := repo.FindOrCreate("telegram", 7301)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SetTextFormat("telegram", 7301, FormatMarkdown); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.FindOrCreate("telegram", 7301)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TextFormat != FormatMarkdown {
		t.Fatalf("text format = %q, want %q", updated.TextFormat, FormatMarkdown)
	}
	if chat.PeerID != updated.PeerID {
		t.Fatal("the same chat must come back")
	}
}

func TestSubscribedNoticesSurviveARowIDThatDiffersFromThePeerID(t *testing.T) {
	repo := openTestRepo(t)

	// A chat row whose primary key is not its Telegram id: the exact case the
	// subscription join has to survive.
	subscribedChat(t, repo, 9001, [2]string{"group", "300"})
	subscribedChat(t, repo, 9002, [2]string{"group", "300"})
	subscribedChat(t, repo, 9003, [2]string{"group", "300"})

	chat, err := repo.FindOrCreate("telegram", 9003)
	if err != nil {
		t.Fatal(err)
	}
	if chat.ID == chat.PeerID {
		t.Fatalf("the test needs a row id that differs from the peer id, both are %d", chat.ID)
	}

	chats, err := repo.FindSubscribedChatsByGroup("telegram", "300", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 3 {
		t.Fatalf("chats = %d, want every subscriber", len(chats))
	}
	count, err := repo.CountSubscriptionsByType("telegram", "group")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
}
