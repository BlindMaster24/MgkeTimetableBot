package schedulediff

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

const fuzzDiffDate = "02.02.2026"

func decodeFuzzLesson(fields []string) timetable.Lesson {
	at := func(index int) string {
		if index < len(fields) {
			return strings.TrimSpace(fields[index])
		}
		return ""
	}

	lesson := timetable.Lesson{}
	if num, err := strconv.Atoi(at(0)); err == nil {
		lesson.Num = num
	}
	lesson.Subject = at(1)
	lesson.Type = at(2)
	lesson.Teacher = at(3)
	lesson.Group = at(4)
	lesson.Room = at(5)
	if subgroup, err := strconv.Atoi(at(6)); err == nil {
		lesson.Subgroup = subgroup
	}
	return lesson
}

func decodeFuzzDay(spec string) timetable.Day {
	day := timetable.Day{Date: fuzzDiffDate}
	for _, line := range strings.Split(spec, "\n") {
		lesson := decodeFuzzLesson(strings.Split(line, ";"))
		if lesson.Subject == "" {
			continue
		}
		day.Lessons = append(day.Lessons, lesson)
	}
	return day
}

func fuzzDiffJSON(t *testing.T, result Result) string {
	t.Helper()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func sameFuzzLesson(old, new timetable.Lesson) bool {
	return old.Subject == new.Subject && old.Type == new.Type && old.Group == new.Group
}

func emptyFuzzLesson(lesson timetable.Lesson) bool {
	return lesson.Num == 0 && lesson.Subject == "" && lesson.Type == "" && lesson.Teacher == "" &&
		lesson.Group == "" && lesson.Room == "" && lesson.Subgroup == 0 && lesson.Extra == nil
}

func fuzzDayHas(day timetable.Day, lesson timetable.Lesson) bool {
	for _, candidate := range day.Lessons {
		if candidate.Num == lesson.Num && sameFuzzLesson(candidate, lesson) &&
			candidate.Teacher == lesson.Teacher && candidate.Room == lesson.Room && candidate.Subgroup == lesson.Subgroup {
			return true
		}
	}
	return false
}

func checkFuzzDiff(t *testing.T, oldDay, newDay timetable.Day) {
	t.Helper()

	result := Diff(oldDay, newDay)
	if result.Empty() != (len(result.Changes) == 0) {
		t.Fatalf("Empty() = %v with %d changes", result.Empty(), len(result.Changes))
	}

	for _, kind := range []Kind{Added, Removed, Moved, RoomChanged, TeacherChanged, TimeChanged} {
		for _, change := range result.Of(kind) {
			if change.Kind != kind {
				t.Fatalf("Of(%d) returned a %d change", kind, change.Kind)
			}
		}
	}

	for _, change := range result.Changes {
		if !fuzzDayHas(oldDay, change.Old) && !emptyFuzzLesson(change.Old) {
			t.Fatalf("a %d change invented the old lesson %+v", change.Kind, change.Old)
		}
		if !fuzzDayHas(newDay, change.New) && !emptyFuzzLesson(change.New) {
			t.Fatalf("a %d change invented the new lesson %+v", change.Kind, change.New)
		}

		from, to := change.Old.Num, change.New.Num
		switch change.Kind {
		case Added:
			if !emptyFuzzLesson(change.Old) {
				t.Fatalf("an added lesson carries an old side: %+v", change)
			}
			from, to = change.New.Num, change.New.Num
		case Removed:
			if !emptyFuzzLesson(change.New) {
				t.Fatalf("a removed lesson carries a new side: %+v", change)
			}
			to = change.Old.Num
		case Moved, RoomChanged, TeacherChanged, TimeChanged:
			if emptyFuzzLesson(change.Old) || emptyFuzzLesson(change.New) {
				t.Fatalf("a %d change has an empty side: %+v", change.Kind, change)
			}
			if !sameFuzzLesson(change.Old, change.New) {
				t.Fatalf("a %d change does not pair the same lesson: %+v", change.Kind, change)
			}
		default:
			t.Fatalf("unknown change kind %d", change.Kind)
		}

		if change.From != from || change.To != to {
			t.Fatalf("a %d change reports %d-%d for lessons at %d and %d", change.Kind, change.From, change.To, change.Old.Num, change.New.Num)
		}
	}

	if first, second := fuzzDiffJSON(t, result), fuzzDiffJSON(t, Diff(oldDay, newDay)); first != second {
		t.Fatalf("diffing the same days twice changed the result:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func FuzzDiffKeepsChangesWithinTheDays(f *testing.F) {
	pairs := [][2]string{
		{"", ""},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;0\n2;Физика;Пр;Петров П.П.;100;202;0",
			"1;Математика;Лек;Иванов И.И.;100;201;0\n2;Физика;Пр;Петров П.П.;100;202;0",
		},
		{
			"2;Физика;Пр;Петров П.П.;100;202;0",
			"4;Физика;Пр;Петров П.П.;100;202;0",
		},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;0",
			"1;Математика;Лек;Иванов И.И.;100;303;0",
		},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;0",
			"1;Математика;Лек;Сидоров С.С.;100;201;0",
		},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;0",
			"",
		},
		{
			"",
			"3;Химия;ЛР;Сидоров С.С.;100;305;0",
		},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;0",
			"1;Математика;Пр;Иванов И.И.;100;201;0",
		},
		{
			"1;Математика;Лек;Иванов И.И.;100;201;1\n1;Математика;Лек;Петров П.П.;100;202;2",
			"1;Математика;Лек;Иванов И.И.;100;201;1\n1;Математика;Лек;Петров П.П.;100;404;2",
		},
	}
	for _, pair := range pairs {
		f.Add(pair[0], pair[1])
	}

	f.Fuzz(func(t *testing.T, old, new string) {
		oldDay := decodeFuzzDay(old)
		newDay := decodeFuzzDay(new)
		if oldDay.Date != newDay.Date {
			t.Fatalf("the fixture days drifted apart: %q and %q", oldDay.Date, newDay.Date)
		}

		checkFuzzDiff(t, oldDay, newDay)

		reversed := Diff(newDay, oldDay)
		for _, change := range reversed.Of(Added) {
			if !emptyFuzzLesson(change.Old) {
				t.Fatalf("an added lesson carries an old side: %+v", change)
			}
		}
		for _, change := range reversed.Of(Removed) {
			if !emptyFuzzLesson(change.New) {
				t.Fatalf("a removed lesson carries a new side: %+v", change)
			}
		}
	})
}
