package utils

import (
	"testing"
	"time"
)

func TestWeekRangeMatchesTheWeekDayIndices(t *testing.T) {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("Europe/Minsk", 3*60*60),
		time.FixedZone("America/Chicago", -6*60*60),
		time.FixedZone("Pacific/Kiritimati", 14*60*60),
	}

	for _, zone := range zones {
		for _, isoWeek := range []int{2900, 2957, 2958, 3000} {
			week := WeekIndexFromNumber(isoWeek)

			first, last := week.WeekRange()
			if first.Weekday() != time.Monday || last.Weekday() != time.Sunday {
				t.Errorf("%s: week %d spans %s..%s, want Monday..Sunday", zone, isoWeek, first.Weekday(), last.Weekday())
			}

			minDay, maxDay := week.WeekDayIndexRange()
			if minDay != isoWeek*7 || maxDay != isoWeek*7+6 {
				t.Errorf("%s: week %d covers day indices %d..%d, want %d..%d", zone, isoWeek, minDay, maxDay, isoWeek*7, isoWeek*7+6)
			}

			if got := DayIndexFromDate(first); got != minDay {
				t.Errorf("%s: the first day of week %d has day index %d, want %d", zone, isoWeek, got, minDay)
			}
			if got := DayIndexFromDate(last); got != maxDay {
				t.Errorf("%s: the last day of week %d has day index %d, want %d", zone, isoWeek, got, maxDay)
			}
		}
	}
}

func TestWeekIndexRoundTripsThroughItsRange(t *testing.T) {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("Europe/Minsk", 3*60*60),
		time.FixedZone("America/Chicago", -6*60*60),
	}

	for _, zone := range zones {
		for _, day := range []time.Time{
			time.Date(2026, 9, 7, 8, 0, 0, 0, zone),
			time.Date(2026, 9, 14, 23, 30, 0, 0, zone),
			time.Date(2027, 1, 3, 12, 0, 0, 0, zone),
		} {
			week := WeekIndexFromDate(day)
			first, last := week.WeekRange()

			if day.Before(first) || day.After(last.AddDate(0, 0, 1)) {
				t.Errorf("%s: %s falls outside its own week %s..%s", zone, day.Format("02.01.2006"), first.Format("02.01.2006"), last.Format("02.01.2006"))
			}

			if got := WeekIndexFromDate(first).Value(); got != week.Value() {
				t.Errorf("%s: the first day of week %d maps back to week %d", zone, week.Value(), got)
			}
		}
	}
}

func TestAcademicWeekNumberRoundTrips(t *testing.T) {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("Europe/Minsk", 3*60*60),
	}

	for _, zone := range zones {
		today := time.Date(2026, 11, 18, 10, 0, 0, 0, zone)

		for _, week := range []int{WeekIndexFromDate(today).Value(), WeekIndexFromDate(today).Value() - 4, WeekIndexFromDate(today).Value() + 3} {
			source := WeekIndexFromNumber(week)
			academic := source.AcademicWeekNumber()

			if got := WeekIndexFromAcademicNumber(academic, today).Value(); got != week {
				t.Errorf("%s: academic week %d maps to week index %d, want %d", zone, academic, got, week)
			}
		}
	}
}

func TestAcademicWeekNumberStartsAtTheFirstSeptemberMonday(t *testing.T) {
	start := WeekIndexFromDate(time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if got := start.AcademicWeekNumber(); got != 1 {
		t.Errorf("the first academic week = %d, want 1", got)
	}

	second := WeekIndexFromNumber(start.Value() + 1)
	if got := second.AcademicWeekNumber(); got != 2 {
		t.Errorf("the second academic week = %d, want 2", got)
	}
}
