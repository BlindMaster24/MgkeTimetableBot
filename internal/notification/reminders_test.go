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

func seedReminderDay(t *testing.T, c *cache.RaspCache, slots [][2][2]string, lessons []any) {
	t.Helper()
	c.SetCalls(t.Context(), cache.Schedule{}, cache.Schedule{Weekdays: slots}, "manual")
	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(todayStr(), lessons)),
	}, "hash1")
}

func TestRemindUpcomingLessonFires(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	start := now.Add(90 * time.Minute)
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

	now := time.Now()
	seedReminderDay(t, c,
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

func TestRemindTeacherLessonFires(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byTeachers = []*EventChat{{ID: 2, PeerID: 1002, Teacher: "Иванов И.И.", Mode: "teacher"}}

	now := time.Now()
	c.SetCalls(t.Context(), cache.Schedule{}, cache.Schedule{Weekdays: [][2][2]string{
		reminderSlot(now.Add(30*time.Minute), now.Add(75*time.Minute)),
	}}, "manual")
	c.SetTeachers(t.Context(), map[string]any{
		"Иванов И.И.": entryWithDays(dayMap(todayStr(), []any{
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
