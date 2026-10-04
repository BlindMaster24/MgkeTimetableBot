package reminder

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func callsFixture() Schedule {
	return Schedule{
		Weekdays: [][2][2]string{
			{{"09:00", "09:45"}, {"09:55", "10:40"}},
			{{"10:50", "11:35"}, {"11:45", "12:30"}},
			{{"12:40", "13:25"}, {"13:35", "14:20"}},
		},
		Saturday: [][2][2]string{
			{{" 09:00 ", "09:30"}, {"09:35", "10:05"}},
		},
	}
}

func lesson(num int, subject string) timetable.Lesson {
	return timetable.Lesson{Num: num, Subject: subject}
}

func dayFixture() timetable.Day {
	return timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		lesson(1, "Math"),
		lesson(2, "Physics"),
		lesson(3, "History"),
	}}
}

func at(hour, minute int) time.Time {
	return time.Date(2026, 9, 7, hour, minute, 0, 0, time.UTC)
}

func TestNextLessonEmptyDayStaysSilent(t *testing.T) {
	calls := callsFixture()
	for _, day := range []timetable.Day{
		{Date: "07.09.2026"},
		{Date: "07.09.2026", Lessons: []timetable.Lesson{}},
		{Date: "07.09.2026", Lessons: []timetable.Lesson{{Num: 1}, {Num: 2, Subject: "  "}}},
	} {
		if _, ok := NextLesson(day, at(8, 0), calls); ok {
			t.Fatalf("day %+v matched, want none", day.Lessons)
		}
	}
}

func TestNextLessonBeforeFirstReturnsOpener(t *testing.T) {
	next, ok := NextLesson(dayFixture(), at(8, 0), callsFixture())
	if !ok {
		t.Fatal("want first lesson, got none")
	}
	if next.Lesson.Subject != "Math" || next.Index != 0 {
		t.Fatalf("next = %+v, want Math at 0", next)
	}
	if next.Start != at(9, 0) || next.End != at(10, 40) {
		t.Fatalf("span = %v-%v, want 09:00-10:40", next.Start, next.End)
	}
}

func TestNextLessonDuringLessonReturnsCurrent(t *testing.T) {
	next, ok := NextLesson(dayFixture(), at(11, 0), callsFixture())
	if !ok || next.Lesson.Subject != "Physics" || next.Index != 1 {
		t.Fatalf("next = %+v, want Physics at 1", next)
	}
}

func TestNextLessonTreatsEndAsPassed(t *testing.T) {
	next, ok := NextLesson(dayFixture(), at(10, 40), callsFixture())
	if !ok || next.Lesson.Subject != "Physics" {
		t.Fatalf("next = %+v, want Physics after Math ended", next)
	}
	next, ok = NextLesson(dayFixture(), at(9, 0), callsFixture())
	if !ok || next.Lesson.Subject != "Math" {
		t.Fatalf("next = %+v, want Math at its start", next)
	}
}

func TestNextLessonAllPassedStaysSilent(t *testing.T) {
	for _, now := range []time.Time{at(14, 20), at(20, 0)} {
		if _, ok := NextLesson(dayFixture(), now, callsFixture()); ok {
			t.Fatalf("now %v matched, want none", now)
		}
	}
}

func TestNextLessonPicksEarliestStartFromUnsortedDay(t *testing.T) {
	day := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		lesson(3, "History"),
		lesson(1, "Math"),
		lesson(2, "Physics"),
	}}
	next, ok := NextLesson(day, at(8, 0), callsFixture())
	if !ok || next.Lesson.Subject != "Math" || next.Index != 1 {
		t.Fatalf("next = %+v, want Math at 1", next)
	}
}

func TestNextLessonPairsSubgroupSlotsInOrder(t *testing.T) {
	day := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		{Num: 1, Subject: "Math", Subgroup: 1},
		{Num: 1, Subject: "Math", Subgroup: 2},
	}}
	next, ok := NextLesson(day, at(9, 30), callsFixture())
	if !ok || next.Index != 0 || next.Lesson.Subgroup != 1 {
		t.Fatalf("next = %+v, want first subgroup at 0", next)
	}
}

func TestNextLessonSkipsUnresolvableLessons(t *testing.T) {
	day := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		{Num: 0, Subject: "Zero"},
		{Num: 9, Subject: "Far"},
		lesson(2, "Physics"),
	}}
	next, ok := NextLesson(day, at(8, 0), callsFixture())
	if !ok || next.Lesson.Subject != "Physics" || next.Index != 2 {
		t.Fatalf("next = %+v, want Physics at 2", next)
	}
	onlyBroken := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{
		{Num: 0, Subject: "Zero"},
		{Num: 9, Subject: "Far"},
	}}
	if _, ok := NextLesson(onlyBroken, at(8, 0), callsFixture()); ok {
		t.Fatal("want none for unresolvable lessons")
	}
}

