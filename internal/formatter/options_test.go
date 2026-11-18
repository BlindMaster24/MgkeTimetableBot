package formatter

import "testing"

func TestGetByIndexFallsBackToTheFirst(t *testing.T) {
	if got := GetByIndex(-1); got != AllFormatters[0] {
		t.Fatalf("negative index = %v, want the default formatter", got.Name())
	}
	if got := GetByIndex(len(AllFormatters)); got != AllFormatters[0] {
		t.Fatalf("index past the end = %v, want the default formatter", got.Name())
	}
	if got := GetByIndex(2); got != AllFormatters[2] {
		t.Fatalf("index 2 = %v, want the third formatter", got.Name())
	}
}

func TestGetFullTeacherNameUsesTheMapWhenItKnowsTheName(t *testing.T) {
	opts := FormatOptions{}
	if got := opts.getFullTeacherName("Иванов И.И."); got != "Иванов И.И." {
		t.Fatalf("without a map = %q, want the short name", got)
	}

	opts.TeacherNames = map[string]string{"Иванов И.И.": "Иванов Иван Иванович"}
	if got := opts.getFullTeacherName("Иванов И.И."); got != "Иванов Иван Иванович" {
		t.Fatalf("with a map = %q, want the full name", got)
	}
	if got := opts.getFullTeacherName("Петров П.П."); got != "Петров П.П." {
		t.Fatalf("unknown name = %q, want the short name", got)
	}
}

func TestAllEqualHandlesShortAndMixedLists(t *testing.T) {
	lesson := func(p LessonPart) string { return p.Lesson }

	if !allEqual(lesson, nil) || !allEqual(lesson, []LessonPart{{Lesson: "Математика"}}) {
		t.Fatal("a list shorter than two parts is always equal")
	}
	same := []LessonPart{{Lesson: "Математика"}, {Lesson: "Математика"}}
	if !allEqual(lesson, same) {
		t.Fatal("two identical parts must compare equal")
	}
	if allEqual(lesson, []LessonPart{{Lesson: "Математика"}, {Lesson: "Физика"}}) {
		t.Fatal("different parts must differ")
	}
}

func TestIsSubgroupList(t *testing.T) {
	if !isSubgroupList([]any{}) {
		t.Fatal("a slice of subgroups must be recognised")
	}
	if isSubgroupList(map[string]any{}) || isSubgroupList(nil) {
		t.Fatal("a single lesson is not a subgroup list")
	}
}

func TestGroupLessonOptionsRevealRepeatedFields(t *testing.T) {
	pair := []LessonPart{
		{Subgroup: 1, Lesson: "Математика", Type: "лек", Teacher: "Иванов", Cabinet: "101", Comment: "a"},
		{Subgroup: 2, Lesson: "Математика", Type: "лек", Teacher: "Иванов", Cabinet: "101", Comment: "a"},
	}
	show := groupLessonOptions(pair, true)
	for _, key := range []string{"subgroup", "lesson", "type", "teacher", "cabinet", "comment"} {
		if key != "subgroup" && !show[key] {
			t.Errorf("%s: a repeated field must stay visible on the first line", key)
		}
	}
	if show["subgroup"] {
		t.Error("subgroup: the group line must not repeat the subgroup number")
	}

	different := []LessonPart{
		{Subgroup: 1, Lesson: "Математика", Type: "лек", Teacher: "Иванов", Cabinet: "101", Comment: "a"},
		{Subgroup: 2, Lesson: "Физика", Type: "пр", Teacher: "Петров", Cabinet: "202", Comment: "b"},
	}
	show = groupLessonOptions(different, true)
	for _, key := range []string{"lesson", "type", "teacher", "cabinet", "comment"} {
		if show[key] {
			t.Errorf("%s: a field that differs must be repeated on every subgroup line", key)
		}
	}

	flat := groupLessonOptions(different, false)
	for key, visible := range flat {
		if !visible && key != "subgroup" {
			t.Errorf("%s: a flat lesson must show every field", key)
		}
	}
}

func TestReverseGroupLessonOptionsFlipsEveryFlag(t *testing.T) {
	show := map[string]bool{"subgroup": true, "lesson": false, "cabinet": true}
	reversed := reverseGroupLessonOptions(show)
	if reversed["subgroup"] || !reversed["lesson"] || reversed["cabinet"] {
		t.Fatalf("reversed = %v, want every flag flipped", reversed)
	}
	if !show["subgroup"] {
		t.Fatal("the source map must stay untouched")
	}
}
