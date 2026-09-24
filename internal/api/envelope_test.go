package api

import (
	"reflect"
	"testing"
	"time"
)

func TestSortNamesMatchesTSSort(t *testing.T) {
	got := sortNames([]string{"100", "99", "ПСМ", "10"})
	want := []string{"10", "99", "100", "ПСМ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortNames = %v, want %v", got, want)
	}

	got = sortNames([]string{"b", "A", "a"})
	want = []string{"A", "a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortNames = %v, want %v", got, want)
	}

	got = sortNames([]string{"063", "63"})
	want = []string{"063", "63"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stable order for numeric twins = %v, want %v", got, want)
	}

	if got := sortNames(nil); len(got) != 0 {
		t.Fatalf("sortNames(nil) = %v, want empty", got)
	}
}

func TestWeekdayName(t *testing.T) {
	cases := map[time.Weekday]string{
		time.Monday:    "Понедельник",
		time.Tuesday:   "Вторник",
		time.Wednesday: "Среда",
		time.Thursday:  "Четверг",
		time.Friday:    "Пятница",
		time.Saturday:  "Суббота",
		time.Sunday:    "Воскресенье",
	}
	for weekday, want := range cases {
		date := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(weekday)-int(time.Monday))
		if got := weekdayName(date); got != want {
			t.Errorf("weekdayName(%v) = %q, want %q", weekday, got, want)
		}
	}
}

func TestDecorateDays(t *testing.T) {
	entry := map[string]any{"days": []any{map[string]any{"day": "01.09.2026", "subject": "Математика"}}}
	days := decorateDays(entry)
	if len(days) != 1 {
		t.Fatalf("days = %v", days)
	}
	day, ok := days[0].(map[string]any)
	if !ok {
		t.Fatalf("day is not a map: %v", days[0])
	}
	if day["weekday"] != "Вторник" {
		t.Errorf("weekday = %v, want Вторник", day["weekday"])
	}
	if day["subject"] != "Математика" || day["day"] != "01.09.2026" {
		t.Errorf("original keys lost: %v", day)
	}

	override := map[string]any{"days": []any{map[string]any{"day": "01.09.2026", "weekday": "Своя"}}}
	day = decorateDays(override)[0].(map[string]any)
	if day["weekday"] != "Своя" {
		t.Errorf("the day's own weekday must win, got %v", day["weekday"])
	}

	if got := decorateDays(map[string]any{}); got != nil {
		t.Errorf("entry without days = %v, want nil", got)
	}

	raw := decorateDays(map[string]any{"days": []any{"raw", map[string]any{"day": "вчера"}}})
	if raw[0] != "raw" {
		t.Errorf("non-map items must pass through, got %v", raw[0])
	}
	bad, ok := raw[1].(map[string]any)
	if !ok {
		t.Fatalf("bad day is not a map: %v", raw[1])
	}
	if _, exists := bad["weekday"]; exists {
		t.Errorf("unparseable date must not gain a weekday: %v", bad)
	}
}
