package model

import "testing"

func TestAsSingle(t *testing.T) {
	e := &GroupLessonExplain{Lesson: "Math"}
	result := AsSingle(e)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Lesson != "Math" {
		t.Errorf("expected Math, got %s", result.Lesson)
	}
}

func TestAsSingleNil(t *testing.T) {
	if AsSingle(nil) != nil {
		t.Error("expected nil")
	}
}

func TestAsArray(t *testing.T) {
	arr := []*GroupLessonExplain{
		{Lesson: "A"},
		{Lesson: "B"},
	}
	result := AsArray(arr)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestAsArrayNil(t *testing.T) {
	if AsArray(nil) != nil {
		t.Error("expected nil")
	}
}

func TestIsLessonArray(t *testing.T) {
	arr := []*GroupLessonExplain{{Lesson: "X"}}
	if !IsLessonArray(arr) {
		t.Error("expected true for array")
	}
	if IsLessonArray(nil) {
		t.Error("expected false for nil")
	}
}
