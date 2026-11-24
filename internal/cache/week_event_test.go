package cache

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func weekEventFor(t *testing.T, previous int, day time.Time) []Event {
	t.Helper()
	data := map[string]any{
		"63": map[string]any{"days": []any{map[string]any{
			"day":     day.Format("02.01.2006"),
			"lessons": []any{map[string]any{"lesson": "A"}},
		}}},
	}
	events, _ := collectWeekEvents(KindGroups, previous, data)
	return events
}

func TestTheCurrentWeekIsNotAnnouncedAsTheNextOne(t *testing.T) {
	now := utils.SiteNow()
	current := utils.WeekIndexFromDate(now)

	if events := weekEventFor(t, current.Value()-1, now); len(events) != 0 {
		t.Fatalf("Monday must not announce the current week as the next one, got %+v", events)
	}
}

func TestAGenuinelyFutureWeekIsAnnouncedOnce(t *testing.T) {
	now := utils.SiteNow()
	future := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(now).Value() + 1)

	events := weekEventFor(t, future.Value()-1, future.FirstDayDate())
	if len(events) != 1 || events[0].Week == nil || events[0].Week.Week != future.Value() {
		t.Fatalf("a future week must produce one event, got %+v", events)
	}
	if events := weekEventFor(t, future.Value(), future.FirstDayDate()); len(events) != 0 {
		t.Fatalf("the same week must not be announced twice, got %+v", events)
	}
}
