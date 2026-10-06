package reminder

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

var fuzzCalls = Schedule{
	Weekdays: [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
		{{"10:50", "11:35"}, {"11:45", "12:30"}},
		{{"12:40", "13:25"}, {"13:35", "14:20"}},
		{{"14:50", "15:35"}, {"15:45", "16:30"}},
		{{"16:40", "17:25"}, {"17:35", "18:20"}},
		{{"18:30", "19:15"}, {"19:25", "20:10"}},
	},
	Saturday: [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
		{{"10:50", "11:35"}, {"11:45", "12:30"}},
		{{"12:50", "13:35"}, {"13:45", "14:30"}},
		{{"14:40", "15:25"}, {"15:35", "16:20"}},
		{{"16:30", "17:15"}, {"17:25", "18:10"}},
		{{"18:20", "19:05"}, {"19:15", "20:00"}},
	},
}

func fuzzDay(date, spec string) timetable.Day {
	day := timetable.Day{Date: date}
	for _, line := range strings.Split(spec, "\n") {
		fields := strings.SplitN(line, ";", 2)
		if len(fields) != 2 {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil || num < 1 || num > 8 {
			continue
		}
		day.Lessons = append(day.Lessons, timetable.Lesson{Num: num, Subject: fields[1]})
	}
	return day
}

func fuzzCheckLesson(t *testing.T, day timetable.Day, found LessonAt, loc *time.Location) {
	t.Helper()

	if strings.TrimSpace(found.Lesson.Subject) == "" {
		t.Fatalf("a blank subject was picked: %+v", found)
	}
	if found.Index < 0 || found.Index >= len(day.Lessons) {
		t.Fatalf("index %d is outside the day of %d lessons", found.Index, len(day.Lessons))
	}
	if !reflect.DeepEqual(found.Lesson, day.Lessons[found.Index]) {
		t.Fatalf("lesson %+v is not the one at index %d", found.Lesson, found.Index)
	}
	if found.Start.Location() != loc || found.End.Location() != loc {
		t.Fatalf("span %v-%v does not live in %v", found.Start, found.End, loc)
	}
	if !found.Start.Before(found.End) {
		t.Fatalf("span %v-%v does not move forward", found.Start, found.End)
	}
}

func fuzzCheckEarliest(t *testing.T, day timetable.Day, now time.Time, keep func(start, end time.Time) bool, got time.Time) {
	t.Helper()

	for i, lesson := range day.Lessons {
		if strings.TrimSpace(lesson.Subject) == "" {
			continue
		}
		start, end, ok := LessonSpan(day, i, now.Location(), fuzzCalls)
		if !ok || !keep(start, end) {
			continue
		}
		if start.Before(got) {
			t.Fatalf("lesson %d starts at %v, before the returned %v", i, start, got)
		}
	}
}

func FuzzCurrentAndUpcomingStayInsideTheDay(f *testing.F) {
	f.Add(2026, 9, 7, 11, 0, "1;Math\n2;Physics")
	f.Add(2026, 9, 7, 10, 45, "1;Math\n2;Physics")
	f.Add(2026, 9, 7, 8, 0, "1;Math")
	f.Add(2026, 9, 7, 21, 30, "1;Math")
	f.Add(2026, 9, 5, 9, 30, "1;Saturday")
	f.Add(2026, 9, 7, 11, 0, "")
	f.Add(2026, 9, 7, 11, 0, "1;  \n3;History")
	f.Add(2026, 9, 7, 13, 0, "3;History\n1;Math\n2;Physics")

	f.Fuzz(func(t *testing.T, year, month, dayOfMonth, hour, minute int, spec string) {
		if year < 2000 || year > 2100 || month < 1 || month > 12 || dayOfMonth < 1 || dayOfMonth > 28 ||
			hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return
		}

		now := time.Date(year, time.Month(month), dayOfMonth, hour, minute, 0, 0, time.UTC)
		day := fuzzDay(now.Format("02.01.2006"), spec)
		loc := now.Location()

		current, hasCurrent := Current(day, now, fuzzCalls)
		if hasCurrent {
			fuzzCheckLesson(t, day, current, loc)
			if current.Start.After(now) || !current.End.After(now) {
				t.Fatalf("current %v-%v does not contain %v", current.Start, current.End, now)
			}
			fuzzCheckEarliest(t, day, now, func(start, end time.Time) bool {
				return !start.After(now) && end.After(now)
			}, current.Start)

			again, againOK := Current(day, now, fuzzCalls)
			if !againOK || !reflect.DeepEqual(again, current) {
				t.Fatalf("current was not stable: %+v then %+v", current, again)
			}
		}

		upcoming, hasUpcoming := Upcoming(day, now, fuzzCalls)
		if hasUpcoming {
			fuzzCheckLesson(t, day, upcoming, loc)
			if !upcoming.Start.After(now) {
				t.Fatalf("upcoming starts at %v, not after %v", upcoming.Start, now)
			}
			fuzzCheckEarliest(t, day, now, func(start, end time.Time) bool {
				return start.After(now)
			}, upcoming.Start)
		}

		next, hasNext := NextLesson(day, now, fuzzCalls)
		if hasNext {
			fuzzCheckLesson(t, day, next, loc)
			if !next.End.After(now) {
				t.Fatalf("next ends at %v, not after %v", next.End, now)
			}
			fuzzCheckEarliest(t, day, now, func(start, end time.Time) bool {
				return end.After(now)
			}, next.Start)
		}

		if hasCurrent && hasUpcoming && !upcoming.Start.After(current.End) {
			t.Fatalf("current %v-%v overlaps upcoming from %v", current.Start, current.End, upcoming.Start)
		}

		for num := 1; num <= 7; num++ {
			start, end, ok := Slot(now, loc, fuzzCalls, num)
			if !ok {
				continue
			}
			if !start.Before(end) {
				t.Fatalf("slot %d is %v-%v, which does not move forward", num, start, end)
			}
			if start.Location() != loc || end.Location() != loc {
				t.Fatalf("slot %d is not in %v", num, loc)
			}
			if start.Year() != now.Year() || start.Month() != now.Month() || start.Day() != now.Day() {
				t.Fatalf("slot %d starts on %v, not on %v", num, start, now)
			}
			if end.Year() != now.Year() || end.Month() != now.Month() || end.Day() != now.Day() {
				t.Fatalf("slot %d ends on %v, not on %v", num, end, now)
			}
		}
	})
}
