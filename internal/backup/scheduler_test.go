package backup

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/robfig/cron/v3"
)

func testLogger() *logger.Logger {
	return logger.New("error", nil)
}

func TestSchedulerRunsTheTaskItWasGiven(t *testing.T) {
	calls := 0
	s := NewScheduler("0 30 4 * * *", testLogger(), func() { calls++ })

	if !s.Start() {
		t.Fatal("a valid schedule must arm the maintenance job")
	}
	defer s.Stop()

	entries := s.cron.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 maintenance job", len(entries))
	}

	entries[0].Job.(cron.FuncJob)()
	if calls != 1 {
		t.Fatalf("task ran %d times, want 1", calls)
	}
}

func TestSchedulerStaysOffWithoutAScheduleOrTask(t *testing.T) {
	withoutSchedule := NewScheduler("", testLogger(), func() {})
	if withoutSchedule.Start() {
		t.Error("an empty schedule must not arm the maintenance job")
	}
	if entries := withoutSchedule.cron.Entries(); len(entries) != 0 {
		t.Errorf("entries = %d, want none", len(entries))
	}

	withoutTask := NewScheduler("0 30 4 * * *", testLogger(), nil)
	if withoutTask.Start() {
		t.Error("a missing task must not arm the maintenance job")
	}
}

func TestSchedulerRejectsABrokenExpression(t *testing.T) {
	s := NewScheduler("every day", testLogger(), func() {})

	if s.Start() {
		t.Error("a broken cron expression must not arm the maintenance job")
	}
	if entries := s.cron.Entries(); len(entries) != 0 {
		t.Errorf("entries = %d, want none", len(entries))
	}
}

func TestSchedulerStopsBeforeItStarted(t *testing.T) {
	NewScheduler("0 30 4 * * *", testLogger(), func() {}).Stop()
}