func TestNextLessonSkipsBrokenSlots(t *testing.T) {
	calls := Schedule{
		Weekdays: [][2][2]string{
			{{"nope", "09:45"}, {"09:55", "10:40"}},
			{{"10:50", "11:35"}, {"11:45", "backwards"}},
			{{"12:40", "13:25"}, {"13:35", "12:00"}},
		},
	}
	if _, ok := NextLesson(dayFixture(), at(8, 0), calls); ok {
		t.Fatal("want none for broken slots")
	}
}

func TestNextLessonRejectsBadDateAndEmptyCalls(t *testing.T) {
	if _, ok := NextLesson(timetable.Day{Date: "tomorrow", Lessons: []timetable.Lesson{lesson(1, "Math")}}, at(8, 0), callsFixture()); ok {
		t.Error("bad date matched, want none")
	}
	if _, ok := NextLesson(dayFixture(), at(8, 0), Schedule{}); ok {
		t.Error("empty calls matched, want none")
	}
}

func TestNextLessonUsesSaturdayTable(t *testing.T) {
	day := timetable.Day{Date: "05.09.2026", Lessons: []timetable.Lesson{lesson(1, "Math")}}
	now := time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)
	next, ok := NextLesson(day, now, callsFixture())
	if !ok {
		t.Fatal("want Saturday lesson, got none")
	}
	wantEnd := time.Date(2026, 9, 5, 10, 5, 0, 0, time.UTC)
	if next.End != wantEnd {
		t.Fatalf("end = %v, want %v", next.End, wantEnd)
	}
}

func TestNextLessonUsesWeekdayTableOnSunday(t *testing.T) {
	day := timetable.Day{Date: "06.09.2026", Lessons: []timetable.Lesson{lesson(1, "Math")}}
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	next, ok := NextLesson(day, now, callsFixture())
	if !ok {
		t.Fatal("want Sunday lesson, got none")
	}
	wantEnd := time.Date(2026, 9, 6, 10, 40, 0, 0, time.UTC)
	if next.End != wantEnd {
		t.Fatalf("end = %v, want %v", next.End, wantEnd)
	}
}

func TestNextLessonKeepsNowLocation(t *testing.T) {
	loc := time.FixedZone("Plus3", 3*60*60)
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, loc)
	next, ok := NextLesson(dayFixture(), now, callsFixture())
	if !ok {
		t.Fatal("want lesson, got none")
	}
	wantStart := time.Date(2026, 9, 7, 9, 0, 0, 0, loc)
	if !next.Start.Equal(wantStart) || next.Start.Location() != loc {
		t.Fatalf("start = %v, want %v in Plus3", next.Start, wantStart)
	}
}

func TestNextLessonFutureDayReturnsOpener(t *testing.T) {
	next, ok := NextLesson(dayFixture(), at(20, 0).Add(-24*time.Hour), callsFixture())
	if !ok || next.Lesson.Subject != "Math" {
		t.Fatalf("next = %+v, want Math for a future day", next)
	}
}

func TestLessonSpanResolvesSlot(t *testing.T) {
	start, end, ok := LessonSpan(dayFixture(), 1, time.UTC, callsFixture())
	if !ok || start != at(10, 50) || end != at(12, 30) {
		t.Fatalf("span = %v-%v %v, want 10:50-12:30 true", start, end, ok)
	}
}

func TestLessonSpanRejectsBadInput(t *testing.T) {
	calls := callsFixture()
	if _, _, ok := LessonSpan(dayFixture(), -1, time.UTC, calls); ok {
		t.Error("negative index matched, want none")
	}
	if _, _, ok := LessonSpan(dayFixture(), 9, time.UTC, calls); ok {
		t.Error("large index matched, want none")
	}
	badDate := timetable.Day{Date: "tomorrow", Lessons: []timetable.Lesson{lesson(1, "Math")}}
	if _, _, ok := LessonSpan(badDate, 0, time.UTC, calls); ok {
		t.Error("bad date matched, want none")
	}
	broken := timetable.Day{Date: "07.09.2026", Lessons: []timetable.Lesson{{Num: 9, Subject: "Far"}}}
	if _, _, ok := LessonSpan(broken, 0, time.UTC, calls); ok {
		t.Error("out of range slot matched, want none")
	}
}
