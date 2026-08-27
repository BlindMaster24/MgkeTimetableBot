package v2

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func TestValidateGroupsEmpty(t *testing.T) {
	result := ValidateGroups(model.Groups{}, 10, 5)
	if result.OK {
		t.Error("expected not OK for empty groups")
	}
}

func TestValidateGroupsValid(t *testing.T) {
	groups := model.Groups{
		"63": &model.Group{
			Group: "63",
			Days: []model.GroupDay{
				{Day: "01.09.2025", Lessons: []model.GroupLesson{
					&model.GroupLessonExplain{Lesson: "Math"},
				}},
				{Day: "02.09.2025", Lessons: []model.GroupLesson{
					&model.GroupLessonExplain{Lesson: "Phys"},
				}},
			},
		},
	}
	result := ValidateGroups(groups, 10, 5)
	if !result.OK {
		t.Errorf("expected OK, got errors: %v", result.Errors)
	}
}

func TestValidateGroupsTooManyLessons(t *testing.T) {
	lessons := make([]model.GroupLesson, 15)
	for i := range lessons {
		lessons[i] = &model.GroupLessonExplain{Lesson: "X"}
	}
	groups := model.Groups{
		"G": &model.Group{
			Group: "G",
			Days:  []model.GroupDay{{Day: "01.01.2025", Lessons: lessons}},
		},
	}
	result := ValidateGroups(groups, 10, 5)
	if result.OK {
		t.Error("expected not OK for too many lessons")
	}
}

func TestValidateTeachersEmpty(t *testing.T) {
	result := ValidateTeachers(model.Teachers{}, 10, 5)
	if result.OK {
		t.Error("expected not OK for empty teachers")
	}
}

func TestValidateTeachersValid(t *testing.T) {
	teachers := model.Teachers{
		"Иванов": &model.Teacher{
			Teacher: "Иванов",
			Days: []model.TeacherDay{
				{Day: "01.09.2025", Lessons: []model.TeacherLesson{
					{Lesson: "Math", Group: "63"},
				}},
			},
		},
	}
	result := ValidateTeachers(teachers, 10, 5)
	if !result.OK {
		t.Errorf("expected OK, got errors: %v", result.Errors)
	}
}

func TestValidateGroupsDuplicateDay(t *testing.T) {
	groups := model.Groups{
		"G": &model.Group{
			Group: "G",
			Days: []model.GroupDay{
				{Day: "01.09.2025", Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "A"}}},
				{Day: "01.09.2025", Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "B"}}},
			},
		},
	}
	result := ValidateGroups(groups, 10, 5)
	if result.OK {
		t.Error("expected not OK for duplicate days")
	}
}
