package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func (c *capturingCaller) textsFor(chatID int64) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var sent []string
	for _, message := range c.messages {
		if message.chatID == chatID {
			sent = append(sent, message.text)
		}
	}
	return sent
}

func groupDays(group string, days ...map[string]any) map[string]any {
	list := make([]any, 0, len(days))
	for _, day := range days {
		list = append(list, day)
	}
	return map[string]any{"group": group, "days": list}
}

func lessonDay(date string, lessons ...string) map[string]any {
	list := make([]any, 0, len(lessons))
	for _, name := range lessons {
		list = append(list, map[string]any{"lesson": name, "type": "Лек", "cabinet": "101"})
	}
	return map[string]any{"day": date, "lessons": list}
}

func noticeNotifier(b *Bot, repo *Repository) *notification.EventNotifier {
	return notification.NewEventNotifier(b.cache, b.cfg, b.log, b, NewEventChatFinder(repo, b.cfg.Telegram.AdminIDs))
}

func TestNextWeekNoticeReachesTheSubscribedChats(t *testing.T) {
	const subscriber = int64(9201)
	const quiet = int64(9202)

	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	repo.SetDefaultAccepted(true)

	subscribe := func(userID int64, nextWeek bool) {
		t.Helper()

		chat, err := repo.FindOrCreate("telegram", userID)
		if err != nil {
			t.Fatal(err)
		}
		chat.Mode = ModeStudent
		chat.Group = "100"
		chat.NoticeNextWeek = nextWeek
		if err := repo.Save(chat); err != nil {
			t.Fatal(err)
		}
	}
	subscribe(subscriber, true)
	subscribe(quiet, false)

	notifier := noticeNotifier(b, repo)

	current := utils.WeekIndexFromDate(time.Now())
	next := current.Next()
	currentDay := current.FirstDayDate().Format("02.01.2006")
	nextDay := next.FirstDayDate().Format("02.01.2006")

	b.cache.SetGroups(map[string]any{"100": groupDays("100", lessonDay(currentDay, "Математика"))}, "week-1")
	b.cache.DrainEvents()

	b.cache.SetGroups(map[string]any{"100": groupDays("100", lessonDay(currentDay, "Математика"), lessonDay(nextDay, "Физика"))}, "week-2")
	events := b.cache.DrainEvents()
	if len(events) == 0 {
		t.Fatal("a new published week must reach the event bus")
	}

	notifier.HandleEvents(events)

	sent := caller.textsFor(subscriber)
	if len(sent) != 1 {
		t.Fatalf("the subscriber must get exactly one notice, got %q", sent)
	}
	if !strings.Contains(sent[0], "Доступно расписание на следующую неделю") {
		t.Fatalf("unexpected notice: %q", sent[0])
	}
	if quietTexts := caller.textsFor(quiet); len(quietTexts) != 0 {
		t.Fatalf("a chat with the toggle off must stay quiet, got %q", quietTexts)
	}
}

func TestTriggerRunsTheDayNoticeThroughTheWiredFunction(t *testing.T) {
	const admin = int64(9203)
	const subscriber = int64(9204)

	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, admin)
	repo.SetDefaultAccepted(true)

	chat, err := repo.FindOrCreate("telegram", subscriber)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.NoticeChanges = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)
	b.cache.SetGroups(map[string]any{"100": groupDays("100",
		lessonDay(today.Format("02.01.2006"), "Математика", "Физика", "Информатика"),
		lessonDay(tomorrow.Format("02.01.2006"), "Химия"),
	)}, "day-1")
	b.cache.DrainEvents()

	notifier := noticeNotifier(b, repo)
	b.SetNoticeDayFunc(notifier.CronDayAll)

	sendMessage(t, b, admin, "private", "/trigger NextDayUpdater 3")

	if caller.last() != "ok" {
		t.Fatalf("the trigger must answer ok, got %q", caller.last())
	}

	sent := caller.textsFor(subscriber)
	if len(sent) != 1 {
		t.Fatalf("the subscriber must get one day notice, got %q", sent)
	}
	if !strings.Contains(sent[0], "Химия") {
		t.Fatalf("the notice must describe the next day with lessons: %q", sent[0])
	}

	notifier.CronDayAll(2)
	if again := caller.textsFor(subscriber); len(again) != 1 {
		t.Fatalf("the same day must not be noticed twice, got %q", again)
	}
}

func TestDayNoticeWorksInEveryTimeZone(t *testing.T) {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("Europe/Minsk", 3*60*60),
		time.FixedZone("America/Chicago", -5*60*60),
		time.FixedZone("Pacific/Kiritimati", 14*60*60),
		time.FixedZone("Pacific/Midway", -11*60*60),
	}

	for _, zone := range zones {
		t.Run(zone.String(), func(t *testing.T) {
			now := time.Date(2026, time.September, 16, 10, 0, 0, 0, zone)

			const userID = int64(9207)
			caller := &capturingCaller{}
			b, repo := setupE2EBotWithCaller(t, caller)
			repo.SetDefaultAccepted(true)

			chat, err := repo.FindOrCreate("telegram", userID)
			if err != nil {
				t.Fatal(err)
			}
			chat.Mode = ModeStudent
			chat.Group = "100"
			chat.NoticeChanges = true
			chat.NoticeNextWeek = true
			if err := repo.Save(chat); err != nil {
				t.Fatal(err)
			}

			b.cache.SetGroups(map[string]any{"100": groupDays("100",
				lessonDay(now.Format("02.01.2006"), "Математика", "Физика", "Информатика"),
				lessonDay(now.AddDate(0, 0, 1).Format("02.01.2006"), "Химия"),
			)}, "day-zone")
			b.cache.DrainEvents()

			notifier := noticeNotifier(b, repo)
			notifier.SetNow(func() time.Time { return now })
			notifier.CronDayAll(2)

			sent := caller.textsFor(userID)
			if len(sent) != 1 || !strings.Contains(sent[0], "Химия") {
				t.Fatalf("the day notice must reach the chat in %s, got %q", zone, sent)
			}

			nextWeekDay := utils.WeekIndexFromDate(now.AddDate(0, 0, 1)).Next().FirstDayDate()
			b.cache.SetGroups(map[string]any{"100": groupDays("100",
				lessonDay(now.Format("02.01.2006"), "Математика", "Физика", "Информатика"),
				lessonDay(now.AddDate(0, 0, 1).Format("02.01.2006"), "Химия"),
				lessonDay(nextWeekDay.Format("02.01.2006"), "Астрономия"),
			)}, "week-zone")
			caller.reset()
			notifier.HandleEvents(b.cache.DrainEvents())

			week := caller.textsFor(userID)
			if len(week) != 1 || !strings.Contains(week[0], "Доступно расписание на следующую неделю") {
				t.Fatalf("the week notice must reach the chat in %s, got %q", zone, week)
			}
		})
	}
}

func TestTriggerStaysQuietWhenNoDayMatches(t *testing.T) {
	const admin = int64(9205)
	const subscriber = int64(9206)

	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, admin)
	repo.SetDefaultAccepted(true)

	chat, err := repo.FindOrCreate("telegram", subscriber)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.NoticeChanges = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	notifier := noticeNotifier(b, repo)
	b.SetNoticeDayFunc(notifier.CronDayAll)

	sendMessage(t, b, admin, "private", "/trigger NextDayUpdater 3")

	if caller.last() != "ok" {
		t.Fatalf("the trigger must answer ok, got %q", caller.last())
	}
	if sent := caller.textsFor(subscriber); len(sent) != 0 {
		t.Fatalf("a day without a match must stay quiet, got %q", sent)
	}
}
