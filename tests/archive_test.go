package tests

import (
	"os"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	
)

func TestDayIndexConversion(t *testing.T) {
	cases := []struct {
		date string
		idx  int64
	}{
		{"01.01.1970", 0},
		{"02.01.1970", 1},
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
}

func TestArchiveRepository(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	repo, err := archive.New(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	db := repo.DB()

	db.Exec(`CREATE TABLE IF NOT EXISTS timetable_archive (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		day INTEGER NOT NULL,
		"group" TEXT,
		teacher TEXT,
		data TEXT NOT NULL,
		UNIQUE(day, "group"),
		UNIQUE(day, teacher)
	)`)

	db.Exec(`CREATE INDEX IF NOT EXISTS idx_group_day ON timetable_archive("group", day)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_teacher_day ON timetable_archive(teacher, day)`)

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
