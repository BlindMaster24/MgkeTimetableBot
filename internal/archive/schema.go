package archive

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed migrations/*.sql
var migrations embed.FS

func (r *Repository) EnsureSchema() error {
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
