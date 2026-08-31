package utils

import (
	"time"
)

var startingWeekIndexDate = time.Date(1970, 1, 5, 0, 0, 0, 0, time.UTC)

const (
	oneDayMs  = 24 * 60 * 60 * 1000
	oneWeekMs = 7 * oneDayMs
)

type WeekIndex struct {
	value int
}

func WeekIndexFromDate(date time.Time) WeekIndex {
	dayOfWeek := date.Weekday()
	if dayOfWeek == time.Sunday {
		date = date.AddDate(0, 0, -1)
	}
	ms := date.UnixMilli() - startingWeekIndexDate.UnixMilli()
	return WeekIndex{value: int(ms / oneWeekMs)}
}

func WeekIndexFromNumber(n int) WeekIndex {
	return WeekIndex{value: n}
}

func (w WeekIndex) Value() int {
	return w.value
}

func (w WeekIndex) FirstDayDate() time.Time {
	ms := int64(w.value)*oneWeekMs + startingWeekIndexDate.UnixMilli()
	return time.UnixMilli(ms)
}

func (w WeekIndex) WeekRange() (time.Time, time.Time) {
	d1 := w.FirstDayDate()
	d2 := d1.AddDate(0, 0, 6)
	return d1, d2
}

func (w WeekIndex) AcademicWeekNumber() int {
	d := w.FirstDayDate()
	start := academicYearStartDate(d)
	diff := d.Sub(start)
	return int(diff/(7*24*time.Hour)) + 1
}

func (w WeekIndex) WeekDayIndexRange() (int, int) {
	d1, d2 := w.WeekRange()
	return dayIndexFromDate(d1), dayIndexFromDate(d2)
}

func (w WeekIndex) GetRelevant(maxLastGroups, maxLastTeachers int) WeekIndex {
	date := time.Now()
	weekIndex := WeekIndexFromDate(date)
	if date.Weekday() == time.Sunday {
		weekIndex = WeekIndexFromNumber(weekIndex.value + 1)
	}
	relevant := weekIndex.value
	if maxLastGroups > 0 && maxLastGroups < relevant {
		relevant = maxLastGroups
	}
	if maxLastTeachers > 0 && maxLastTeachers < relevant {
		relevant = maxLastTeachers
	}
	return WeekIndexFromNumber(relevant)
}

func (w WeekIndex) Next() WeekIndex {
	return WeekIndexFromNumber(w.value + 1)
}

func (w WeekIndex) Prev() WeekIndex {
	return WeekIndexFromNumber(w.value - 1)
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

func dayIndexFromDate(date time.Time) int {
	ms := date.UnixMilli() - startingWeekIndexDate.UnixMilli()
	return int(ms / oneDayMs)
}
