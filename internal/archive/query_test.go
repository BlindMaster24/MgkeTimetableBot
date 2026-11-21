package archive

import (
	"context"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func seedDays(t *testing.T, repo *Repository, entries []AppendDay) {
	t.Helper()
	if err := repo.AppendDays(context.Background(), entries); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestDistinctStringsSkipsEmptyKeys(t *testing.T) {
	repo := newTestRepo(t)
	seedDays(t, repo, []AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "group", Value: "100", Day: map[string]any{"day": "08.09.2026", "lessons": []any{}}},
		{Type: "group", Value: "200", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
	})

	groups, err := repo.Groups(context.Background())
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	if len(groups) != 2 || groups[0] != "100" || groups[1] != "200" {
		t.Fatalf("groups = %v, want the two distinct groups in order", groups)
	}

	teachers, err := repo.Teachers(context.Background())
	if err != nil {
		t.Fatalf("Teachers: %v", err)
	}
	if len(teachers) != 1 || teachers[0] != "Иванов И.И." {
		t.Fatalf("teachers = %v, want the single teacher", teachers)
	}
}

func TestDistinctStringsReportsAClosedDatabase(t *testing.T) {
	repo := newTestRepo(t)
	repo.Close()

	if _, err := repo.Groups(context.Background()); err == nil {
		t.Fatal("Groups must report a closed database")
	}
	if _, err := repo.Teachers(context.Background()); err == nil {
		t.Fatal("Teachers must report a closed database")
	}
}

func TestGroupAndTeacherDaysRespectTheFromDay(t *testing.T) {
	repo := newTestRepo(t)
	seedDays(t, repo, []AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "group", Value: "100", Day: map[string]any{"day": "08.09.2026", "lessons": []any{}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: map[string]any{"day": "08.09.2026", "lessons": []any{}}},
	})

	all, err := repo.GroupDays(context.Background(), "100", nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("GroupDays without a bound = %d days (%v), want 2", len(all), err)
	}
	from := int64(utils.DayIndexFromDate(time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)))
	part, err := repo.GroupDays(context.Background(), "100", &from)
	if err != nil {
		t.Fatalf("GroupDays: %v", err)
	}
	if len(part) != 1 || part[0].Day != "08.09.2026" {
		t.Fatalf("GroupDays from day %d = %+v, want only 08.09.2026", from, part)
	}

	teacherAll, err := repo.TeacherDays(context.Background(), "Иванов И.И.", nil)
	if err != nil || len(teacherAll) != 2 {
		t.Fatalf("TeacherDays without a bound = %d days (%v), want 2", len(teacherAll), err)
	}
	teacherPart, err := repo.TeacherDays(context.Background(), "Иванов И.И.", &from)
	if err != nil {
		t.Fatalf("TeacherDays: %v", err)
	}
	if len(teacherPart) != 1 || teacherPart[0].Day != "08.09.2026" {
		t.Fatalf("TeacherDays from day %d = %+v, want only 08.09.2026", from, teacherPart)
	}
}

func TestWeekIndexBoundsCoverTheStoredDays(t *testing.T) {
	repo := newTestRepo(t)
	empty, err := repo.WeekIndexBounds(context.Background())
	if err != nil {
		t.Fatalf("WeekIndexBounds on an empty archive: %v", err)
	}
	if empty.Min != empty.Max {
		t.Fatalf("empty bounds = %+v, want a single week", empty)
	}

	seedDays(t, repo, []AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "group", Value: "100", Day: map[string]any{"day": "14.09.2026", "lessons": []any{}}},
	})
	bounds, err := repo.WeekIndexBounds(context.Background())
	if err != nil {
		t.Fatalf("WeekIndexBounds: %v", err)
	}
	if bounds.Max != bounds.Min+1 {
		t.Fatalf("bounds = %+v, want two consecutive weeks", bounds)
	}
}

func TestBrokenLessonPayloadIsReported(t *testing.T) {
	repo := newTestRepo(t)
	if _, err := repo.db.Exec(`INSERT INTO timetable_archive (day, "group", teacher, data) VALUES (?, ?, ?, ?)`, 20725, "100", "Иванов И.И.", "{not json"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GroupDays(context.Background(), "100", nil); err == nil {
		t.Fatal("GroupDays must report a payload it cannot read")
	}
	if _, err := repo.TeacherDays(context.Background(), "Иванов И.И.", nil); err == nil {
		t.Fatal("TeacherDays must report a payload it cannot read")
	}
}
