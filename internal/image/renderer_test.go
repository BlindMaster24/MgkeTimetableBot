package image

import (
	"os"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func TestBuildDayTable(t *testing.T) {
	subgroup := 1
	ltype := "лекция"
	teacher := "Иванов"
	cabinet := "101"
	comment := "2 часа"

	lessons := []model.GroupLesson{
		&model.GroupLessonExplain{
			Subgroup: &subgroup,
			Lesson:   "Математика",
			Type:     &ltype,
			Teacher:  &teacher,
			Cabinet:  &cabinet,
			Comment:  &comment,
		},
		nil,
		[]*model.GroupLessonExplain{
			{Lesson: "А"},
			{Lesson: "Б"},
		},
	}

	day := model.GroupDay{
		Day:     "01.09.2025",
		Lessons: lessons,
	}

	table := BuildDayTable(day)

	if table.Date != "01.09.2025" {
		t.Errorf("expected date 01.09.2025, got %s", table.Date)
	}
	if len(table.Lessons) != 3 {
		t.Errorf("expected 3 lesson rows, got %d", len(table.Lessons))
	}
	if table.Lessons[0].Index != 1 {
		t.Errorf("expected index 1, got %d", table.Lessons[0].Index)
	}
	if table.Lessons[1].Content != "-" {
		t.Errorf("expected dash for nil lesson, got %s", table.Lessons[1].Content)
	}
}

func TestBuildTeacherDayTable(t *testing.T) {
	ltype := "пр-ка"
	day := model.TeacherDay{
		Day: "01.09.2025",
		Lessons: []model.TeacherLesson{
			{
				Group:  "63",
				Lesson: "Математика",
				Type:   &ltype,
			},
		},
	}

	table := BuildTeacherDayTable(day)

	if len(table.Lessons) != 1 {
		t.Errorf("expected 1 lesson row, got %d", len(table.Lessons))
	}
}

func TestFormatGroupLessonNil(t *testing.T) {
	result := formatGroupLesson(nil)
	if result != "-" {
		t.Errorf("expected '-', got %s", result)
	}
}

func TestFormatSingleGroupLesson(t *testing.T) {
	subgroup := 2
	ltype := "лекция"
	teacher := "Петров"
	cabinet := "202"

	e := &model.GroupLessonExplain{
		Subgroup: &subgroup,
		Lesson:   "Физика",
		Type:     &ltype,
		Teacher:  &teacher,
		Cabinet:  &cabinet,
	}

	result := formatSingleGroupLesson(e)
	if result == "" || result == "-" {
		t.Error("expected non-empty formatted lesson")
	}
}

func TestFormatTeacherLessonNil(t *testing.T) {
	result := formatTeacherLesson(nil)
	if result != "-" {
		t.Errorf("expected '-', got %s", result)
	}
}

func TestWeekdayName(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"01.09.2025", "Понедельник"},
		{"02.09.2025", "Вторник"},
		{"06.09.2025", "Суббота"},
		{"invalid", ""},
	}

	for _, c := range cases {
		got := weekdayName(c.date)
		if got != c.want {
			t.Errorf("weekdayName(%s) = %s, want %s", c.date, got, c.want)
		}
	}
}

func TestRenderDayTablesEmpty(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)

	_, err := r.RenderDayTables("Test", nil)
	if err == nil {
		t.Error("expected error for empty tables")
	}
}

func TestRendererOutputDir(t *testing.T) {
	dir := t.TempDir()
	_ = NewRenderer(dir)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected output dir to exist")
	}
}
