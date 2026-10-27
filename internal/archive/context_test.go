package archive

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadsAbortWithACancelledContext(t *testing.T) {
	repo := newTestRepo(t)
	appendRetentionDay(t, repo, "07.09.2026", "100")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dayIndex := int64(DateToDayIndex("07.09.2026"))
	checks := []struct {
		name string
		call func() error
	}{
		{"GroupDays", func() error { _, err := repo.GroupDays(ctx, "100", nil); return err }},
		{"GroupDay", func() error { _, err := repo.GroupDay(ctx, dayIndex, "100"); return err }},
		{"GroupDaysByRange", func() error { _, err := repo.GroupDaysByRange(ctx, dayIndex, dayIndex, "100"); return err }},
		{"TeacherDays", func() error { _, err := repo.TeacherDays(ctx, "Иванов И.И.", nil); return err }},
		{"TeacherDay", func() error { _, err := repo.TeacherDay(ctx, dayIndex, "Иванов И.И."); return err }},
		{"TeacherDaysByRange", func() error {
			_, err := repo.TeacherDaysByRange(ctx, dayIndex, dayIndex, "Иванов И.И.")
			return err
		}},
		{"DayIndexBounds", func() error { _, err := repo.DayIndexBounds(ctx); return err }},
		{"WeekIndexBounds", func() error { _, err := repo.WeekIndexBounds(ctx); return err }},
		{"Stats", func() error { _, err := repo.Stats(ctx); return err }},
		{"Groups", func() error { _, err := repo.Groups(ctx); return err }},
		{"Teachers", func() error { _, err := repo.Teachers(ctx); return err }},
	}

	for _, check := range checks {
		if err := check.call(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a cancelled context = %v, want context.Canceled", check.name, err)
		}
	}
}

func TestReadsAbortWhenTheDeadlinePassed(t *testing.T) {
	repo := newTestRepo(t)
	appendRetentionDay(t, repo, "07.09.2026", "100")

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	if _, err := repo.TeacherDays(ctx, "Иванов И.И.", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TeacherDays with an expired deadline = %v, want context.DeadlineExceeded", err)
	}
}
