package archive

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func DateToDayIndex(dateStr string) int64 {
	parts := strings.Split(dateStr, ".")
	if len(parts) != 3 {
		return 0
	}

	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])

	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int64(t.Sub(epoch).Hours() / 24)
}

func DayIndexToDate(dayIndex int64) string {
	t := epoch.Add(time.Duration(dayIndex) * 24 * time.Hour)
	return fmt.Sprintf("%02d.%02d.%04d", t.Day(), t.Month(), t.Year())
}
