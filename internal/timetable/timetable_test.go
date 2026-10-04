package timetable

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseDateAcceptsPlainDates(t *testing.T) {
	parsed, err := ParseDate("07.09.2026")
	if err != nil {
		t.Fatalf("ParseDate() = %v, want nil", err)
	}
	if parsed.Day() != 7 || parsed.Month() != time.September || parsed.Year() != 2026 {
		t.Errorf("ParseDate() = %v, want 07.09.2026", parsed)
	}
}

func TestParseDateTrimsSpaces(t *testing.T) {
	if _, err := ParseDate("  07.09.2026\n"); err != nil {
		t.Errorf("ParseDate() = %v, want nil", err)
	}
}

func TestParseDateRejectsLegacyLabels(t *testing.T) {
	for _, value := range []string{"", "Понедельник, 07.09.2026", "2026-09-07", "07/09/2026", "32.01.2026"} {
		if _, err := ParseDate(value); err == nil {
			t.Errorf("ParseDate(%q) = nil, want an error", value)
		}
	}
}

func TestFormatDateRoundTrips(t *testing.T) {
	parsed, err := ParseDate("07.09.2026")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatDate(parsed); got != "07.09.2026" {
		t.Errorf("FormatDate() = %q, want 07.09.2026", got)
	}
}

func TestLessonValidate(t *testing.T) {
	if err := (Lesson{Num: 1, Subject: "Математика"}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	for _, lesson := range []Lesson{
		{Num: 0, Subject: "Математика"},
		{Num: 1, Subject: "   "},
		{Num: 1, Subject: "Математика", Subgroup: -1},
	} {
		if err := lesson.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error", lesson)
		}
	}
}

func TestDayValidate(t *testing.T) {
	valid := Day{Date: "07.09.2026", Lessons: []Lesson{{Num: 1, Subject: "Математика"}}}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if _, err := valid.Time(); err != nil {
		t.Errorf("Time() = %v, want nil", err)
	}

	shared := Day{Date: "07.09.2026", Lessons: []Lesson{
		{Num: 1, Subject: "Математика"},
		{Num: 1, Subject: "Физика", Subgroup: 2},
	}}
	if err := shared.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a shared slot", err)
	}
	invalid := []Day{
		{Date: "Понедельник", Lessons: []Lesson{{Num: 1, Subject: "Математика"}}},
		{Date: "07.09.2026", Lessons: []Lesson{{Num: 0, Subject: "Математика"}}},
		{Date: "07.09.2026", Lessons: []Lesson{{Num: 1, Subject: "   "}}},
	}
	for _, day := range invalid {
		if err := day.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want an error", day)
		}
	}
}

func TestWeekValidateRejectsRepeatedDates(t *testing.T) {
	week := Week{Days: []Day{
		{Date: "07.09.2026", Lessons: []Lesson{{Num: 1, Subject: "Математика"}}},
		{Date: "07.09.2026", Lessons: []Lesson{{Num: 1, Subject: "Физика"}}},
	}}
	if err := week.Validate(); err == nil || !strings.Contains(err.Error(), "07.09.2026") {
		t.Errorf("Validate() = %v, want an error naming the repeated day", err)
	}
}

func TestWeekSortOrdersDaysChronologically(t *testing.T) {
	week := Week{Days: []Day{
		{Date: "07.09.2026", Lessons: []Lesson{{Num: 2, Subject: "Физика"}, {Num: 1, Subject: "Математика"}}},
		{Date: "31.08.2026", Lessons: []Lesson{{Num: 1, Subject: "История"}}},
	}}
	week.Sort()
	if week.Days[0].Date != "31.08.2026" || week.Days[1].Date != "07.09.2026" {
		t.Errorf("Sort() ordered days as %q, %q, want chronological order", week.Days[0].Date, week.Days[1].Date)
	}
	if week.Days[1].Lessons[0].Num != 1 || week.Days[1].Lessons[1].Num != 2 {
		t.Errorf("Sort() left lessons as %+v, want them numbered in order", week.Days[1].Lessons)
	}
	if err := week.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestDayKeepsItsJSONShape(t *testing.T) {
	day := Day{Date: "07.09.2026", Index: 42, Lessons: []Lesson{{Num: 1, Subject: "Математика", Teacher: "Иванов И.И.", Room: "3-205"}}}
	raw, err := json.Marshal(day)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Day
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if decoded.Index != 42 || decoded.Lessons[0].Room != "3-205" {
		t.Errorf("round trip lost data: %+v", decoded)
	}
}
