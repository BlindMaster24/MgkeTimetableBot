package cache

import (
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func dayLessonsChanged(oldDay, newDay map[string]any) bool {
	if result, ok := diffDayLessons(oldDay, newDay); ok {
		return !result.Empty() || lessonsJSON(oldDay["lessons"]) != lessonsJSON(newDay["lessons"])
	}
	return lessonsJSON(oldDay["lessons"]) != lessonsJSON(newDay["lessons"])
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
	lesson := timetable.Lesson{
		Num:     num,
		Subject: strField(m, "lesson"),
		Type:    strField(m, "type"),
		Teacher: strField(m, "teacher"),
		Group:   strField(m, "group"),
		Room:    strField(m, "cabinet"),
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

func strField(m map[string]any, key string) string {
	text, _ := m[key].(string)
	return text
}
