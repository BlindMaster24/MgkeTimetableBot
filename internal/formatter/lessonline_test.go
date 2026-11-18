package formatter

import (
	"strings"
	"testing"
)

func TestTeacherLessonLinesCoverEveryPart(t *testing.T) {
	part := LessonPart{Group: "100", Lesson: "Математика", Type: "лек", Cabinet: "101", Comment: "перенос", Subgroup: 2}

	cases := []struct {
		name  string
		line  string
		parts []string
	}{
		{
			name:  "compact",
			line:  (&CompactFormatter{}).formatTeacherLesson(part),
			parts: []string{"2.", "100-Математика", "{101}"},
		},
		{
			name:  "default",
			line:  (&DefaultFormatter{}).formatTeacherLesson(part),
			parts: []string{"2.", "100-Математика", "(лек)", "{101}", "// перенос"},
		},
		{
			name:  "visual",
			line:  (&VisualFormatter{}).formatTeacherLesson(part, FormatOptions{}),
			parts: []string{"📚 100-Математика", "(лек)", "🏫 101", "// перенос"},
		},
	}
	for _, tc := range cases {
		for _, part := range tc.parts {
			if !strings.Contains(tc.line, part) {
				t.Errorf("%s: %q does not contain %q", tc.name, tc.line, part)
			}
		}
	}
}

func TestTeacherLessonLinesWithoutSubgroupOrCabinet(t *testing.T) {
	part := LessonPart{Group: "100", Lesson: "Математика"}

	if got := (&CompactFormatter{}).formatTeacherLesson(part); got != "100-Математика" {
		t.Fatalf("compact = %q, want the plain group-lesson pair", got)
	}
	if got := (&DefaultFormatter{}).formatTeacherLesson(part); got != "100-Математика" {
		t.Fatalf("default = %q, want the plain group-lesson pair", got)
	}
	if got := (&CompactFormatter{}).formatTeacherLesson(LessonPart{Group: "100", Lesson: "Физика", Subgroup: 1}); got != "1. 100-Физика" {
		t.Fatalf("compact with a subgroup = %q", got)
	}
}

func TestTeacherLessonsFallBackToTheEmptyLabel(t *testing.T) {
	opts := FormatOptions{Labels: map[string]string{"no_lessons": "нет пар"}}
	got := (&DefaultFormatter{}).formatTeacherLessons(nil, opts)
	if !strings.Contains(got, "нет пар") {
		t.Fatalf("an empty timetable must show the configured label, got %q", got)
	}
}
