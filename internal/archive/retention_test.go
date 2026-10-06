package archive

import (
	"path/filepath"
	"testing"
	"time"
)

func appendRetentionDay(t *testing.T, repo *Repository, day, group string) {
	t.Helper()

	err := repo.AppendDays([]AppendDay{{
		Type:  "group",
		Value: group,
		Day:   map[string]any{"day": day, "lessons": []any{map[string]any{"lesson": "Математика"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRetentionCutoffCountsFullDaysBack(t *testing.T) {
	now := time.Date(2026, 10, 6, 4, 30, 0, 0, time.UTC)

	if got := RetentionCutoff(now, 0); got != 0 {
		t.Fatalf("cutoff without retention = %d, want 0", got)
	}

	month := RetentionCutoff(now, 30)
	week := RetentionCutoff(now, 7)
	if month >= week {
		t.Fatalf("a longer retention must reach further back: week=%d month=%d", week, month)
	}
	if want := int64(DateToDayIndex("06.09.2026")); month != want {
		t.Fatalf("month cutoff = %d, want %d", month, want)
	}
	if want := int64(DateToDayIndex("29.09.2026")); week != want {
		t.Fatalf("week cutoff = %d, want %d", week, want)
	}
}

func TestPruneDaysBeforeKeepsEverythingItDoesNotOwn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")

	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	appendRetentionDay(t, repo, "01.02.2026", "100")
	appendRetentionDay(t, repo, "02.02.2026", "100")
	appendRetentionDay(t, repo, "10.09.2026", "100")

	if _, err := repo.DB().Exec(`INSERT INTO timetable_archive (day, teacher, data) VALUES (?, ?, ?)`, DateToDayIndex("01.02.2026"), "Иванов И.И.", "[]"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`CREATE TABLE decoy_notes (id INTEGER PRIMARY KEY, note TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`INSERT INTO decoy_notes (note) VALUES ('keep me')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`CREATE TABLE timetable_archive_old (day INTEGER, data TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`INSERT INTO timetable_archive_old (day, data) VALUES (?, '[]')`, DateToDayIndex("01.02.2026")); err != nil {
		t.Fatal(err)
	}

	cutoff := RetentionCutoff(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), 30)
	removed, err := repo.PruneDaysBefore(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Fatalf("removed %d rows, want 3 (two group days and one teacher day before the cutoff)", removed)
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 1 || stats.WithGroup != 1 {
		t.Fatalf("archive after pruning = %+v, want the day inside the window only", stats)
	}

	days, err := repo.GroupDays("100", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].Day != "10.09.2026" {
		t.Fatalf("kept days = %+v, want the fresh day", days)
	}

	var notes, planted int
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM decoy_notes`).Scan(&notes); err != nil {
		t.Fatalf("the decoy table did not survive: %v", err)
	}
	if notes != 1 {
		t.Fatalf("decoy rows = %d, want 1", notes)
	}
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM timetable_archive_old`).Scan(&planted); err != nil {
		t.Fatalf("the decoy table with a similar name did not survive: %v", err)
	}
	if planted != 1 {
		t.Fatalf("decoy rows = %d, want 1", planted)
	}

	if err := repo.Compact(); err != nil {
		t.Fatalf("compact after a prune: %v", err)
	}
	if stats, err = repo.Stats(); err != nil || stats.Total != 1 {
		t.Fatalf("compaction changed the archive: %+v %v", stats, err)
	}
}
