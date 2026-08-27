package v2

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func TestDiffGroupsEmpty(t *testing.T) {
	result := DiffGroups(model.Groups{}, model.Groups{}, 100)
	if len(result) != 0 {
		t.Errorf("expected empty diff, got %d", len(result))
	}
}

func TestDiffGroupsAdded(t *testing.T) {
	current := model.Groups{
		"63": &model.Group{Group: "63", Days: []model.GroupDay{
			{Day: "01.09.2025", Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "Math"}}},
		}},
	}
	result := DiffGroups(current, model.Groups{}, 100)
	if len(result) != 1 {
		t.Errorf("expected 1 diff line, got %d", len(result))
	}
}

func TestDiffGroupsRemoved(t *testing.T) {
	previous := model.Groups{
		"63": &model.Group{Group: "63", Days: []model.GroupDay{
			{Day: "01.09.2025", Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "Math"}}},
		}},
	}
	result := DiffGroups(model.Groups{}, previous, 100)
	if len(result) != 1 {
		t.Errorf("expected 1 diff line, got %d", len(result))
	}
}

func TestDiffGroupsLimit(t *testing.T) {
	current := model.Groups{}
	for i := 0; i < 50; i++ {
		key := string(rune('A'+i%26)) + string(rune('0'+i/26))
		current[key] = &model.Group{Group: key, Days: []model.GroupDay{
			{Day: "01.09.2025", Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "X"}}},
		}}
	}
	result := DiffGroups(current, model.Groups{}, 5)
	if len(result) > 5 {
		t.Errorf("expected at most 5, got %d", len(result))
	}
}

func TestDiffTeachersEmpty(t *testing.T) {
	result := DiffTeachers(model.Teachers{}, model.Teachers{}, 100)
	if len(result) != 0 {
		t.Errorf("expected empty diff, got %d", len(result))
	}
}
