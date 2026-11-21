package app

import (
	"context"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func weekEntry(day string, lesson map[string]any) map[string]any {
	lessons := []any{}
	if lesson != nil {
		lessons = append(lessons, lesson)
	}
	return map[string]any{"days": []any{map[string]any{"day": day, "lessons": lessons}}}
}

func newEventsRuntime(t *testing.T) *runtime {
	t.Helper()

	rasp, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return &runtime{
		log:     logger.New("error", nil),
		metrics: health.NewTracker(health.DefaultThresholds()),
		cache:   rasp,
	}
}

func TestDrainEventsSummarisesAndEmptiesTheQueue(t *testing.T) {
	r := newEventsRuntime(t)
	ctx := context.Background()
	today := utils.SiteNow().Format("02.01.2006")

	r.cache.SetGroups(ctx, map[string]any{"100": weekEntry(today, map[string]any{"lesson": "Математика"})}, "h1")
	r.cache.DrainEvents(ctx)
	r.cache.SetGroups(ctx, map[string]any{"100": weekEntry(today, map[string]any{"lesson": "Физика"})}, "h2")

	if got := len(r.cache.DrainEvents(ctx)); got == 0 {
		t.Fatal("a changed day must leave an event for the notifier")
	}

	r.cache.SetGroups(ctx, map[string]any{"100": weekEntry(today, map[string]any{"lesson": "Химия"})}, "h3")
	r.drainEvents(ctx, "test")
	if left := r.cache.DrainEvents(ctx); len(left) != 0 {
		t.Fatalf("the queue must be empty after the drain, still %d events", len(left))
	}
}

func TestSyncCalendarsStaysQuietWhenGoogleIsOff(t *testing.T) {
	r := newEventsRuntime(t)
	today := utils.SiteNow().Format("02.01.2006")
	r.cache.SetGroups(context.Background(), map[string]any{"100": weekEntry(today, nil)}, "")

	days, err := r.syncCalendars(context.Background(), "test")
	if err != nil || days != 0 {
		t.Fatalf("syncCalendars without Google = %d %v, want 0 nil", days, err)
	}
}
