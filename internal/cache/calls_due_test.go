package cache

import (
	"context"
	"testing"
	"time"
)

func TestCallsDueFollowsTheInterval(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if !c.CallsDue(now, 0) {
		t.Fatal("a zero interval always allows a refresh")
	}
	if !c.CallsDue(now, time.Hour) {
		t.Fatal("a missing update time always allows a refresh")
	}

	c.Calls.Update = now.Add(-30 * time.Minute).UnixMilli()
	if c.CallsDue(now, time.Hour) {
		t.Fatal("an update 30 minutes old must not be due for an hourly interval")
	}
	if !c.CallsDue(now, 15*time.Minute) {
		t.Fatal("an update 30 minutes old is due for a 15 minute interval")
	}
}

func TestCallsGettersReadTheActiveSchedule(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	weekday := [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}}
	saturday := [][2][2]string{{{"09:00", "09:45"}, {"09:50", "10:35"}}}
	c.Calls.Active.Schedule = CallsSchedule{Weekdays: weekday, Saturday: saturday}

	if got := c.GetCallsWeekdays(); len(got) != 1 || got[0] != weekday[0] {
		t.Fatalf("GetCallsWeekdays() = %v", got)
	}
	if got := c.GetCallsSaturday(); len(got) != 1 || got[0] != saturday[0] {
		t.Fatalf("GetCallsSaturday() = %v", got)
	}
}

func TestDrainDayChangesDedupesAndFilters(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	change := DayChange{Kind: "group", Value: "100", Date: "07.09.2026"}
	c.dayChanges = []DayChange{
		change,
		change,
		{Kind: "group", Value: "", Date: "07.09.2026"},
		{Kind: "teacher", Value: "Иванов", Date: ""},
	}

	got := c.DrainDayChanges(context.Background())
	if len(got) != 1 || got[0] != change {
		t.Fatalf("DrainDayChanges() = %+v, want only the unique complete change", got)
	}
	if again := c.DrainDayChanges(context.Background()); len(again) != 0 {
		t.Fatalf("the drain must empty the buffer, got %+v", again)
	}

	c.dayChanges = []DayChange{change}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := c.DrainDayChanges(cancelled); got != nil {
		t.Fatalf("a cancelled context must drain nothing, got %+v", got)
	}
}

func TestGroupKeysListsEveryGroup(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.Groups.Timetable = GroupTimetable{"100": {}, "200": {}}

	keys := c.GroupKeys()
	seen := map[string]bool{}
	for _, key := range keys {
		seen[key] = true
	}
	if !seen["100"] || !seen["200"] || len(keys) != 2 {
		t.Fatalf("GroupKeys() = %v", keys)
	}
}
