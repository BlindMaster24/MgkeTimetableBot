package utils

import (
	"testing"
	"time"
)

func TestDayIndexFollowsTheCalendarDateOfTheTimezone(t *testing.T) {
	minsk := time.FixedZone("Europe/Minsk", 3*60*60)

	lateUTC := time.Date(2026, 9, 13, 22, 30, 0, 0, time.UTC)
	sameInstantLocal := lateUTC.In(minsk)

	if sameInstantLocal.Day() != 14 {
		t.Fatalf("the fixture must cross midnight in the local zone, got %s", sameInstantLocal)
	}

	if DayIndexFromDate(lateUTC) == DayIndexFromDate(sameInstantLocal) {
		t.Error("a UTC process and a local process must not agree on the day near midnight")
	}

	nextLocalDay := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	if got, want := DayIndexFromDate(sameInstantLocal), DayIndexFromDate(nextLocalDay); got != want {
		t.Errorf("the local day index = %d, want the index of 14.09 (%d)", got, want)
	}
}

func TestDayIndexRoundTripsThroughTheLocalDate(t *testing.T) {
	minsk := time.FixedZone("Europe/Minsk", 3*60*60)

	for _, moment := range []time.Time{
		time.Date(2026, 9, 13, 0, 0, 0, 0, minsk),
		time.Date(2026, 9, 13, 23, 59, 59, 0, minsk),
		time.Date(2027, 1, 1, 12, 0, 0, 0, minsk),
	} {
		index := DayIndexFromDate(moment)
		back := DayIndexToDate(index)

		if back.Year() != moment.Year() || back.Month() != moment.Month() || back.Day() != moment.Day() {
			t.Errorf("index %d maps back to %s, want the local date %s", index, back.Format("02.01.2006"), moment.Format("02.01.2006"))
		}
	}
}

func TestWeekIndexFollowsTheLocalWeek(t *testing.T) {
	minsk := time.FixedZone("Europe/Minsk", 3*60*60)

	sundayEveningUTC := time.Date(2026, 9, 13, 22, 30, 0, 0, time.UTC)
	mondayNightLocal := sundayEveningUTC.In(minsk)

	if mondayNightLocal.Weekday() != time.Monday {
		t.Fatalf("the fixture must be Monday in the local zone, got %s", mondayNightLocal.Weekday())
	}

	utcWeek := WeekIndexFromDate(sundayEveningUTC).Value()
	localWeek := WeekIndexFromDate(mondayNightLocal).Value()

	if utcWeek == localWeek {
		t.Error("a UTC process and a local process must not agree on the academic week around the weekend")
	}

	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, minsk)
	if got, want := localWeek, WeekIndexFromDate(monday).Value(); got != want {
		t.Errorf("the local week index = %d, want the index of Monday 14.09 (%d)", got, want)
	}
}
