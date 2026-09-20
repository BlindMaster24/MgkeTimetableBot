package telegram

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
)

func TestEventChatFinderKeepsEveryNoticeFlag(t *testing.T) {
	const userID = int64(9001)
	const adminID = int64(9002)

	_, repo := setupE2EBot(t, adminID)
	repo.SetDefaultAccepted(true)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Teacher = "Иванов И.И."
	chat.Formatter = 2
	chat.NoticeChanges = true
	chat.NoticeNextWeek = true
	chat.NoticeCalls = true
	chat.NoticeParserErrors = true
	chat.AllowSendMess = true
	chat.HidePastDays = true
	chat.ShowHints = true
	chat.ShowParserTime = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindOrCreate("telegram", adminID); err != nil {
		t.Fatal(err)
	}

	finder := NewEventChatFinder(repo, []int64{adminID})

	byGroup, err := finder.FindChatsByGroups("telegram", []string{"100"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(byGroup) != 1 {
		t.Fatalf("expected one chat, got %d", len(byGroup))
	}

	found := byGroup[0]
	if found.ID != chat.ID || found.PeerID != userID {
		t.Fatalf("unexpected identifiers: %+v", found)
	}
	if found.Mode != string(ModeStudent) || found.Group != "100" || found.Teacher != "Иванов И.И." {
		t.Fatalf("unexpected scope: %+v", found)
	}
	if found.Formatter != 2 {
		t.Fatalf("formatter = %d, want 2", found.Formatter)
	}
	if !found.NoticeChanges || !found.NoticeNextWeek || !found.NoticeCalls || !found.NoticeParserErrs {
		t.Fatalf("the notice flags must survive the adapter: %+v", found)
	}
	if !found.AllowSendMess || !found.HidePastDays || !found.ShowHints || !found.ShowParserTime {
		t.Fatalf("the display flags must survive the adapter: %+v", found)
	}

	byNotice, err := finder.FindChatsWithNotice("telegram", "notice_calls")
	if err != nil {
		t.Fatal(err)
	}
	found = nil
	for _, c := range byNotice {
		if c.PeerID == userID {
			found = c
		}
	}
	if found == nil || !found.NoticeCalls || found.Group != "100" {
		t.Fatalf("the calls toggle must be reported for the chat: %+v", byNotice)
	}

	admins, err := finder.FindAdminChats("telegram")
	if err != nil {
		t.Fatal(err)
	}
	if len(admins) != 1 || admins[0].PeerID != adminID {
		t.Fatalf("the admin chat must be found: %+v", admins)
	}
}

func TestEventChatFinderMatchesTheNotifierContract(t *testing.T) {
	const userID = int64(9003)

	_, repo := setupE2EBot(t)
	repo.SetDefaultAccepted(true)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.NoticeNextWeek = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddSubscription(chat.ID, "group", "100"); err != nil {
		t.Fatal(err)
	}

	var finder notification.EventChatFinder = NewEventChatFinder(repo, nil)

	chats, err := finder.FindSubscribedChatsByGroup("telegram", "100", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || !chats[0].NoticeNextWeek {
		t.Fatalf("the subscription lookup must keep the next week toggle: %+v", chats)
	}
}
