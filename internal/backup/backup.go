package backup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/sqlitedb"
)

var (
	ErrNoDirectory = errors.New("backup directory is not configured")
	ErrNoPrefix    = errors.New("backup file prefix is empty")
)

var stampRe = regexp.MustCompile(`^\d{8}-\d{6}$`)

const stampLayout = "20060102-150405"

type Options struct {
	DBPath        string
	Dir           string
	Keep          int
	Now           time.Time
	BusyTimeout   time.Duration
	PrunePrefixes []string
}

type Result struct {
	Path    string
	Removed []string
	Kept    []string
}

type owned struct {
	name  string
	stamp time.Time
}

func Run(opts Options) (Result, error) {
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return Result{}, ErrNoDirectory
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}

	dest := filepath.Join(dir, Name(opts.DBPath, opts.Now))
	if err := SnapshotWithOptions(opts.DBPath, dest, opts); err != nil {
		return Result{}, err
	}

	prefixes := append([]string{Prefix(opts.DBPath)}, opts.PrunePrefixes...)
	removed, kept, err := Prune(dir, prefixes, opts.Keep)
	result := Result{Path: dest, Removed: removed, Kept: kept}
	if err != nil {
		return result, err
	}
	return result, nil
}

func Prefix(dbPath string) string {
	base := filepath.Base(strings.TrimSpace(dbPath))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func Name(dbPath string, now time.Time) string {
	return fmt.Sprintf("%s-%s.db", Prefix(dbPath), now.Format(stampLayout))
}

func Snapshot(dbPath, dest string) error {
	return SnapshotWithOptions(dbPath, dest, Options{})
}

func SnapshotWithOptions(dbPath, dest string, opts Options) error {
	db, err := sqlitedb.Open(dbPath, sqlitedb.Options{BusyTimeout: snapshotBusyTimeout(opts)})
	if err != nil {
		return fmt.Errorf("snapshot %s: %w", dbPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("snapshot %s: %w", dbPath, err)
	}
	return nil
}

func snapshotBusyTimeout(opts Options) time.Duration {
	if opts.BusyTimeout <= 0 {
		return sqlitedb.DefaultBusyTimeout
	}
	return opts.BusyTimeout
}

func Prune(dir string, prefixes []string, keep int) ([]string, []string, error) {
	clean := uniquePrefixes(prefixes)
	if len(clean) == 0 {
		return nil, nil, ErrNoPrefix
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	var ownedFiles []owned
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		for _, prefix := range clean {
			stamp, ok := parseStamp(entry.Name(), prefix)
			if !ok {
				continue
			}
			ownedFiles = append(ownedFiles, owned{name: entry.Name(), stamp: stamp})
			break
		}
	}

	sort.Slice(ownedFiles, func(i, j int) bool {
		if !ownedFiles[i].stamp.Equal(ownedFiles[j].stamp) {
			return ownedFiles[i].stamp.After(ownedFiles[j].stamp)
		}
		return ownedFiles[i].name > ownedFiles[j].name
	})

	var removed, kept []string
	for i, file := range ownedFiles {
		if keep <= 0 || i < keep {
			kept = append(kept, file.name)
			continue
		}
		removed = append(removed, file.name)
	}

	var failures []error
	for _, name := range removed {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			failures = append(failures, err)
		}
	}

	sort.Strings(removed)

	return removed, kept, errors.Join(failures...)
}

func uniquePrefixes(prefixes []string) []string {
	seen := make(map[string]bool, len(prefixes))
	var clean []string
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" || seen[prefix] {
			continue
		}
		seen[prefix] = true
		clean = append(clean, prefix)
	}
	return clean
}

func parseStamp(name, prefix string) (time.Time, bool) {
	if !strings.HasSuffix(name, ".db") {
		return time.Time{}, false
	}

	stamp := strings.TrimSuffix(name, ".db")
	if !strings.HasPrefix(stamp, prefix+"-") {
		return time.Time{}, false
	}
	stamp = strings.TrimPrefix(stamp, prefix+"-")
	if !stampRe.MatchString(stamp) {
		return time.Time{}, false
	}

	parsed, err := time.Parse(stampLayout, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
