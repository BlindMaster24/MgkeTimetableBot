package sqlitedb

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDataSourceNameCarriesTheConfiguredPragmas(t *testing.T) {
	dsn, err := dataSourceName("archive.db", Options{
		BusyTimeout: 2 * time.Second,
		Synchronous: "normal",
		TxLock:      "immediate",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "archive.db?_pragma=busy_timeout(2000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	if dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

func TestDataSourceNameRejectsBrokenOptions(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"synchronous", Options{Synchronous: "sometimes"}},
		{"txlock", Options{TxLock: "whenever"}},
		{"query", Options{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "archive.db"
			if tc.name == "query" {
				path += "?mode=ro"
			}
			if _, err := dataSourceName(path, tc.opts); err == nil {
				t.Fatalf("dataSourceName(%q) = nil error, want a refusal", path)
			}
		})
	}
}

func TestRetryStopsWhenTheContextIsCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	blocker := prepareBlockedDatabase(t, path)
	defer blocker.Close()

	opts := DefaultOptions()
	opts.BusyTimeout = 50 * time.Millisecond
	opts.Retries = 3

	db, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err = opts.Retry(ctx, func() error {
		attempts++
		cancel()
		_, err := db.Exec("INSERT INTO rows (n) VALUES (9)")
		return err
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation to reach the caller", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1: a cancelled context must stop the retry", attempts)
	}
}

func TestRetryWaitsBetweenAttempts(t *testing.T) {
	if retryBaseDelay <= 0 || retryMaxDelay < retryBaseDelay {
		t.Fatalf("retry delays are broken: base %v, max %v", retryBaseDelay, retryMaxDelay)
	}

	opts := DefaultOptions()
	opts.BusyTimeout = 50 * time.Millisecond
	opts.Retries = 1

	path := filepath.Join(t.TempDir(), "archive.db")
	blocker := prepareBlockedDatabase(t, path)
	defer blocker.Close()

	db, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var firstAttempt, secondAttempt time.Time
	opts.OnRetry = func(int, error) { _, _ = blocker.Exec("COMMIT") }

	err = opts.Retry(context.Background(), func() error {
		if firstAttempt.IsZero() {
			firstAttempt = time.Now()
		} else {
			secondAttempt = time.Now()
		}
		_, err := db.Exec("INSERT INTO rows (n) VALUES (10)")
		return err
	})
	if err != nil {
		t.Fatalf("the retried write must succeed: %v", err)
	}
	if secondAttempt.IsZero() {
		t.Fatal("the write was not retried")
	}
	if gap := secondAttempt.Sub(firstAttempt); gap < retryBaseDelay/2 {
		t.Errorf("the retry fired after %v, want a wait of at least %v", gap, retryBaseDelay/2)
	}
}

func TestOpenRefusesAPathItCannotUse(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir, Options{})
	if err == nil {
		t.Fatal("opening a directory as a database must fail")
	}
	if !strings.Contains(err.Error(), "sqlite") {
		t.Errorf("error %q must name the database layer", err)
	}
}

func TestWaitBackoffGrowsAndSaturates(t *testing.T) {
	ctx := t.Context()

	for _, attempt := range []int{0, 1, 5, 15} {
		started := time.Now()
		if err := waitBackoff(ctx, attempt); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		waited := time.Since(started)
		want := retryBaseDelay << attempt
		if want > retryMaxDelay || attempt >= 16 {
			want = retryMaxDelay
		}
		// Jitter cuts the delay to 50-100% of the calculated value.
		if waited < want/2 || waited > want+50*time.Millisecond {
			t.Errorf("attempt %d waited %v, want %v..%v", attempt, waited, want/2, want)
		}
	}

	started := time.Now()
	if err := waitBackoff(ctx, 16); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(started); waited < retryMaxDelay/2 {
		t.Errorf("attempt 16 waited %v, want at least %v", waited, retryMaxDelay/2)
	}
}

func TestDataSourceNameWithoutABusyTimeoutLeavesThePragmaOut(t *testing.T) {
	dsn, err := dataSourceName("cache.db", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dsn, "busy_timeout") {
		t.Fatalf("dsn = %q, want no busy_timeout pragma for a zero timeout", dsn)
	}

	dsn, err = dataSourceName("cache.db", Options{BusyTimeout: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "_pragma=busy_timeout(250)") {
		t.Fatalf("dsn = %q, want the configured timeout", dsn)
	}
}

func TestReadPragmaIntReportsAClosedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragma.db")
	db, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	if _, err := readPragmaInt(db, path, "user_version"); err == nil {
		t.Fatal("reading a pragma from a closed database must fail")
	}
}
