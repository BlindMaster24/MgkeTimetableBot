package notification

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/reminder"
)

func reminderSlot(start, end time.Time) [2][2]string {
	return [2][2]string{{start.Format("15:04"), start.Format("15:04")}, {end.Format("15:04"), end.Format("15:04")}}
}

func reminderMoment() time.Time {
	now := time.Now()
	for now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		now = now.AddDate(0, 0, 1)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
}

func seedReminderDay(t *testing.T, c *cache.RaspCache, now time.Time, slots [][2][2]string, lessons []any) {
	t.Helper()
	c.SetCalls(t.Context(), cache.Schedule{}, cache.Schedule{Weekdays: slots}, "manual")
	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(now.Format("02.01.2006"), lessons)),
	}, "hash1")
}

func TestRemindUpcomingLessonFires(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(sender.sent))
	}
	for _, want := range []string{"⏰", "Математика", "· 101"} {
		if !strings.Contains(sender.sent[0].text, want) {
			t.Errorf("reminder must contain %q, got %q", want, sender.sent[0].text)
		}
	}
	if sender.sent[0].chatID != 1001 {
		t.Errorf("reminder must go to peer 1001, got %d", sender.sent[0].chatID)
	}
}

func TestRemindUpcomingLessonWithoutRoomSkipsSuffix(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(sender.sent))
	}
	if strings.Contains(sender.sent[0].text, "·") {
		t.Errorf("reminder without room must skip the suffix, got %q", sender.sent[0].text)
	}
}

func TestRemindPassedLessonStaysSilent(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(-90*time.Minute), now.Add(-5*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)

	if len(sender.sent) != 0 {
		t.Fatalf("passed lesson must not remind, got %d", len(sender.sent))
	}
}

func TestRemindRespectsLeadTime(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(90*time.Minute), now.Add(135*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)
	if len(sender.sent) != 0 {
		t.Fatalf("lesson outside lead must not remind, got %d", len(sender.sent))
	}

	n.RemindUpcoming(now.Add(35*time.Minute), time.Hour, 0, nil)
	if len(sender.sent) != 1 {
		t.Fatalf("lesson inside lead must remind, got %d", len(sender.sent))
	}
}

func TestRemindSendsOnce(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)
	n.RemindUpcoming(now.Add(time.Minute), time.Hour, 0, nil)
	n.RemindUpcoming(now.Add(2*time.Minute), time.Hour, 0, nil)

	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly 1 reminder, got %d", len(sender.sent))
	}
}

func TestRemindWithoutLeadStaysSilent(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, 0, 0, nil)

	if len(sender.sent) != 0 {
		t.Fatalf("disabled reminders must not send, got %d", len(sender.sent))
	}
}

func TestRemindWaitsForChats(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)
	if len(sender.sent) != 0 {
		t.Fatalf("no chats must mean no sends, got %d", len(sender.sent))
	}

	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	n.RemindUpcoming(now.Add(time.Minute), time.Hour, 0, nil)
	if len(sender.sent) != 1 {
		t.Fatalf("late chats must still get the reminder, got %d", len(sender.sent))
	}
}

