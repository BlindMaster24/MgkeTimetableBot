package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	DefaultBusyTimeout = 5 * time.Second
	DefaultRetries     = 1
	JournalWAL         = "WAL"
	SyncNormal         = "NORMAL"
	TxLockImmediate    = "immediate"

	openTimeout    = 5 * time.Second
	retryBaseDelay = 25 * time.Millisecond
	retryMaxDelay  = time.Second
)

var (
	journalModeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	syncModes     = map[string]bool{"OFF": true, "NORMAL": true, "FULL": true, "EXTRA": true}
	txLocks       = map[string]bool{"deferred": true, "immediate": true, "exclusive": true}
)

type Options struct {
	BusyTimeout time.Duration
	Retries     int
	JournalMode string
	Synchronous string
	TxLock      string
	OnRetry     func(attempt int, err error)
	OnFailure   func(err error)
}

func DefaultOptions() Options {
	return Options{BusyTimeout: DefaultBusyTimeout, Retries: DefaultRetries}
}

func Open(path string, opts Options) (*sql.DB, error) {
	dsn, err := dataSourceName(path, opts)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil && !IsBusy(err) {
		db.Close()
		return nil, fmt.Errorf("sqlite %s: ping: %w", path, err)
	}

	if err := ensureJournalMode(db, path, opts.JournalMode); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func dataSourceName(path string, opts Options) (string, error) {
	if strings.Contains(path, "?") {
		return "", fmt.Errorf("sqlite path %q contains a query string", path)
	}

	var params []string
	if opts.BusyTimeout > 0 {
		params = append(params, fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeout.Milliseconds()))
	}
	if mode := strings.ToUpper(strings.TrimSpace(opts.Synchronous)); mode != "" {
		if !syncModes[mode] {
			return "", fmt.Errorf("sqlite %s: unsupported synchronous mode %q", path, opts.Synchronous)
		}
		params = append(params, "_pragma=synchronous("+mode+")")
	}
	if lock := strings.ToLower(strings.TrimSpace(opts.TxLock)); lock != "" {
		if !txLocks[lock] {
			return "", fmt.Errorf("sqlite %s: unsupported transaction lock %q", path, opts.TxLock)
		}
		params = append(params, "_txlock="+lock)
	}
	if len(params) == 0 {
		return path, nil
	}
	return path + "?" + strings.Join(params, "&"), nil
}

func ensureJournalMode(db *sql.DB, path, mode string) error {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode == "" {
		return nil
	}
	if !journalModeRe.MatchString(mode) {
		return fmt.Errorf("sqlite %s: unsupported journal mode %q", path, mode)
	}

	var reported string
	if err := db.QueryRow("PRAGMA journal_mode=" + mode).Scan(&reported); err != nil {
		return fmt.Errorf("sqlite %s: journal mode: %w", path, err)
	}
	if !strings.EqualFold(reported, mode) {
		return fmt.Errorf("sqlite %s: asked for journal_mode=%s, the database reports %q", path, mode, reported)
	}
	return nil
}

func (o Options) Retry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; ; attempt++ {
		err = fn()
		if err == nil || !IsBusy(err) {
			return err
		}
		if attempt >= o.Retries {
			if o.OnFailure != nil {
				o.OnFailure(err)
			}
			return err
		}
		if o.OnRetry != nil {
			o.OnRetry(attempt+1, err)
		}
		if err := waitBackoff(ctx, attempt); err != nil {
			return err
		}
	}
}

func waitBackoff(ctx context.Context, attempt int) error {
	delay := retryMaxDelay
	if attempt < 16 {
		if grown := retryBaseDelay << attempt; grown < retryMaxDelay {
			delay = grown
		}
	}
	half := delay / 2
	if half > 0 {
		delay = half + time.Duration(rand.Int64N(int64(half)+1))
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func IsBusy(err error) bool {
	var busy *sqlite.Error
	if !errors.As(err, &busy) {
		return false
	}

	switch busy.Code() & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return true
	}
	return false
}
