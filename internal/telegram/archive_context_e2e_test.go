package telegram

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestE2E_ArchiveReadsStopWhenTheContextIsCancelled(t *testing.T) {
	b, _ := setupE2EBot(t)

	archiveRepo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archiveRepo.Close() })

	if err := archiveRepo.AppendDays(context.Background(), []archive.AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
	}); err != nil {
		t.Fatal(err)
	}
	b.archive = archiveRepo

	days, result := b.archiveDaysForRange(context.Background(), "group", "100", 0, 1<<30)
	if result != archiveAnswered || len(days) != 1 {
		t.Fatalf("live archive read = %v %v, want one answered day", days, result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	days, result = b.archiveDaysForRange(ctx, "group", "100", 0, 1<<30)
	if result != archiveFailed || days != nil {
		t.Fatalf("archiveDaysForRange with a cancelled context = %v %v, want archiveFailed and no days", days, result)
	}

	week := utils.WeekIndexFromNumber(0)
	if got := b.weekDays(ctx, daysFromArchive, "group", "100", week); len(got) != 0 {
		t.Fatalf("weekDays with a cancelled context read %d days, want none", len(got))
	}
}
