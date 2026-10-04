package cache

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
)

func diffLesson(subject, room, teacher, time string) map[string]any {
	return map[string]any{"lesson": subject, "cabinet": room, "teacher": teacher, "time": time}
}

func diffDay(date string, lessons ...any) map[string]any {
	list := make([]any, 0, len(lessons))
	for _, l := range lessons {
		list = append(list, l)
	}
	return map[string]any{"day": date, "lessons": list}
}

func TestDiffDayLessonsSurfacesEveryKind(t *testing.T) {
	date := "07.09.2026"
	cases := []struct {
		name string
		old  map[string]any
		new  map[string]any
		kind schedulediff.Kind
	}{
		{
			"added",
			diffDay(date, diffLesson("Math", "101", "A", "09:00")),
			diffDay(date, diffLesson("Math", "101", "A", "09:00"), diffLesson("Physics", "102", "B", "10:00")),
			schedulediff.Added,
		},
		{
			"removed",
			diffDay(date, diffLesson("Math", "101", "A", "09:00"), diffLesson("Physics", "102", "B", "10:00")),
			diffDay(date, diffLesson("Math", "101", "A", "09:00")),
			schedulediff.Removed,
		},
		{
			"moved",
			diffDay(date, diffLesson("Math", "101", "A", "09:00"), diffLesson("Physics", "102", "B", "10:00")),
			diffDay(date, diffLesson("Physics", "102", "B", "10:00"), diffLesson("Math", "101", "A", "09:00")),
			schedulediff.Moved,
		},
		{
			"room",
			diffDay(date, diffLesson("Math", "101", "A", "09:00")),
			diffDay(date, diffLesson("Math", "205", "A", "09:00")),
			schedulediff.RoomChanged,
		},
		{
			"teacher",
			diffDay(date, diffLesson("Math", "101", "A", "09:00")),
			diffDay(date, diffLesson("Math", "101", "B", "09:00")),
			schedulediff.TeacherChanged,
		},
		{
			"time",
			diffDay(date, diffLesson("Math", "101", "A", "09:00")),
			diffDay(date, diffLesson("Math", "101", "A", "10:00")),
			schedulediff.TimeChanged,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := diffDayLessons(tc.old, tc.new)
			if !ok {
				t.Fatal("days must be comparable")
			}
			if len(result.Of(tc.kind)) == 0 {
				t.Fatalf("result = %+v, want kind %d", result.Changes, tc.kind)
			}
		})
	}
}

func TestDiffDayLessonsIdenticalDaysStaySilent(t *testing.T) {
	day := diffDay("07.09.2026", diffLesson("Math", "101", "A", "09:00"))
	result, ok := diffDayLessons(day, diffDay("07.09.2026", diffLesson("Math", "101", "A", "09:00")))
	if !ok {
		t.Fatal("days must be comparable")
	}
	if !result.Empty() {
		t.Fatalf("result = %+v, want none", result.Changes)
	}
	if dayLessonsChanged(day, diffDay("07.09.2026", diffLesson("Math", "101", "A", "09:00"))) {
		t.Error("identical days must not count as changed")
	}
}

func TestDayLessonsChangedKeepsCommentEditsVisible(t *testing.T) {
	old := map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any{"lesson": "Math", "comment": "old"},
	}}
	new := map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any{"lesson": "Math", "comment": "new"},
	}}
	if !dayLessonsChanged(old, new) {
		t.Error("comment-only edit must stay visible like the hash comparison did")
	}
	if result, ok := diffDayLessons(old, new); !ok || !result.Empty() {
		t.Fatalf("result = %+v %v, want empty but comparable", result.Changes, ok)
	}
}

func TestDiffDayLessonsFallsBackOnUnknownShapes(t *testing.T) {
	broken := map[string]any{"day": "07.09.2026", "lessons": []any{"nope"}}
	plain := diffDay("07.09.2026", diffLesson("Math", "101", "A", "09:00"))
	if _, ok := diffDayLessons(broken, plain); ok {
		t.Error("string lesson must not be comparable")
	}
	if !dayLessonsChanged(broken, plain) {
		t.Error("fallback must flag differing lessons as changed")
	}
	if dayLessonsChanged(broken, map[string]any{"day": "07.09.2026", "lessons": []any{"nope"}}) {
		t.Error("fallback must stay silent on identical lessons")
	}
	if _, ok := diffDayLessons(map[string]any{"day": "tomorrow"}, plain); ok {
		t.Error("bad date must not be comparable")
	}
	if _, ok := diffDayLessons(nil, plain); ok {
		t.Error("nil day must not be comparable")
	}
	if _, ok := diffDayLessons(map[string]any{"day": "07.09.2026", "lessons": map[string]any{}}, plain); ok {
		t.Error("non-array lessons must not be comparable")
	}
	if _, ok := diffDayLessons(map[string]any{"day": "07.09.2026", "lessons": []any{nil}}, plain); !ok {
		t.Error("nil entries must be skipped like the typed converter does")
	}
	subgroup := map[string]any{"day": "07.09.2026", "lessons": []any{
		[]any{map[string]any{"lesson": "Math", "subgroup": float64(1)}, map[string]any{"lesson": "Math", "subgroup": 2}},
	}}
	if _, ok := diffDayLessons(subgroup, subgroup); !ok {
		t.Error("subgroup arrays must be comparable")
	}
	nonString := map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any{"lesson": 42},
	}}
	if result, ok := diffDayLessons(nonString, plain); !ok || result.Empty() {
		t.Error("non-string subjects must degrade gracefully and still count as changed")
	}
	if _, ok := diffDayLessons(map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any{"lesson": "Math", "subgroup": "first"},
	}}, plain); ok {
		t.Error("string subgroup must not be comparable")
	}
	if _, ok := diffDayLessons(map[string]any{"day": "07.09.2026", "lessons": []any{
		[]any{},
	}}, plain); !ok {
		t.Error("empty arrays must be skipped")
	}
	if _, ok := diffDayLessons(plain, broken); ok {
		t.Error("broken new day must not be comparable")
	}
	arrayErr := map[string]any{"day": "07.09.2026", "lessons": []any{
		[]any{"nope"},
	}}
	if _, ok := diffDayLessons(arrayErr, plain); ok {
		t.Error("broken array entries must not be comparable")
	}
	singleErr := map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any{"lesson": "Math", "subgroup": true},
	}}
	if _, ok := diffDayLessons(singleErr, plain); ok {
		t.Error("broken single entries must not be comparable")
	}
	nilMap := map[string]any{"day": "07.09.2026", "lessons": []any{
		map[string]any(nil),
	}}
	if _, ok := diffDayLessons(nilMap, plain); ok {
		t.Error("nil lesson maps must not be comparable")
	}
}
