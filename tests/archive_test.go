package tests

import (
	"os"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestDayIndexConversion(t *testing.T) {
	cases := []struct {
		date string
		idx  int64
	}{
		{"05.01.1970", 0},
		{"06.01.1970", 1},
		{"27.08.2026", archive.DateToDayIndex("27.08.2026")},
	}

	for _, c := range cases {
		idx := archive.DateToDayIndex(c.date)
		date := archive.DayIndexToDate(idx)
		if idx != c.idx {
			t.Errorf("DateToDayIndex(%s) = %d, want %d", c.date, idx, c.idx)
		}
		if date != c.date {
			t.Errorf("DayIndexToDate(%d) = %s, want %s", idx, date, c.date)
		}
	}

	if got := archive.DateToDayIndex("не дата"); got != 0 {
		t.Errorf("an unparsable date must fall back to 0, got %d", got)
	}
}

func TestArchiveDaysAlignWithTheWeekIndex(t *testing.T) {
	repo, err := archive.New(t.TempDir() + "/archive.db")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	lessons := []any{map[string]any{"lesson": "Математика", "cabinet": "101"}}
	if err := repo.AppendDays([]archive.AppendDay{{
		Type:  "group",
		Value: "100",
		Day:   map[string]any{"day": day.Format("02.01.2006"), "lessons": lessons},
	}}); err != nil {
		t.Fatal(err)
	}

	week := utils.WeekIndexFromDate(day)
	minIdx, maxIdx := week.WeekDayIndexRange()
	days, err := repo.GroupDaysByRange(int64(minIdx), int64(maxIdx), "100")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("the bot's own week window must find the archived day, got %d", len(days))
	}
	if days[0].Day != day.Format("02.01.2006") {
		t.Errorf("archived day = %s, want %s", days[0].Day, day.Format("02.01.2006"))
	}

	exact, err := repo.GroupDay(int64(utils.DayIndexFromDate(day)), "100")
	if err != nil {
		t.Fatal(err)
	}
	if exact == nil {
		t.Fatal("an exact day lookup by the bot's index must find the archived day")
	}

	bounds, err := repo.DayIndexBounds()
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Min != int64(utils.DayIndexFromDate(day)) {
		t.Errorf("bounds = %+v, want the bot's index %d", bounds, utils.DayIndexFromDate(day))
	}
}

func TestArchiveRepository(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	repo, err := archive.New(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	db := repo.DB()

	group := "63"
	teacher := "Ivanov"
	dayIdx := archive.DateToDayIndex("01.09.2025")
	lessons := `[{"lesson":"Math","type":"lecture","teacher":"Ivanov","cabinet":"101"}]`

	_, err = db.Exec("INSERT INTO timetable_archive (day, \"group\", data) VALUES (?, ?, ?)", dayIdx, group, lessons)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO timetable_archive (day, teacher, data) VALUES (?, ?, ?)", dayIdx, teacher, lessons)
	if err != nil {
		t.Fatal(err)
	}

	bounds, err := repo.DayIndexBounds()
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Min != dayIdx || bounds.Max != dayIdx {
		t.Errorf("bounds = %+v, want min=%d max=%d", bounds, dayIdx, dayIdx)
	}

	groups, err := repo.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0] != "63" {
		t.Errorf("groups = %v, want [63]", groups)
	}

	teachers, err := repo.Teachers()
	if err != nil {
		t.Fatal(err)
	}
	if len(teachers) != 1 || teachers[0] != "Ivanov" {
		t.Errorf("teachers = %v, want [Ivanov]", teachers)
	}

	gd, err := repo.GroupDay(dayIdx, "63")
	if err != nil {
		t.Fatal(err)
	}
	if gd == nil {
		t.Fatal("expected group day, got nil")
	}
	if len(gd.Lessons) != 1 {
		t.Fatalf("expected 1 lesson, got %d", len(gd.Lessons))
	}

	td, err := repo.TeacherDay(dayIdx, "Ivanov")
	if err != nil {
		t.Fatal(err)
	}
	if td == nil {
		t.Fatal("expected teacher day, got nil")
	}
	if len(td.Lessons) != 1 {
		t.Fatalf("expected 1 lesson, got %d", len(td.Lessons))
	}

	days, err := repo.GroupDaysByRange(dayIdx, dayIdx, "63")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Errorf("expected 1 day in range, got %d", len(days))
	}

	os.Remove(tmpFile)
}
