package schedulediff

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func lesson(num int, subject string) timetable.Lesson {
	return timetable.Lesson{Num: num, Subject: subject}
}

func TestDiffIdenticalDaysStaySilent(t *testing.T) {
	day := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		{Num: 1, Subject: "Математика", Teacher: "Иванов И.И.", Room: "101"},
		{Num: 2, Subject: "Физика", Subgroup: 1},
		{Num: 2, Subject: "Физика", Subgroup: 2},
	}}
	result := Diff(day, day)
	if !result.Empty() {
		t.Fatalf("changes = %+v, want none", result.Changes)
	}
	if len(result.Of(Added)) != 0 {
		t.Errorf("Of(Added) = %d, want none", len(result.Of(Added)))
	}
}

func TestDiffEmptyDaysStaySilent(t *testing.T) {
	if result := Diff(timetable.Day{}, timetable.Day{}); !result.Empty() {
		t.Fatalf("changes = %+v, want none", result.Changes)
	}
}

func TestDiffAspectChanges(t *testing.T) {
	cases := []struct {
		name string
		old  timetable.Lesson
		new  timetable.Lesson
		kind Kind
	}{
		{"room", lesson(1, "М"), timetable.Lesson{Num: 1, Subject: "М", Room: "202"}, RoomChanged},
		{"teacher", lesson(1, "М"), timetable.Lesson{Num: 1, Subject: "М", Teacher: "Петров П.П."}, TeacherChanged},
		{"type", timetable.Lesson{Num: 1, Subject: "М", Type: "Лек"}, timetable.Lesson{Num: 1, Subject: "М", Type: "Пр"}, Removed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{tc.old}}
			new := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{tc.new}}
			result := Diff(old, new)
			if len(result.Of(tc.kind)) != 1 {
				t.Fatalf("kinds = %+v, want one %d", result.Changes, tc.kind)
			}
		})
	}
}

func TestDiffTimeChangeComesFromExtras(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{{Num: 1, Subject: "М", Extra: map[string]any{"time": "09:00"}}}}
	new := timetable.Day{Lessons: []timetable.Lesson{{Num: 1, Subject: "М", Extra: map[string]any{"time": "10:00"}}}}
	result := Diff(old, new)
	changes := result.Of(TimeChanged)
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want one time change", result.Changes)
	}
	if changes[0].Old.Extra["time"] != "09:00" || changes[0].New.Extra["time"] != "10:00" {
		t.Errorf("change = %+v, want old and new times", changes[0])
	}

	plain := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М")}}
	if result := Diff(plain, plain); !result.Empty() {
		t.Errorf("changes = %+v, want none without times", result.Changes)
	}
	odd := timetable.Day{Lessons: []timetable.Lesson{{Num: 1, Subject: "М", Extra: map[string]any{"time": 42}}}}
	if result := Diff(odd, plain); !result.Empty() {
		t.Errorf("changes = %+v, want none for a non-string time", result.Changes)
	}
}

func TestDiffAddedAndRemoved(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М"), lesson(2, "Ф")}}
	new := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М"), lesson(3, "Х")}}
	result := Diff(old, new)
	removed := result.Of(Removed)
	if len(removed) != 1 || removed[0].Old.Subject != "Ф" || removed[0].From != 2 || removed[0].To != 2 {
		t.Errorf("removed = %+v, want Ф at 2", removed)
	}
	added := result.Of(Added)
	if len(added) != 1 || added[0].New.Subject != "Х" || added[0].From != 3 || added[0].To != 3 {
		t.Errorf("added = %+v, want Х at 3", added)
	}
}

func TestDiffMovedLesson(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М"), lesson(2, "Ф")}}
	new := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М"), lesson(4, "Ф")}}
	result := Diff(old, new)
	moved := result.Of(Moved)
	if len(moved) != 1 {
		t.Fatalf("changes = %+v, want one move", result.Changes)
	}
	if moved[0].From != 2 || moved[0].To != 4 || moved[0].New.Subject != "Ф" {
		t.Errorf("move = %+v, want Ф from 2 to 4", moved[0])
	}
	if result.Empty() {
		t.Errorf("Empty() = true, want false with changes present")
	}
}

