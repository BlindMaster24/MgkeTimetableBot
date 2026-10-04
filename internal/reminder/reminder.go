package reminder

import (
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type Schedule struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

type LessonAt struct {
	Lesson timetable.Lesson
	Start  time.Time
	End    time.Time
	Index  int
}

func NextLesson(day timetable.Day, now time.Time, calls Schedule) (LessonAt, bool) {
	date, err := timetable.ParseDate(day.Date)
	if err != nil {
		return LessonAt{}, false
	}
	table := tableFor(calls, date)
	if len(table) == 0 {
		return LessonAt{}, false
	}
	loc := now.Location()
	best := LessonAt{}
	found := false
	for i, lesson := range day.Lessons {
		if strings.TrimSpace(lesson.Subject) == "" {
			continue
		}
		start, end, ok := spanAt(date, loc, table, lesson.Num)
		if !ok || !end.After(now) {
			continue
		}
		if !found || start.Before(best.Start) {
			best = LessonAt{Lesson: lesson, Start: start, End: end, Index: i}
			found = true
		}
	}
	return best, found
}

func LessonSpan(day timetable.Day, index int, loc *time.Location, calls Schedule) (time.Time, time.Time, bool) {
	if index < 0 || index >= len(day.Lessons) {
		return time.Time{}, time.Time{}, false
	}
	date, err := timetable.ParseDate(day.Date)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return spanAt(date, loc, tableFor(calls, date), day.Lessons[index].Num)
}

func tableFor(calls Schedule, date time.Time) [][2][2]string {
	if date.Weekday() == time.Saturday {
		return calls.Saturday
	}
	return calls.Weekdays
}

func spanAt(date time.Time, loc *time.Location, table [][2][2]string, num int) (time.Time, time.Time, bool) {
	if num < 1 || num-1 >= len(table) {
		return time.Time{}, time.Time{}, false
	}
	slot := table[num-1]
	start, ok := clockAt(date, loc, slot[0][0])
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	end, ok := clockAt(date, loc, slot[1][1])
	if !ok || !end.After(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func clockAt(date time.Time, loc *time.Location, value string) (time.Time, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(date.Year(), date.Month(), date.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc), true
}
