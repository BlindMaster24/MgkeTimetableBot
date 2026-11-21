package cache

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

// dayLessonsChanged reports whether the lessons of a day really changed.
//
// Both sides are compared in the typed form, because the same day reaches this
// function in two shapes: the parser produces "lesson"/"cabinet" keys and
// nested subgroup arrays, while the cache file stores the typed view
// ("num"/"subject"/"room" rows). Comparing the raw JSON made every day look
// changed after a restart and sent "расписание изменилось" to every chat.
func dayLessonsChanged(oldDay, newDay map[string]any) bool {
	oldTyped, oldErr := timetableDayFromMap(oldDay)
	newTyped, newErr := timetableDayFromMap(newDay)
	if oldErr == nil && newErr == nil {
		return !schedulediff.Diff(oldTyped, newTyped).Empty() || !sameTypedDay(oldTyped, newTyped)
	}
	return lessonsJSON(oldDay["lessons"]) != lessonsJSON(newDay["lessons"])
}

// sameTypedDay compares the canonical encoding, so comments and anything the
// diff does not inspect still count as a change.
func sameTypedDay(a, b timetable.Day) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(left, right)
}

func diffDayLessons(oldDay, newDay map[string]any) (schedulediff.Result, bool) {
	oldTyped, err := timetableDayFromMap(oldDay)
	if err != nil {
		return schedulediff.Result{}, false
	}
	newTyped, err := timetableDayFromMap(newDay)
	if err != nil {
		return schedulediff.Result{}, false
	}
	return schedulediff.Diff(oldTyped, newTyped), true
}

func timetableDayFromMap(day map[string]any) (timetable.Day, error) {
	if day == nil {
		return timetable.Day{}, fmt.Errorf("day of unknown shape")
	}
	date, _ := day["day"].(string)
	out := timetable.Day{Date: date}
	if _, err := timetable.ParseDate(date); err != nil {
		return timetable.Day{}, err
	}
	raw, _ := day["lessons"].([]any)
	if day["lessons"] != nil && raw == nil {
		return timetable.Day{}, fmt.Errorf("day %q holds lessons of unknown shape", date)
	}
	num := 0
	for _, item := range raw {
		if item == nil {
			continue
		}
		if arr, ok := item.([]any); ok {
			if len(arr) == 0 {
				continue
			}
			num++
			for _, sub := range arr {
				lesson, err := timetableLessonFromMap(sub, num)
				if err != nil {
					return timetable.Day{}, err
				}
				out.Lessons = append(out.Lessons, lesson)
			}
			continue
		}
		single, ok := item.(map[string]any)
		if !ok {
			return timetable.Day{}, fmt.Errorf("day %q holds a lesson of unknown shape", date)
		}
		num++
		lesson, err := timetableLessonFromMap(single, num)
		if err != nil {
			return timetable.Day{}, err
		}
		out.Lessons = append(out.Lessons, lesson)
	}
	return out, nil
}

func timetableLessonFromMap(item any, num int) (timetable.Lesson, error) {
	m, ok := item.(map[string]any)
	if !ok || m == nil {
		return timetable.Lesson{}, fmt.Errorf("lesson of unknown shape")
	}
	// A day read back from the cache file carries the typed key names
	// (num/subject/room); the parser's own shape uses lesson/cabinet. Both have
	// to fold into the same lesson, otherwise the two shapes never compare
	// equal and every restart looks like a timetable change.
	lesson := timetable.Lesson{
		Num:     numField(m, num),
		Subject: firstField(m, "lesson", "subject"),
		Type:    strField(m, "type"),
		Teacher: strField(m, "teacher"),
		Group:   strField(m, "group"),
		Room:    firstField(m, "cabinet", "room"),
		Comment: strField(m, "comment"),
	}
	switch sub := m["subgroup"].(type) {
	case nil:
	case float64:
		lesson.Subgroup = int(sub)
	case int:
		lesson.Subgroup = sub
	default:
		return timetable.Lesson{}, fmt.Errorf("lesson holds a subgroup of unknown shape")
	}
	if text, ok := m["time"].(string); ok && text != "" {
		lesson.Extra = map[string]any{"time": text}
	}
	return lesson, nil
}

func firstField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := m[key].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func numField(m map[string]any, fallback int) int {
	switch value := m["num"].(type) {
	case float64:
		if value > 0 {
			return int(value)
		}
	case int:
		if value > 0 {
			return value
		}
	}
	return fallback
}

func strField(m map[string]any, key string) string {
	text, _ := m[key].(string)
	return text
}
