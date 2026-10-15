package sqlitedb

import (
	"database/sql"
	"errors"
	"fmt"
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
)

var journalModeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

type Options struct {
	BusyTimeout time.Duration
	Retries     int
	JournalMode string
	OnRetry     func(attempt int, err error)
	OnFailure   func(err error)
}

func DefaultOptions() Options {
	return Options{BusyTimeout: DefaultBusyTimeout, Retries: DefaultRetries}
}

func Open(path string, opts Options) (*sql.DB, error) {
	dsn, err := dataSourceName(path, opts.BusyTimeout)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := ensureJournalMode(db, path, opts.JournalMode); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func dataSourceName(path string, busyTimeout time.Duration) (string, error) {
	if strings.Contains(path, "?") {
		return "", fmt.Errorf("sqlite path %q contains a query string", path)
	}
	if busyTimeout <= 0 {
		return path, nil
	}
	return fmt.Sprintf("%s?_pragma=busy_timeout(%d)", path, busyTimeout.Milliseconds()), nil
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

func (o Options) Retry(fn func() error) error {
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
