package notification

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type failingEventSender struct {
	attempts int
}

func (s *failingEventSender) SendText(int64, string) error {
	s.attempts++
	return errors.New("telegram is down")
}

func (s *failingEventSender) SendTextWithButtons(int64, string, []KeyboardButton) error {
	s.attempts++
	return errors.New("telegram is down")
}

func TestEventNotifierCountsNotificationsPerKind(t *testing.T) {
	n, _, finder, c := newTestNotifier(t)
	counters := health.NewTelegramCounters()
	n.SetCounterSink(counters)

	now := reminderMoment()
	day := dayMap(now.Format("02.01.2006"), dayWith("Математика", "08:00 - 08:45"))

	n.sendDay(cache.KindGroups, "63", false, "сегодня", day, []*EventChat{{ID: 1, PeerID: 1001, Group: "63"}})
	n.sendDayUpdate(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: day}, "сегодня", []*EventChat{{ID: 1, PeerID: 1001, Group: "63"}})

	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", NoticeNextWeek: true}}
	c.SetGroups(t.Context(), map[string]any{
		"63": map[string]any{
			"days": []any{map[string]any{"day": futureWeekStr(), "lessons": []any{map[string]any{"lesson": "X"}}}},
		},
	}, "h")
	n.UpdateWeek(cache.KindGroups, utils.WeekIndexFromDate(now).Value()+1)

	finder.subsGroup = []*EventChat{{ID: 2, PeerID: 2002, NoticeNextWeek: true}}
	n.WithdrawWeek(cache.KindGroups, 99999, []string{"63"})

	finder.withNotice = []*EventChat{{ID: 3, PeerID: 3003, Group: "63"}}
	n.CallsChanged(&cache.CallsEvent{
		WeekdaysChanged: true,
		Schedule:        cache.CallsSchedule{Weekdays: [][2][2]string{{{"10:00", "10:30"}, {"10:40", "11:10"}}}},
	})

	finder.admins = []*EventChat{{ID: 4, PeerID: 4004}}
	n.ParserError(errors.New("boom"))

	finder.subsGroup = nil
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)
	n.RemindUpcoming(now, time.Hour, 0, nil)

	stats := counters.TelegramStats()
	want := map[string]int64{
		kindDayNotice:     1,
		kindDayChange:     1,
		kindNewWeek:       1,
		kindWeekWithdrawn: 2,
		kindCalls:         1,
		kindParserError:   2,
		kindReminder:      1,
	}
	if !maps.Equal(stats.Notifications, want) {
		t.Errorf("notifications = %v, want %v", stats.Notifications, want)
	}
	if len(stats.Commands) != 0 || len(stats.Callbacks) != 0 || len(stats.Transports) != 0 {
		t.Errorf("the notifier must only feed its own dictionary: %+v", stats)
	}
}

func TestHealthNotifierCountsItsAlerts(t *testing.T) {
	_, _, finder, _ := newTestNotifier(t)
	counters := health.NewTelegramCounters()
	sender := &mockEventSender{}

	notifier := NewHealthNotifier(health.NewDefaultTracker(), logger.New("error", nil), sender, finder, 0, nil)
	notifier.SetCounterSink(counters)

	notifier.broadcast([]*EventChat{{ID: 1}, {ID: 2, PeerID: 2002}}, "⚠️ parser_stale", []KeyboardButton{{Text: "🔄", Data: ParserReparseCallback}})

	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 alert messages, got %d", len(sender.sent))
	}
	stats := counters.TelegramStats()
	if !maps.Equal(stats.Notifications, map[string]int64{kindHealth: 2}) {
		t.Errorf("notifications = %v, want %v", stats.Notifications, map[string]int64{kindHealth: 2})
	}
}

func TestFailedSendsAreNotCountedAsNotifications(t *testing.T) {
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sender := &failingEventSender{}
	counters := health.NewTelegramCounters()

	n := NewEventNotifier(c, &config.Config{}, logger.New("error", nil), sender, &mockEventChatFinder{})
	n.SetCounterSink(counters)

	n.sendDay(cache.KindGroups, "63", false, "сегодня", dayMap(todayStr(), dayWith("Математика", "08:00 - 08:45")), []*EventChat{{ID: 1}})

	if sender.attempts != 1 {
		t.Fatalf("the send must be attempted once, got %d", sender.attempts)
	}
	if stats := counters.TelegramStats(); stats.Notifications != nil {
		t.Errorf("a failed send must not count as a notification: %v", stats.Notifications)
	}
}

func TestSchedulerCountsTheRemindersItSends(t *testing.T) {
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Timetable.ReminderLeadMinutes = 15
	sender := &mockEventSender{}
	finder := &mockEventChatFinder{}
	counters := health.NewTelegramCounters()

	scheduler := NewScheduler(cfg, c, logger.New("error", nil), sender, finder, nil, nil)
	scheduler.SetCounterSink(counters)
	if got := scheduler.TrackedReminders(); got != 0 {
		t.Fatalf("a scheduler that never started tracks %d reminders", got)
	}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(10*time.Minute), now.Add(55*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student", ReminderLeadMinutes: 15}}

	scheduler.Start()
	defer scheduler.Stop()
	scheduler.RemindNow(now)

	if got := scheduler.TrackedReminders(); got == 0 {
		t.Error("the gauge must follow the notifier that actually sends the reminders")
	}
	if got := counters.TelegramStats().Notifications[kindReminder]; got != 1 {
		t.Errorf("reminders counted = %d, want 1", got)
	}
}
