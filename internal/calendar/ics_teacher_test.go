package calendar

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func TestICSBuilderTeacherDaySkipsLessonsBeyondTheBellSchedule(t *testing.T) {
	day := model.TeacherDay{
		Day: "01.09.2025",
		Lessons: []model.TeacherLesson{
			&model.TeacherLessonExplain{Lesson: "Математика", Group: "63"},
			&model.TeacherLessonExplain{Lesson: "Физика", Group: "64"},
			&model.TeacherLessonExplain{Lesson: "Химия", Group: "65"},
		},
	}

	b := fixedBuilder(weekdayCalls[:1], saturdayCalls, 1)
	b.AddTeacherDay(day, "Иванов И.И.")

	if b.EventCount() != 1 {
		t.Fatalf("lessons beyond the bell schedule must be skipped, got %d", b.EventCount())
	}
	built := b.Build()
	if !strings.Contains(built, "SUMMARY:Математика") {
		t.Error("expected the first lesson to stay")
	}
	if strings.Contains(built, "Физика") || strings.Contains(built, "Химия") {
		t.Error("lessons without a bell slot must not appear")
	}
}

func TestICSBuilderTeacherDaySkipsEmptyLessons(t *testing.T) {
	day := model.TeacherDay{
		Day: "01.09.2025",
		Lessons: []model.TeacherLesson{
			nil,
			&model.TeacherLessonExplain{Lesson: ""},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddTeacherDay(day, "Иванов И.И.")

	if b.EventCount() != 0 {
		t.Errorf("expected no events, got %d", b.EventCount())
	}
}

func TestICSBuilderTeacherDayWithoutSubgroupHasNoSuffix(t *testing.T) {
	day := model.TeacherDay{
		Day:     "01.09.2025",
		Lessons: []model.TeacherLesson{&model.TeacherLessonExplain{Lesson: "Математика", Group: "63"}},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddTeacherDay(day, "Иванов И.И.")

	built := b.Build()
	if strings.Contains(built, ", подгр.") {
		t.Errorf("a lesson without a subgroup must not gain one: %s", built)
	}
	if !strings.Contains(built, "DESCRIPTION:Группа: 63") {
		t.Errorf("expected the group in the description: %s", built)
	}
}

func TestICSBuilderSkipsADayWithAnUnreadableDate(t *testing.T) {
	day := model.TeacherDay{
		Day:     "31.02.2026",
		Lessons: []model.TeacherLesson{&model.TeacherLessonExplain{Lesson: "Математика"}},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddTeacherDay(day, "Иванов И.И.")

	if b.EventCount() != 0 {
		t.Fatalf("an impossible date must produce no events, got %d", b.EventCount())
	}
}

func TestICSBuilderSkipsUnreadableBellTimes(t *testing.T) {
	broken := [][2][2]string{{{"99:99", "99:99"}, {"99:99", "99:99"}}}
	day := model.GroupDay{
		Day:     "01.09.2025",
		Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "Математика"}},
	}

	b := fixedBuilder(broken, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	if b.EventCount() != 0 {
		t.Fatalf("a bell time that cannot be parsed must drop the lesson, got %d events", b.EventCount())
	}
}