func TestRemindAppliesJitterWindow(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	start := now.Add(90 * time.Minute)
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(start, start.Add(45*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	lead := time.Hour
	window := 5 * time.Minute
	fireAt := reminder.ReminderTime(start.Truncate(time.Minute), lead, window, rand.New(rand.NewSource(3)))

	n.RemindUpcoming(fireAt.Add(-time.Minute), lead, window, rand.New(rand.NewSource(3)))
	if len(sender.sent) != 0 {
		t.Fatalf("jittered reminder must wait for its fire time, got %d", len(sender.sent))
	}

	n.RemindUpcoming(fireAt, lead, window, rand.New(rand.NewSource(3)))
	if len(sender.sent) != 1 {
		t.Fatalf("jittered reminder must fire at its fire time, got %d", len(sender.sent))
	}
}

func remindScheduler(t *testing.T, c *cache.RaspCache, sender *mockEventSender, finder *mockEventChatFinder, leadMinutes int) *Scheduler {
	t.Helper()
	cfg := &config.Config{}
	cfg.Timetable.ReminderLeadMinutes = leadMinutes
	s := NewScheduler(cfg, c, logger.New("error", nil), sender, finder, nil, nil)
	s.notifier = NewEventNotifier(c, cfg, logger.New("error", nil), sender, finder)
	s.SetRand(rand.New(rand.NewSource(11)))
	return s
}

func TestSchedulerRemindsUpcomingLesson(t *testing.T) {
	_, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	remindScheduler(t, c, sender, finder, 60).RemindNow(now)

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 scheduler reminder, got %d", len(sender.sent))
	}
}

func TestSchedulerSkipsPassedLesson(t *testing.T) {
	_, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(-90*time.Minute), now.Add(-5*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	remindScheduler(t, c, sender, finder, 60).RemindNow(now)

	if len(sender.sent) != 0 {
		t.Fatalf("passed lesson must stay silent, got %d", len(sender.sent))
	}
}

func TestSchedulerStaysSilentWithoutLead(t *testing.T) {
	_, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	remindScheduler(t, c, sender, finder, 0).RemindNow(now)

	if len(sender.sent) != 0 {
		t.Fatalf("disabled scheduler must stay silent, got %d", len(sender.sent))
	}
}

func TestReminderLeadDefaultsToDisabled(t *testing.T) {
	if lead, window := reminderLead(nil); lead != 0 || window != 0 {
		t.Fatalf("lead = %v window = %v, want zeros", lead, window)
	}
	if lead, _ := reminderLead(&config.Config{}); lead != 0 {
		t.Fatalf("lead = %v, want zero without config", lead)
	}
}

func TestSchedulerRegistersReminderTick(t *testing.T) {
	_, _, _, c := newTestNotifier(t)
	cfg := &config.Config{}
	cfg.Timetable.ReminderLeadMinutes = 15
	s := NewScheduler(cfg, c, logger.New("error", nil), nil, nil, nil, nil)
	s.notifier = NewEventNotifier(c, cfg, logger.New("error", nil), nil, nil)
	s.registerReminders()
	if len(s.cron.Entries()) != 1 {
		t.Fatalf("entries = %d, want 1 reminder tick", len(s.cron.Entries()))
	}

	quiet := NewScheduler(&config.Config{}, c, logger.New("error", nil), nil, nil, nil, nil)
	quiet.registerReminders()
	if len(quiet.cron.Entries()) != 0 {
		t.Fatalf("entries = %d, want none without lead", len(quiet.cron.Entries()))
	}
}

func TestRemindUsesTheChatLeadOverride(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{
		{ID: 1, PeerID: 1001, Group: "63", Mode: "student", ReminderLeadMinutes: 90},
		{ID: 2, PeerID: 1002, Group: "63", Mode: "student"},
	}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(90*time.Minute), now.Add(135*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, 15*time.Minute, 0, nil)
	if len(sender.sent) != 1 {
		t.Fatalf("the chat override must fire on its own lead, got %d sends", len(sender.sent))
	}
	if sender.sent[0].chatID != 1001 {
		t.Fatalf("the override chat must hear first, got %d", sender.sent[0].chatID)
	}

	n.RemindUpcoming(now.Add(76*time.Minute), 15*time.Minute, 0, nil)
	if len(sender.sent) != 2 {
		t.Fatalf("the default chat must fire on the shared lead, got %d sends", len(sender.sent))
	}
	if sender.sent[1].chatID != 1002 {
		t.Fatalf("the default chat must hear later, got %d", sender.sent[1].chatID)
	}

	n.RemindUpcoming(now.Add(80*time.Minute), 15*time.Minute, 0, nil)
	if len(sender.sent) != 2 {
		t.Fatalf("every chat must hear about a pair once, got %d sends", len(sender.sent))
	}
}

func TestRemindChatLeadCanBeShorterThanTheDefault(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student", ReminderLeadMinutes: 5}}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)
	if len(sender.sent) != 0 {
		t.Fatalf("a short chat lead must wait instead of firing early, got %d sends", len(sender.sent))
	}

	n.RemindUpcoming(now.Add(25*time.Minute), time.Hour, 0, nil)
	if len(sender.sent) != 1 {
		t.Fatalf("the short chat lead must fire at its own time, got %d sends", len(sender.sent))
	}
}

func TestRemindSendsToEveryChatOnce(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{
		{ID: 1, PeerID: 1001, Group: "63", Mode: "student"},
		{ID: 2, PeerID: 1002, Group: "63", Mode: "student"},
	}

	now := reminderMoment()
	seedReminderDay(t, c, now,
		[][2][2]string{reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute))},
		[]any{map[string]any{"lesson": "Математика", "cabinet": "101"}},
	)

	n.RemindUpcoming(now, time.Hour, 0, nil)
	if len(sender.sent) != 2 {
		t.Fatalf("both chats must hear about the pair, got %d sends", len(sender.sent))
	}

	n.RemindUpcoming(now.Add(time.Minute), time.Hour, 0, nil)
	if len(sender.sent) != 2 {
		t.Fatalf("a repeated tick must not send again, got %d sends", len(sender.sent))
	}
}

func TestRemindTeacherLessonFires(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byTeachers = []*EventChat{{ID: 2, PeerID: 1002, Teacher: "Иванов И.И.", Mode: "teacher"}}

	now := reminderMoment()
	c.SetCalls(t.Context(), cache.Schedule{}, cache.Schedule{Weekdays: [][2][2]string{
		reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute)),
	}}, "manual")
	c.SetTeachers(t.Context(), map[string]any{
		"Иванов И.И.": entryWithDays(dayMap(now.Format("02.01.2006"), []any{
			map[string]any{"lesson": "Математика", "group": "63"},
		})),
	}, "hash1")

	n.RemindUpcoming(now, time.Hour, 0, nil)

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 teacher reminder, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "Математика") {
		t.Errorf("reminder must name the lesson, got %q", sender.sent[0].text)
	}
}
