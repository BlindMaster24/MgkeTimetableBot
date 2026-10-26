package archive

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/blindmaster24/MgkeTimetableBot/internal/sqlitedb"
)

const (
	ApplicationID = 0x4D474B41
	SchemaVersion = 1
)

var archiveTables = []string{"timetable_archive"}

//go:embed migrations/*.sql
var migrations embed.FS

func (r *Repository) EnsureSchema() error {
	if err := sqlitedb.CheckIdentity(r.db, r.path, sqlitedb.Identity{
		ApplicationID: ApplicationID,
		SchemaVersion: SchemaVersion,
		Tables:        archiveTables,
	}); err != nil {
		return err
	}

	entries, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)

	for _, entry := range entries {
		statement, err := migrations.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry, err)
		}
		if _, err := r.db.Exec(string(statement)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry, err)
		}
	}
	return nil
}
