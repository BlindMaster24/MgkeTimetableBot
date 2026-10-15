package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
)

func fixedClock() time.Time {
	return time.Date(2026, 10, 6, 4, 30, 15, 0, time.UTC)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendDay(t *testing.T, repo *archive.Repository, day, group string) {
	t.Helper()

	err := repo.AppendDays([]archive.AppendDay{{
		Type:  "group",
		Value: group,
		Day:   map[string]any{"day": day, "lessons": []any{map[string]any{"lesson": "Математика"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func storedDays(t *testing.T, path string) []string {
	t.Helper()

	repo, err := archive.New(path)
	if err != nil {
		t.Fatalf("the file is not a usable database: %v", err)
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

func TestSnapshotOfALiveDatabaseIsReadable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sqlite3.db")

	repo, err := archive.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	appendDay(t, repo, "12.02.2026", "100")
	appendDay(t, repo, "13.02.2026", "100")

	backupDir := filepath.Join(dir, "backups")
	result, err := Run(Options{DBPath: dbPath, Dir: backupDir, Keep: 2, Now: fixedClock()})
	if err != nil {
		t.Fatalf("backup of a database that is still open failed: %v", err)
	}
	if want := filepath.Join(backupDir, "sqlite3-20261006-043015.db"); result.Path != want {
		t.Fatalf("path = %q, want %q", result.Path, want)
	}

	days := storedDays(t, result.Path)
	if len(days) != 2 || days[0] != "12.02.2026" || days[1] != "13.02.2026" {
		t.Fatalf("snapshot holds %v, want both days", days)
	}

	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotWaitsForAnExclusiveWriter(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sqlite3.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE archive_rows (day INTEGER NOT NULL, data TEXT NOT NULL)",
		"INSERT INTO archive_rows (day, data) VALUES (1, 'kept')",
		"PRAGMA journal_mode=DELETE",
		"BEGIN EXCLUSIVE",
		"UPDATE archive_rows SET data = data",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	dest := filepath.Join(dir, "snapshot.db")
	snapshot := make(chan error, 1)
	go func() { snapshot <- Snapshot(dbPath, dest) }()

	select {
	case err := <-snapshot:
		t.Fatalf("the snapshot failed while a writer held the lock instead of waiting for it: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	if _, err := db.Exec("COMMIT"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-snapshot:
		if err != nil {
			t.Fatalf("the snapshot must succeed once the writer released the lock: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the snapshot never finished after the writer released the lock")
	}

	check, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()

	var rows int
	if err := check.QueryRow("SELECT COUNT(*) FROM archive_rows").Scan(&rows); err != nil {
		t.Fatalf("the snapshot is not a readable database: %v", err)
	}
	if rows != 1 {
		t.Fatalf("snapshot holds %d rows, want the committed one", rows)
	}
}

func TestSnapshotSucceedsWhileAWriterIsMidTransaction(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sqlite3.db")

	repo, err := archive.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	appendDay(t, repo, "12.02.2026", "100")

	tx, err := repo.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE timetable_archive SET data = data WHERE "group" = ?`, "100"); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "snapshot.db")
	if err := Snapshot(dbPath, dest); err != nil {
		t.Fatalf("the snapshot must read a live WAL database while a writer is mid-transaction: %v", err)
	}

	days := storedDays(t, dest)
	if len(days) != 1 || days[0] != "12.02.2026" {
		t.Fatalf("snapshot holds %v, want the day that was committed before it started", days)
	}
}

func TestPruneRemovesOnlyItsOwnOldestBackups(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Dir(dir)

	decoys := map[string]string{
		"sqlite3-notes.db":               "no stamp",
		"sqlite3-20260101-0000.db":       "short stamp",
		"sqlite3-20260101-000000.db.bak": "wrong suffix",
		"archive-20260101-000000.db":     "foreign prefix",
		"keep-me.txt":                    "unrelated file",
	}
	for name, content := range decoys {
		writeFile(t, filepath.Join(dir, name), content)
	}
	if err := os.Mkdir(filepath.Join(dir, "sqlite3-20200101-000000.db"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(parent, "sqlite3-20190101-000000.db"), "outside the directory")

	var owned []string
	for _, name := range []string{
		"sqlite3-20260101-000000.db",
		"sqlite3-20260102-000000.db",
		"sqlite3-20260103-000000.db",
		"sqlite3-20260104-000000.db",
	} {
		writeFile(t, filepath.Join(dir, name), "snapshot")
		owned = append(owned, name)
	}

	removed, kept, err := Prune(dir, "sqlite3", 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(removed) != 2 || removed[0] != "sqlite3-20260101-000000.db" || removed[1] != "sqlite3-20260102-000000.db" {
		t.Fatalf("removed = %v, want the two oldest snapshots", removed)
	}
	if len(kept) != 2 || kept[0] != "sqlite3-20260104-000000.db" || kept[1] != "sqlite3-20260103-000000.db" {
		t.Fatalf("kept = %v, want the two newest snapshots", kept)
	}

	for name := range decoys {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("retention deleted %q, which it does not own: %v", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "sqlite3-20200101-000000.db")); err != nil || !info.IsDir() {
		t.Errorf("retention touched the directory named like a backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "sqlite3-20190101-000000.db")); err != nil {
		t.Errorf("retention reached outside its directory: %v", err)
	}
	for _, name := range []string{"sqlite3-20260103-000000.db", "sqlite3-20260104-000000.db"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("retention deleted the kept snapshot %q: %v", name, err)
		}
	}
	if len(owned) != 4 {
		t.Fatalf("fixture planted %d snapshots", len(owned))
	}
}

func TestPruneKeepsEveryBackupWithoutALimit(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sqlite3-20250101-000000.db", "sqlite3-20260101-000000.db"} {
		writeFile(t, filepath.Join(dir, name), "snapshot")
	}

	removed, kept, err := Prune(dir, "sqlite3", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	if len(kept) != 2 {
		t.Fatalf("kept = %v, want both snapshots", kept)
	}
}

func TestPruneRefusesAnEmptyPrefix(t *testing.T) {
	if _, _, err := Prune(t.TempDir(), "", 1); err != ErrNoPrefix {
		t.Fatalf("err = %v, want %v", err, ErrNoPrefix)
	}
}

func TestRunRefusesWithoutADirectory(t *testing.T) {
	if _, err := Run(Options{DBPath: "sqlite3.db", Now: fixedClock()}); err != ErrNoDirectory {
		t.Fatalf("err = %v, want %v", err, ErrNoDirectory)
	}
}

func TestSnapshotRefusesToOverwriteAnExistingBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sqlite3.db")

	repo, err := archive.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	dest := filepath.Join(dir, Name(dbPath, fixedClock()))
	writeFile(t, dest, "not a backup")

	if err := Snapshot(dbPath, dest); err == nil {
		t.Fatal("a second snapshot into the same file must fail instead of silently replacing it")
	}
	if content, err := os.ReadFile(dest); err != nil || string(content) != "not a backup" {
		t.Fatalf("the existing file was overwritten: %q %v", content, err)
	}
}
