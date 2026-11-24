package cache

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func weekEventFor(t *testing.T, now, day time.Time, previous int) []Event {
	t.Helper()
	data := map[string]any{
		"63": map[string]any{"days": []any{map[string]any{
			"day":     day.Format("02.01.2006"),
			"lessons": []any{map[string]any{"lesson": "A"}},
		}}},
	}
	c := &RaspCache{}
	c.SetClock(func() time.Time { return now })
	events, _ := c.collectWeekEvents(KindGroups, previous, data)
	return events
}

func TestTheCurrentWeekIsNotAnnouncedAsTheNextOne(t *testing.T) {
	now := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	current := utils.WeekIndexFromDate(now)

	if events := weekEventFor(t, now, now, current.Value()-1); len(events) != 0 {
		t.Fatalf("Monday must not announce the current week as the next one, got %+v", events)
	}
}

func TestAGenuinelyFutureWeekIsAnnouncedOnce(t *testing.T) {
	now := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	future := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(now).Value() + 1)

	events := weekEventFor(t, now, future.FirstDayDate(), future.Value()-1)
	if len(events) != 1 || events[0].Week == nil || events[0].Week.Week != future.Value() {
		t.Fatalf("a future week must produce one event, got %+v", events)
	}
	if events := weekEventFor(t, now, future.FirstDayDate(), future.Value()); len(events) != 0 {
		t.Fatalf("the same week must not be announced twice, got %+v", events)
	}
}
