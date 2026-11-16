package telegram

import "testing"

func TestLessonSubjectsReadsBothLessonShapes(t *testing.T) {
	group := map[string]any{"lesson": "Математика"}
	if got := lessonSubjects(group); len(got) != 1 || got[0] != "Математика" {
		t.Fatalf("a group lesson = %+v", got)
	}

	subgroups := []any{
		map[string]any{"lesson": "Физика", "subgroup": float64(1)},
		map[string]any{"lesson": ""},
		map[string]any{"subgroup": float64(2)},
		"не урок",
		map[string]any{"lesson": "Химия", "subgroup": float64(2)},
	}
	if got := lessonSubjects(subgroups); len(got) != 2 || got[0] != "Физика" || got[1] != "Химия" {
		t.Fatalf("subgroup lessons = %+v", got)
	}

	for _, broken := range []any{nil, "строка", 42, map[string]any{"lesson": 7}, []any{}} {
		if got := lessonSubjects(broken); len(got) != 0 {
			t.Fatalf("lessonSubjects(%#v) = %+v, want nothing", broken, got)
		}
	}
}

func TestSameSubjectsOfComparesIgnoringCaseAndSpaces(t *testing.T) {
	left := map[string]any{"lesson": "  Математика "}
	right := map[string]any{"lesson": "математика"}

	if got := sameSubjectsOf(left, right); len(got) != 1 || got[0] != "  Математика " {
		t.Fatalf("same subjects = %+v, want the left spelling kept", got)
	}

	if got := sameSubjectsOf(left, map[string]any{"lesson": "Физика"}); got != nil {
		t.Fatalf("different subjects must not match, got %+v", got)
	}
	if got := sameSubjectsOf(left, []any{}); got != nil {
		t.Fatalf("an empty side must not match, got %+v", got)
	}
}

func TestNormalizeSubjectKeyCollapsesTheSpelling(t *testing.T) {
	cases := map[string]string{
		"Математика":         "математика",
		"  МАТЕМАТИКА  ":     "математика",
		"Физика   и   химия": "физика и химия",
		"Информатика\tи ИКТ": "информатика и икт",
		"Английский\nязык":   "английский язык",
	}
	for input, want := range cases {
		if got := normalizeSubjectKey(input); got != want {
			t.Fatalf("normalizeSubjectKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatPairNamesTheBellScheduleSlot(t *testing.T) {
	calls := [][2][2]string{
		{{"08:00", "08:45"}, {"08:55", "09:40"}},
		{{"09:50", "10:35"}, {"10:45", "11:30"}},
	}

	if got := formatPair("2", calls); got != "2 (09:50-11:30)" {
		t.Fatalf("formatPair(2) = %q", got)
	}
	if got := formatPair("1", calls); got != "1 (08:00-09:40)" {
		t.Fatalf("formatPair(1) = %q", got)
	}
	for _, raw := range []string{"", "0", "3", "пара", "-1"} {
		if got := formatPair(raw, calls); got != raw {
			t.Fatalf("formatPair(%q) = %q, want it untouched", raw, got)
		}
	}
}
