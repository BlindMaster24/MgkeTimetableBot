package testgolden

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeReplacesTheVolatileParts(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

	text := strings.Join([]string{
		"07.09.2026, 12:30 обновлено",
		"08.09.2026 расписание",
		"07.09 урок",
		"Понедельник · № 12",
		"Понедельник (сегодня)",
		"расписание загружена 5 мин. назад",
		"(только что)",
		"👉 пара 👈",
	}, "\n")

	got := Normalize(text, now)
	for _, want := range []string{"{{datetime}} обновлено", "{{tomorrow}} расписание", "{{dd.mm}} урок", "{{wd}} · №{{week}}", "{{wd}} {{hint}}", "{{ago}}", "({{ago}})", "{{now}} пара {{now}}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Normalize lost %q in %q", want, got)
		}
	}
	if strings.Contains(got, "07.09.2026") {
		t.Fatalf("Normalize left today in the clear: %q", got)
	}
}

func TestNormalizeKeepsTheRestOfTheText(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	if got := Normalize("Математика 101 Иванов", now); got != "Математика 101 Иванов" {
		t.Fatalf("Normalize touched the stable text: %q", got)
	}
}

func TestNormalizeWeekNumberMarksOnlyTheNeighbouringWeeks(t *testing.T) {
	got := NormalizeWeekNumber("w=10 9 11 12 20", 10)
	if got != "w={{week}} {{week-1}} {{week+1}} {{week+2}} 20" {
		t.Fatalf("NormalizeWeekNumber = %q", got)
	}

	if got := NormalizeWeekNumber("№ 10", 10); got != "№ {{week}}" {
		t.Fatalf("the week number pattern must win the second pass, got %q", got)
	}
}

func TestNormalizeWeekDropsTheDayHint(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	got := NormalizeWeek("07.09.2026 <i>(сегодня)</i> и 08.09.2026 урок", now)
	if strings.Contains(got, "<i>") || strings.Contains(got, "(сегодня)") {
		t.Fatalf("the hint must be gone: %q", got)
	}
	if !strings.Contains(got, "{{date}} урок") {
		t.Fatalf("both days must collapse into one token: %q", got)
	}
}

func TestFirstDifferenceNamesTheLine(t *testing.T) {
	got := FirstDifference("a\nb\nc", "a\nB\nc")
	if !strings.Contains(got, "line 2") || !strings.Contains(got, "b") || !strings.Contains(got, "B") {
		t.Fatalf("FirstDifference = %q", got)
	}

	if got := FirstDifference("a\nb", "a\nb\nc"); !strings.Contains(got, "line 3") {
		t.Fatalf("a missing tail must name the extra line, got %q", got)
	}
	if got := FirstDifference("same\ntext", "same\ntext"); got != "no line difference found" {
		t.Fatalf("equal texts must report equality, got %q", got)
	}
}

func TestWeekDatesStartsOnTheFirstDay(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	week := RelevantWeek(now)

	dates := WeekDates(week, 2)
	if len(dates) != 2 {
		t.Fatalf("dates = %v", dates)
	}
	first := week.FirstDayDate().Format("02.01.2006")
	if dates[0] != first {
		t.Fatalf("first date = %q, want %q", dates[0], first)
	}
	if want := week.FirstDayDate().AddDate(0, 0, 1).Format("02.01.2006"); dates[1] != want {
		t.Fatalf("second date = %q, want %q", dates[1], want)
	}
}
