package sqlitedb

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func prepareBlockedDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE rows (n INTEGER NOT NULL)",
		"INSERT INTO rows (n) VALUES (1)",
		"PRAGMA journal_mode=DELETE",
		"BEGIN EXCLUSIVE",
		"UPDATE rows SET n = n",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	return db
}

func TestOpenPinsThePoolAndAdoptsTheRequestedJournalMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")

	db, err := Open(path, Options{BusyTimeout: 400 * time.Millisecond, JournalMode: JournalWAL})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("pool allows %d connections, want 1", got)
	}

	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
		t.Fatal(err)
	}
	if timeout != 400 {
		t.Fatalf("busy_timeout = %d, want the configured 400", timeout)
	}

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, JournalWAL) {
		t.Fatalf("journal_mode = %q, want %q", mode, JournalWAL)
	}
}

func TestOpenRefusesAJournalModeTheDatabaseDidNotAdopt(t *testing.T) {
	_, err := Open(":memory:", Options{BusyTimeout: time.Second, JournalMode: JournalWAL})
	if err == nil {
		t.Fatal("a database that cannot use WAL must not open as if it did")
	}
	if !strings.Contains(err.Error(), "journal_mode") {
		t.Fatalf("error does not name the journal mode: %v", err)
	}
}

func TestOpenRefusesAnUnsupportedJournalModeName(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "archive.db"), Options{JournalMode: "WAL; DROP TABLE rows"})
	if err == nil || !strings.Contains(err.Error(), "unsupported journal mode") {
		t.Fatalf("err = %v, want a refusal of the mode name", err)
	}
}

func TestOpenRefusesAPathThatAlreadyCarriesAQueryString(t *testing.T) {
	_, err := Open("file:app.db?mode=ro", Options{BusyTimeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "query string") {
		t.Fatalf("err = %v, want a refusal of the path", err)
	}
}

func TestOpenWaitsForABusyLockInsteadOfFailingAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	blocker := prepareBlockedDatabase(t, path)
	defer blocker.Close()

	db, err := Open(path, Options{BusyTimeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	started := time.Now()
	_, err = db.Exec("INSERT INTO rows (n) VALUES (2)")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("the write must not report success while an exclusive lock is held")
	}
	if !IsBusy(err) {
		t.Fatalf("the write failed with %v, want a busy error", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("the write gave up after %s, want it to wait for the configured 200ms lock", elapsed)
	}
}

func TestRetryRepeatsABusyWriteOnceTheBlockerGoesAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	blocker := prepareBlockedDatabase(t, path)
	defer blocker.Close()

	opts := DefaultOptions()
	opts.BusyTimeout = 100 * time.Millisecond
	opts.Retries = 1

	db, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	attempts, retries, failures := 0, 0, 0
	opts.OnRetry = func(attempt int, err error) {
		retries++
		if attempt != 1 {
			t.Errorf("retry attempt = %d, want 1", attempt)
		}
		if !IsBusy(err) {
			t.Errorf("retry saw %v, want a busy error", err)
		}
		if _, err := blocker.Exec("COMMIT"); err != nil {
			t.Errorf("releasing the blocker: %v", err)
		}
	}
	opts.OnFailure = func(error) { failures++ }

	err = opts.Retry(func() error {
		attempts++
		_, err := db.Exec("INSERT INTO rows (n) VALUES (7)")
		return err
	})

	if err != nil {
		t.Fatalf("the retried write must succeed once the lock is free: %v", err)
	}
	if attempts != 2 || retries != 1 {
		t.Fatalf("attempts = %d, retries = %d, want 2 and 1", attempts, retries)
	}
	if failures != 0 {
		t.Fatalf("failures = %d, want none", failures)
	}

	var rows int
	if err := db.QueryRow("SELECT COUNT(*) FROM rows WHERE n = 7").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("the retried write stored %d rows, want 1", rows)
	}
}

func TestRetryReportsABusyWriteItCouldNotRetryAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	blocker := prepareBlockedDatabase(t, path)
	defer blocker.Close()

	opts := DefaultOptions()
	opts.BusyTimeout = 50 * time.Millisecond
	opts.Retries = 1

	db, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	attempts, retries, failures := 0, 0, 0
	opts.OnRetry = func(int, error) { retries++ }
	opts.OnFailure = func(err error) {
		failures++
		if !IsBusy(err) {
			t.Errorf("failure saw %v, want a busy error", err)
		}
	}

	err = opts.Retry(func() error {
		attempts++
		_, err := db.Exec("INSERT INTO rows (n) VALUES (8)")
		return err
	})

	if !IsBusy(err) {
		t.Fatalf("err = %v, want the busy error to reach the caller", err)
	}
	if attempts != 2 || retries != 1 || failures != 1 {
		t.Fatalf("attempts = %d, retries = %d, failures = %d, want 2, 1 and 1", attempts, retries, failures)
	}
	if blocker == nil {
		t.Fatal("the blocker must stay held for the whole test")
	}
}

func TestRetryDoesNotRepeatOtherErrors(t *testing.T) {
	opts := DefaultOptions()
	opts.BusyTimeout = 50 * time.Millisecond

	attempts, retries, failures := 0, 0, 0
	opts.OnRetry = func(int, error) { retries++ }
	opts.OnFailure = func(error) { failures++ }

	wanted := fmt.Errorf("no such column: nope")
	err := opts.Retry(func() error {
		attempts++
		return wanted
	})

	if err != wanted {
		t.Fatalf("err = %v, want it passed through", err)
	}
	if attempts != 1 || retries != 0 || failures != 0 {
		t.Fatalf("attempts = %d, retries = %d, failures = %d, want 1, 0 and 0", attempts, retries, failures)
	}
}
