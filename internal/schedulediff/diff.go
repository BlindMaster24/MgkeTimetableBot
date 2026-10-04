package schedulediff

import (
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type Kind int

const (
	Added Kind = iota + 1
	Removed
	Moved
	RoomChanged
	TeacherChanged
	TimeChanged
)

type Change struct {
	Kind Kind
	Old  timetable.Lesson
	New  timetable.Lesson
	From int
	To   int
}

type Result struct {
	Changes []Change
}

func (r Result) Empty() bool {
	return len(r.Changes) == 0
}

func (r Result) Of(kind Kind) []Change {
	var out []Change
	for _, change := range r.Changes {
		if change.Kind == kind {
			out = append(out, change)
		}
	}
	return out
}

type lessonKey struct {
	num      int
	subgroup int
}

func keyOf(lesson timetable.Lesson) lessonKey {
	return lessonKey{num: lesson.Num, subgroup: lesson.Subgroup}
}

func lessonTime(lesson timetable.Lesson) string {
	if lesson.Extra == nil {
		return ""
	}
	text, _ := lesson.Extra["time"].(string)
	return text
}

func sameLesson(old, new timetable.Lesson) bool {
	return old.Subject == new.Subject && old.Type == new.Type && old.Group == new.Group
}

func Diff(oldDay, newDay timetable.Day) Result {
	byKey := make(map[lessonKey][]int, len(newDay.Lessons))
	for i, lesson := range newDay.Lessons {
		key := keyOf(lesson)
		byKey[key] = append(byKey[key], i)
	}
	consumed := make([]bool, len(newDay.Lessons))

	var result Result
	var unmatchedOld []timetable.Lesson
	var unmatchedNew []timetable.Lesson

	take := func(key lessonKey) (timetable.Lesson, bool) {
		for _, i := range byKey[key] {
			if !consumed[i] {
				consumed[i] = true
				return newDay.Lessons[i], true
			}
		}
		return timetable.Lesson{}, false
	}

	for _, old := range oldDay.Lessons {
		matched, ok := take(keyOf(old))
		if !ok {
			unmatchedOld = append(unmatchedOld, old)
			continue
		}
		if !sameLesson(old, matched) {
			unmatchedOld = append(unmatchedOld, old)
			unmatchedNew = append(unmatchedNew, matched)
			continue
		}
		if old.Room != matched.Room {
			result.Changes = append(result.Changes, Change{Kind: RoomChanged, Old: old, New: matched, From: old.Num, To: matched.Num})
		}
		if old.Teacher != matched.Teacher {
			result.Changes = append(result.Changes, Change{Kind: TeacherChanged, Old: old, New: matched, From: old.Num, To: matched.Num})
		}
		if lessonTime(old) != lessonTime(matched) {
			result.Changes = append(result.Changes, Change{Kind: TimeChanged, Old: old, New: matched, From: old.Num, To: matched.Num})
		}
	}

	for i, lesson := range newDay.Lessons {
		if !consumed[i] {
			unmatchedNew = append(unmatchedNew, lesson)
		}
	}

	for _, old := range unmatchedOld {
		moved := false
		for i, candidate := range unmatchedNew {
			if sameLesson(old, candidate) {
				result.Changes = append(result.Changes, Change{Kind: Moved, Old: old, New: candidate, From: old.Num, To: candidate.Num})
				if old.Room != candidate.Room {
					result.Changes = append(result.Changes, Change{Kind: RoomChanged, Old: old, New: candidate, From: old.Num, To: candidate.Num})
				}
				if old.Teacher != candidate.Teacher {
					result.Changes = append(result.Changes, Change{Kind: TeacherChanged, Old: old, New: candidate, From: old.Num, To: candidate.Num})
				}
				if lessonTime(old) != lessonTime(candidate) {
					result.Changes = append(result.Changes, Change{Kind: TimeChanged, Old: old, New: candidate, From: old.Num, To: candidate.Num})
				}
				unmatchedNew = append(unmatchedNew[:i], unmatchedNew[i+1:]...)
				moved = true
				break
			}
		}
		if !moved {
			result.Changes = append(result.Changes, Change{Kind: Removed, Old: old, From: old.Num, To: old.Num})
		}
	}

	for _, candidate := range unmatchedNew {
		result.Changes = append(result.Changes, Change{Kind: Added, New: candidate, From: candidate.Num, To: candidate.Num})
	}

	return result
}
