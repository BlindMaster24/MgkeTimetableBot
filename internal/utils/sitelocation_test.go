package utils

import (
	"testing"
	"time"
)

func TestSiteLocationIsMinsk(t *testing.T) {
	if name := SiteLocation.String(); name != "Europe/Minsk" {
		t.Fatalf("SiteLocation = %q, want Europe/Minsk", name)
	}
	_, offset := time.Date(2026, 9, 7, 12, 0, 0, 0, SiteLocation).Zone()
	if offset != 3*60*60 {
		t.Fatalf("offset = %d, want three hours", offset)
	}
}

func TestSiteNowFollowsTheCollegeClock(t *testing.T) {
	now := SiteNow()
	if _, offset := now.Zone(); offset != 3*60*60 {
		t.Fatalf("SiteNow offset = %d, want three hours", offset)
	}
	if drift := time.Since(now); drift > time.Minute || drift < -time.Minute {
		t.Fatalf("SiteNow drifted from the system clock: %v", drift)
	}
}

// TestSiteNowKeepsLateSundayOutOfThePreviousWeek documents the bug the old
// bot's users hit: a server clock in UTC on Sunday evening is already Monday
// in Minsk, so the previous day's math put the coming Monday into the
// finished week.
func TestSiteNowKeepsLateSundayOutOfThePreviousWeek(t *testing.T) {
	utcSundayEvening := time.Date(2026, 11, 15, 22, 0, 0, 0, time.UTC)
	mondayMorning := utcSundayEvening.In(SiteLocation)

	if utcSundayEvening.Weekday() != time.Sunday || mondayMorning.Weekday() != time.Monday {
		t.Fatalf("fixture drift: %v / %v", utcSundayEvening.Weekday(), mondayMorning.Weekday())
	}
	if DayIndexFromDate(utcSundayEvening) >= DayIndexFromDate(mondayMorning) {
		t.Fatal("the Minsk morning must be a later calendar day")
	}
	if WeekIndexFromDate(utcSundayEvening).Value() >= WeekIndexFromDate(mondayMorning).Value() {
		t.Fatalf("the UTC day belongs to the finished week, Minsk already to the next one: %d vs %d",
			WeekIndexFromDate(utcSundayEvening).Value(), WeekIndexFromDate(mondayMorning).Value())
	}
}

func TestSundayStaysInsideItsMondayStartWeek(t *testing.T) {
	monday := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	sunday := monday.AddDate(0, 0, 6)

	if WeekIndexFromDate(monday) != WeekIndexFromDate(sunday) {
		t.Fatalf("Sunday %v must belong to the week of Monday %v", sunday, monday)
	}
	if first, last := WeekIndexFromDate(monday).WeekRange(); !first.Equal(monday) || !last.Equal(sunday) {
		t.Fatalf("week range = %v..%v, want %v..%v", first, last, monday, sunday)
	}
}
