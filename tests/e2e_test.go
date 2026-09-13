package tests

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

type sentMessage struct {
	chatID int64
	text   string
}

type recordingSender struct {
	sent []sentMessage
}

func (s *recordingSender) SendText(chatID int64, text string) error {
	s.sent = append(s.sent, sentMessage{chatID: chatID, text: text})
	return nil
}

func (s *recordingSender) SendTextWithButtons(chatID int64, text string, buttons []notification.KeyboardButton) error {
	return s.SendText(chatID, text)
}

func (s *recordingSender) textsFor(chatID int64) []string {
	var texts []string
	for _, message := range s.sent {
		if message.chatID == chatID {
			texts = append(texts, message.text)
		}
	}
	return texts
}

type repoChatFinder struct {
	repo     *telegrambot.Repository
	adminIDs []int64
}

func eventChats(chats []*telegrambot.Chat) []*notification.EventChat {
	result := make([]*notification.EventChat, 0, len(chats))
	for _, chat := range chats {
		result = append(result, &notification.EventChat{
			ID:               chat.ID,
			PeerID:           chat.PeerID,
			Mode:             string(chat.Mode),
			Group:            chat.Group,
			Teacher:          chat.Teacher,
			NoticeChanges:    chat.NoticeChanges,
			NoticeNextWeek:   chat.NoticeNextWeek,
			NoticeCalls:      chat.NoticeCalls,
			NoticeParserErrs: chat.NoticeParserErrors,
			AllowSendMess:    chat.AllowSendMess,
			Formatter:        chat.Formatter,
		})
	}
	return result
}

func (f *repoChatFinder) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsByGroups(service, groups, noticeChanges)
	return eventChats(chats), err
}

func (f *repoChatFinder) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsByTeachers(service, teachers, noticeChanges)
	return eventChats(chats), err
}

func (f *repoChatFinder) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindSubscribedChatsByGroup(service, group, noticeChanges)
	return eventChats(chats), err
}

func (f *repoChatFinder) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindSubscribedChatsByTeacher(service, teacher, noticeChanges)
	return eventChats(chats), err
}

func (f *repoChatFinder) FindChatsWithNotice(service string, notice string) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindChatsWithNotice(service, notice)
	return eventChats(chats), err
}

func (f *repoChatFinder) FindAdminChats(service string) ([]*notification.EventChat, error) {
	chats, err := f.repo.FindAdminChats(service, f.adminIDs)
	return eventChats(chats), err
}

func newChat(t *testing.T, repo *telegrambot.Repository, peerID int64, group string, noticeChanges bool) *telegrambot.Chat {
	t.Helper()

	chat, err := repo.FindOrCreate("telegram", peerID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Accepted = true
	chat.AllowSendMess = true
	chat.Mode = telegrambot.ModeStudent
	chat.Group = group
	chat.NoticeChanges = noticeChanges
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}
	return chat
}

func groupDay(date string, lessons ...string) map[string]any {
	items := make([]any, 0, len(lessons))
	for _, lesson := range lessons {
		items = append(items, map[string]any{"lesson": lesson, "time": "08:00 - 08:45"})
	}
	return map[string]any{"day": date, "lessons": items}
}

func groupEntry(days ...map[string]any) map[string]any {
	items := make([]any, 0, len(days))
	for _, day := range days {
		items = append(items, day)
	}
	return map[string]any{"days": items}
}

func TestE2E_ChangedDayReachesOnlyInterestedChats(t *testing.T) {
	repo, err := telegrambot.New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	newChat(t, repo, 5001, "100", true)
	newChat(t, repo, 5002, "100", false)

	subscriber := newChat(t, repo, 5003, "999", true)
	if _, err := repo.AddSubscription(subscriber.ID, "group", "200"); err != nil {
		t.Fatal(err)
	}

	raspCache, err := cache.New(t.TempDir() + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	sender := &recordingSender{}
	finder := &repoChatFinder{repo: repo}
	notifier := notification.NewEventNotifier(raspCache, &config.Config{}, logger.New("error", nil), sender, finder)

	tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")

	raspCache.SetGroups(map[string]any{
		"100": groupEntry(groupDay(tomorrow, "Математика")),
		"200": groupEntry(groupDay(tomorrow, "Физика")),
	}, "hash-1")
	notifier.HandleEvents(raspCache.DrainEvents())
	if len(sender.sent) != 0 {
		t.Fatalf("the first parse must not notify, got %+v", sender.sent)
	}

	raspCache.SetGroups(map[string]any{
		"100": groupEntry(groupDay(tomorrow, "Математика", "Информатика")),
		"200": groupEntry(groupDay(tomorrow, "Физика", "Химия")),
	}, "hash-2")
	notifier.HandleEvents(raspCache.DrainEvents())

	if got := sender.textsFor(5001); len(got) != 1 {
		t.Errorf("the group owner must get one notice, got %v", got)
	} else if !strings.Contains(got[0], "Информатика") {
		t.Errorf("notice must mention the new lesson: %q", got[0])
	}

	if got := sender.textsFor(5003); len(got) != 1 {
		t.Errorf("the subscriber must get one notice, got %v", got)
	} else if !strings.Contains(got[0], "Химия") {
		t.Errorf("subscriber notice must mention the new lesson: %q", got[0])
	}

	if got := sender.textsFor(5002); len(got) != 0 {
		t.Errorf("a chat with the notices turned off must be skipped, got %v", got)
	}
}

func TestE2E_HealthAlertsSurviveARestart(t *testing.T) {
	repo, err := telegrambot.New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	admin := newChat(t, repo, 9001, "100", true)

	thresholds := health.DefaultThresholds()
	thresholds.ParserFailures = 1
	finder := &repoChatFinder{repo: repo, adminIDs: []int64{admin.PeerID}}
	log := logger.New("error", nil)

	sender := &recordingSender{}
	tracker := health.NewTracker(thresholds)
	if err := tracker.Restore(repo); err != nil {
		t.Fatal(err)
	}

	tracker.ParserFailure(errors.New("site down"))
	if err := tracker.Flush(repo); err != nil {
		t.Fatal(err)
	}

	notifier := notification.NewHealthNotifier(tracker, log, sender, finder, time.Hour, repo)
	notifier.Check()
	if got := sender.textsFor(admin.PeerID); len(got) != 1 {
		t.Fatalf("the admin must be told about the failure, got %v", got)
	}

	restartedTracker := health.NewTracker(thresholds)
	if err := restartedTracker.Restore(repo); err != nil {
		t.Fatal(err)
	}
	if runs := restartedTracker.Snapshot().Parser.Runs; runs != 1 {
		t.Fatalf("metrics must survive a restart, runs = %d", runs)
	}

	restartedNotifier := notification.NewHealthNotifier(restartedTracker, log, sender, finder, time.Hour, repo)
	restartedNotifier.Check()
	if got := sender.textsFor(admin.PeerID); len(got) != 1 {
		t.Errorf("a restart must not repeat an alert inside the cooldown, got %v", got)
	}

	restartedTracker.ParserSuccess(time.Millisecond)
	if err := restartedTracker.Flush(repo); err != nil {
		t.Fatal(err)
	}
	restartedNotifier.Check()

	got := sender.textsFor(admin.PeerID)
	if len(got) != 2 {
		t.Fatalf("expected a recovery message after the restart, got %v", got)
	}
	if !strings.Contains(got[1], "восстановлено") {
		t.Errorf("recovery text = %q", got[1])
	}
}
