package utils

import (
	"time"
)

var startingWeekIndexDate = time.Date(1970, 1, 5, 0, 0, 0, 0, time.UTC)

func dayNumber(date time.Time) int64 {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).UnixMilli() / oneDayMs
}

const (
	oneDayMs  = 24 * 60 * 60 * 1000
	oneWeekMs = 7 * oneDayMs
)

type WeekIndex struct {
	value int
}

func WeekIndexFromDate(date time.Time) WeekIndex {
	year, month, dayOfMonth := date.Date()
	day := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
	if day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, -1)
	}
	days := dayNumber(day) - dayNumber(startingWeekIndexDate)
	return WeekIndex{value: int(days / 7)}
}

func WeekIndexFromNumber(n int) WeekIndex {
	return WeekIndex{value: n}
}

func WeekIndexFromAcademicNumber(weekNumber int, date time.Time) WeekIndex {
	start := academicYearStartDate(date)
	return WeekIndexFromDate(start.AddDate(0, 0, 7*(weekNumber-1)))
}

func (w WeekIndex) Value() int {
	return w.value
}

func (w WeekIndex) FirstDayDate() time.Time {
	return DayIndexToDate(w.value * 7)
}

func (w WeekIndex) WeekRange() (time.Time, time.Time) {
	d1 := w.FirstDayDate()
	d2 := d1.AddDate(0, 0, 6)
	return d1, d2
}

func (w WeekIndex) AcademicWeekNumber() int {
	d := w.FirstDayDate()
	weeks := dayNumber(d) - dayNumber(academicYearStartDate(d))
	return int(weeks/7) + 1
}

func (w WeekIndex) WeekDayIndexRange() (int, int) {
	d1, d2 := w.WeekRange()
	return DayIndexFromDate(d1), DayIndexFromDate(d2)
}

func (w WeekIndex) Next() WeekIndex {
	return WeekIndexFromNumber(w.value + 1)
}

func (w WeekIndex) IsFutureWeek() bool {
	return w.value > WeekIndexFromDate(time.Now()).Value()
}

func (w WeekIndex) String() string {
	d1, d2 := w.WeekRange()
	return d1.Format("02.01") + "-" + d2.Format("02.01")
}

func academicYearStartDate(date time.Time) time.Time {
	year := date.Year()
	if date.Month() < 9 {
		year--
	}
	start := time.Date(year, 9, 1, 0, 0, 0, 0, time.UTC)
	day := start.Weekday()
	if day == time.Monday {
		return start
	}
	if day == time.Sunday {
		return start.AddDate(0, 0, 1)
	}
	return start.AddDate(0, 0, 8-int(day))
}

func DayIndexFromDate(date time.Time) int {
	return int(dayNumber(date) - dayNumber(startingWeekIndexDate))
}

func DayIndexToDate(index int) time.Time {
	return time.UnixMilli((dayNumber(startingWeekIndexDate) + int64(index)) * oneDayMs).UTC()
}
