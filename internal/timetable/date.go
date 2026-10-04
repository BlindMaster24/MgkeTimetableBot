package timetable

import (
	"fmt"
	"strings"
	"time"
)

const DateFormat = "02.01.2006"

func ParseDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := time.Parse(DateFormat, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("timetable date %q must look like dd.mm.yyyy", value)
	}
	return parsed, nil
}

func FormatDate(day time.Time) string {
	return day.Format(DateFormat)
}
