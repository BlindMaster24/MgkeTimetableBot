package archive

import (
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func RetentionCutoff(now time.Time, days int) int64 {
	if days <= 0 {
		return 0
	}
	return int64(utils.DayIndexFromDate(now.AddDate(0, 0, -days)))
}

func (r *Repository) PruneDaysBefore(dayIndex int64) (int64, error) {
	result, err := r.db.Exec(`DELETE FROM timetable_archive WHERE day < ?`, dayIndex)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Repository) Compact() error {
	_, err := r.db.Exec(`VACUUM`)
	return err
}