func TestDiffMovedLessonKeepsAspectChanges(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{{Num: 2, Subject: "Ф", Teacher: "А", Room: "10", Extra: map[string]any{"time": "09:00"}}}}
	new := timetable.Day{Lessons: []timetable.Lesson{{Num: 4, Subject: "Ф", Teacher: "Б", Room: "20", Extra: map[string]any{"time": "10:00"}}}}
	result := Diff(old, new)
	for _, kind := range []Kind{Moved, RoomChanged, TeacherChanged, TimeChanged} {
		if len(result.Of(kind)) != 1 {
			t.Errorf("Of(%d) = %d, want 1 (changes=%+v)", kind, len(result.Of(kind)), result.Changes)
		}
	}
}

func TestDiffMovePrefersFirstCandidate(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М")}}
	new := timetable.Day{Lessons: []timetable.Lesson{lesson(2, "М"), lesson(3, "М")}}
	result := Diff(old, new)
	moved := result.Of(Moved)
	added := result.Of(Added)
	if len(moved) != 1 || moved[0].To != 2 {
		t.Errorf("moved = %+v, want the first candidate", moved)
	}
	if len(added) != 1 || added[0].To != 3 {
		t.Errorf("added = %+v, want the leftover at 3", added)
	}
}

func TestDiffReplacementAtSameSlot(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "М")}}
	new := timetable.Day{Lessons: []timetable.Lesson{lesson(1, "Ф")}}
	result := Diff(old, new)
	if len(result.Of(Moved)) != 0 {
		t.Errorf("changes = %+v, want no move for a replacement", result.Changes)
	}
	if len(result.Of(Removed)) != 1 || len(result.Of(Added)) != 1 {
		t.Errorf("changes = %+v, want remove plus add", result.Changes)
	}
}

func TestDiffGroupChangeIsReplacement(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{{Num: 1, Subject: "М", Group: "100"}}}
	new := timetable.Day{Lessons: []timetable.Lesson{{Num: 2, Subject: "М", Group: "200"}}}
	result := Diff(old, new)
	if len(result.Of(Moved)) != 0 {
		t.Errorf("changes = %+v, want no move across groups", result.Changes)
	}
	if len(result.Of(Removed)) != 1 || len(result.Of(Added)) != 1 {
		t.Errorf("changes = %+v, want remove plus add", result.Changes)
	}
}

func TestDiffDuplicateSlotsPairInOrder(t *testing.T) {
	old := timetable.Day{Lessons: []timetable.Lesson{
		{Num: 1, Subject: "М", Subgroup: 1, Room: "10"},
		{Num: 1, Subject: "М", Subgroup: 1, Room: "11"},
	}}
	new := timetable.Day{Lessons: []timetable.Lesson{
		{Num: 1, Subject: "М", Subgroup: 1, Room: "10"},
		{Num: 1, Subject: "М", Subgroup: 1, Room: "12"},
	}}
	result := Diff(old, new)
	changed := result.Of(RoomChanged)
	if len(changed) != 1 || changed[0].New.Room != "12" {
		t.Errorf("changes = %+v, want one room change pairing in order", result.Changes)
	}
}

func TestDiffOfFiltersEveryKind(t *testing.T) {
	result := Result{Changes: []Change{
		{Kind: Added}, {Kind: Removed}, {Kind: Moved},
		{Kind: RoomChanged}, {Kind: TeacherChanged}, {Kind: TimeChanged},
	}}
	for _, kind := range []Kind{Added, Removed, Moved, RoomChanged, TeacherChanged, TimeChanged} {
		if len(result.Of(kind)) != 1 {
			t.Errorf("Of(%d) = %d, want 1", kind, len(result.Of(kind)))
		}
	}
	if len(result.Of(Kind(99))) != 0 {
		t.Errorf("Of(unknown) matched, want none")
	}
}
