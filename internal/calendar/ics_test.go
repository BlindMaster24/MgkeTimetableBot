package calendar

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func TestICSBuilderGroupDay(t *testing.T) {
	b := NewICSBuilder()

	subgroup := 0
	ltype := "лекция"
	teacher := "Иванов А.А."
	cabinet := "101"

	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{
				Subgroup: &subgroup,
				Lesson:   "Математика",
				Type:     &ltype,
				Teacher:  &teacher,
				Cabinet:  &cabinet,
			},
			nil,
			&model.GroupLessonExplain{
				Lesson: "Физика",
			},
		},
	}

	b.AddGroupDay(day, "63")

	if b.EventCount() != 2 {
		t.Errorf("expected 2 events, got %d", b.EventCount())
	}

	ics := b.Build()
	if !strings.HasPrefix(ics, "BEGIN:VCALENDAR") {
		t.Error("expected VCALENDAR header")
	}
	if !strings.HasSuffix(ics, "END:VCALENDAR\r\n") {
		t.Error("expected VCALENDAR footer")
	}
	if !strings.Contains(ics, "Математика") {
		t.Error("expected math lesson in output")
	}
	if !strings.Contains(ics, "Группа: 63") {
		t.Error("expected group in description")
	}
}

func TestICSBuilderTeacherDay(t *testing.T) {
	b := NewICSBuilder()

	day := model.TeacherDay{
		Day: "02.09.2025",
		Lessons: []model.TeacherLesson{
			{
				Group:  "63",
				Lesson: "Математика",
			},
		},
	}

	b.AddTeacherDay(day, "Иванов")

	if b.EventCount() != 1 {
		t.Errorf("expected 1 event, got %d", b.EventCount())
	}

	ics := b.Build()
	if !strings.Contains(ics, "Преподаватель: Иванов") {
		t.Error("expected teacher in description")
	}
}

func TestICSBuilderEmpty(t *testing.T) {
	b := NewICSBuilder()

	day := model.GroupDay{
		Day:     "01.09.2025",
		Lessons: []model.GroupLesson{nil},
	}

	b.AddGroupDay(day, "63")

	if b.EventCount() != 0 {
		t.Errorf("expected 0 events for nil lessons, got %d", b.EventCount())
	}
}

func TestICSBuilderInvalidDate(t *testing.T) {
	b := NewICSBuilder()

	day := model.GroupDay{
		Day: "invalid-date",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Test"},
		},
	}

	b.AddGroupDay(day, "63")

	if b.EventCount() != 0 {
		t.Errorf("expected 0 events for invalid date, got %d", b.EventCount())
	}
}

func TestICSBuilderMultipleDays(t *testing.T) {
	b := NewICSBuilder()

	d1 := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "A"},
		},
	}
	d2 := model.GroupDay{
		Day: "02.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "B"},
		},
	}

	b.AddGroupDay(d1, "63")
	b.AddGroupDay(d2, "63")

	if b.EventCount() != 2 {
		t.Errorf("expected 2 events, got %d", b.EventCount())
	}

	ics := b.Build()
	if !strings.Contains(ics, "VEVENT") {
		t.Error("expected VEVENT in output")
	}
	count := strings.Count(ics, "BEGIN:VEVENT")
	if count != 2 {
		t.Errorf("expected 2 VEVENT blocks, got %d", count)
	}
}
