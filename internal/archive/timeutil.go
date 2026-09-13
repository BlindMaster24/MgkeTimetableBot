package archive

import (
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func DateToDayIndex(dateStr string) int64 {
	parsed, err := time.Parse("02.01.2006", strings.TrimSpace(dateStr))
	if err != nil {
		return 0
	}
	return int64(utils.DayIndexFromDate(parsed))
}

func DayIndexToDate(dayIndex int64) string {
	return utils.DayIndexToDate(int(dayIndex)).Format("02.01.2006")
}
