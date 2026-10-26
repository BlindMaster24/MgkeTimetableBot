package archive

import (
	"context"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func RetentionCutoff(now time.Time, days int) int64 {
	if days <= 0 {
		return 0
	}
	return int64(utils.DayIndexFromDate(now.AddDate(0, 0, -days)))
}

func (r *Repository) PruneDaysBefore(ctx context.Context, dayIndex int64) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM timetable_archive WHERE day < ?`, dayIndex)
	if err != nil {
		return 0, err
	}
	pruned, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	_, _ = r.db.ExecContext(ctx, "PRAGMA optimize")
	return pruned, nil
}

func (r *Repository) Compact(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `VACUUM`)
	return err
}
