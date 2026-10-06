package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func maintenanceConfig(t *testing.T) (*config.Config, *archive.Repository) {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(dir, "sqlite3.db")}
	cfg.Archive.BackupDir = filepath.Join(dir, "backups")
	cfg.Archive.BackupKeep = 2
	cfg.Archive.RetentionDays = 30

	repo, err := archive.New(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	for _, day := range []string{"01.02.2026", "02.02.2026", "06.10.2026"} {
		if err := repo.AppendDays([]archive.AppendDay{{
			Type:  "group",
			Value: "100",
			Day:   map[string]any{"day": day, "lessons": []any{map[string]any{"lesson": "Математика"}}},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.DB().Exec(`INSERT INTO timetable_archive (day, teacher, data) VALUES (?, ?, '[]')`, archive.DateToDayIndex("01.02.2026"), "Иванов И.И."); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`CREATE TABLE decoy_notes (id INTEGER PRIMARY KEY, note TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().Exec(`INSERT INTO decoy_notes (note) VALUES ('keep me')`); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(cfg.Archive.BackupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"notes.txt": "keep me", "sqlite3-20250101-000000.db.gz": "keep me"} {
		if err := os.WriteFile(filepath.Join(cfg.Archive.BackupDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return cfg, repo
}

func maintenanceAt(m *archiveMaintenance, now time.Time) {
	m.now = func() time.Time { return now }
}

func storedDays(t *testing.T, path string) []string {
	t.Helper()

	repo, err := archive.New(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer repo.Close()

	days, err := repo.GroupDays("100", nil)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(days))
	for _, day := range days {
		out = append(out, day.Day)
	}
	return out
}

func TestArchiveMaintenanceBacksUpBeforeItPrunes(t *testing.T) {
	cfg, repo := maintenanceConfig(t)
	m := newArchiveMaintenance(cfg, repo, logger.New("error", nil))
	maintenanceAt(m, time.Date(2026, 10, 6, 4, 30, 15, 0, time.UTC))

	report, err := m.Run()
	if err != nil {
		t.Fatalf("maintenance: %v", err)
	}

	if !report.Backup || !report.Retention {
		t.Fatalf("report = %+v, want both halves on", report)
	}
	if want := filepath.Join(cfg.Archive.BackupDir, "sqlite3-20261006-043015.db"); report.Path != want {
		t.Fatalf("path = %q, want %q", report.Path, want)
	}
	if report.PrunedRows != 3 {
		t.Errorf("pruned %d rows, want 3 (two old group days and one teacher day)", report.PrunedRows)
	}
	if report.Kept != 1 || report.Removed != 0 {
		t.Errorf("kept=%d removed=%d, want the fresh copy only", report.Kept, report.Removed)
	}

	live := storedDays(t, cfg.DBPath)
	if len(live) != 1 || live[0] != "06.10.2026" {
		t.Fatalf("live archive holds %v, want the day inside the retention window", live)
	}
	snapshot := storedDays(t, report.Path)
	if len(snapshot) != 3 {
		t.Fatalf("snapshot holds %v, want every day that existed before the prune", snapshot)
	}

	for _, name := range []string{"notes.txt", "sqlite3-20250101-000000.db.gz"} {
		if _, err := os.Stat(filepath.Join(cfg.Archive.BackupDir, name)); err != nil {
			t.Errorf("maintenance deleted %q, which it does not own: %v", name, err)
		}
	}
	var notes int
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM decoy_notes`).Scan(&notes); err != nil || notes != 1 {
		t.Errorf("the decoy table did not survive: %d %v", notes, err)
	}
}

func TestArchiveMaintenanceRetiresOnlyItsOwnOldestCopies(t *testing.T) {
	cfg, repo := maintenanceConfig(t)
	m := newArchiveMaintenance(cfg, repo, logger.New("error", nil))

	maintenanceAt(m, time.Date(2026, 10, 6, 4, 30, 15, 0, time.UTC))
	if _, err := m.Run(); err != nil {
		t.Fatal(err)
	}
	maintenanceAt(m, time.Date(2026, 10, 7, 4, 30, 15, 0, time.UTC))
	second, err := m.Run()
	if err != nil {
		t.Fatal(err)
	}
	if second.Kept != 2 || second.Removed != 0 {
		t.Fatalf("kept=%d removed=%d, want both copies inside the limit", second.Kept, second.Removed)
	}

	maintenanceAt(m, time.Date(2026, 10, 8, 4, 30, 15, 0, time.UTC))
	third, err := m.Run()
	if err != nil {
		t.Fatal(err)
	}
	if third.Kept != 2 || third.Removed != 1 {
		t.Fatalf("kept=%d removed=%d, want the oldest copy retired", third.Kept, third.Removed)
	}
	if _, err := os.Stat(filepath.Join(cfg.Archive.BackupDir, "sqlite3-20261006-043015.db")); !os.IsNotExist(err) {
		t.Errorf("the oldest copy should have been retired, got %v", err)
	}
	for _, name := range []string{"sqlite3-20261007-043015.db", "sqlite3-20261008-043015.db"} {
		if _, err := os.Stat(filepath.Join(cfg.Archive.BackupDir, name)); err != nil {
			t.Errorf("copy %s must survive: %v", name, err)
		}
	}
}

func TestArchiveMaintenanceDoesNothingWhenItIsSwitchedOff(t *testing.T) {
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "sqlite3.db")}
	m := newArchiveMaintenance(cfg, nil, logger.New("error", nil))

	report, err := m.Run()
	if err != nil {
		t.Fatalf("maintenance: %v", err)
	}
	if report.Backup || report.Retention || report.Path != "" || report.PrunedRows != 0 {
		t.Fatalf("report = %+v, want an empty run", report)
	}
}
