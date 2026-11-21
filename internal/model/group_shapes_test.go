package model

import "testing"

func TestAsSingleRejectsAnotherShape(t *testing.T) {
	if got := AsSingle(map[string]any{"lesson": "Математика"}); got != nil {
		t.Fatalf("a map is not a single lesson, got %+v", got)
	}
	if got := AsSingle([]*GroupLessonExplain{{Lesson: "Математика"}}); got != nil {
		t.Fatal("an array is not a single lesson")
	}
}

func TestAsArrayRejectsAnotherShape(t *testing.T) {
	if got := AsArray(map[string]any{"lesson": "Математика"}); got != nil {
		t.Fatalf("a map is not a lesson array, got %+v", got)
	}
	if got := AsArray(&GroupLessonExplain{Lesson: "Математика"}); got != nil {
		t.Fatal("a single lesson is not an array")
	}
}
