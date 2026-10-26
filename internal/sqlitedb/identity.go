package sqlitedb

import (
	"database/sql"
	"fmt"
	"strings"
)

type Identity struct {
	ApplicationID int64
	SchemaVersion int64
	Tables        []string
}

func CheckIdentity(db *sql.DB, path string, want Identity) error {
	current, err := readPragmaInt(db, path, "application_id")
	if err != nil {
		return err
	}

	switch current {
	case 0:
		unknown, err := foreignTables(db, path, want.Tables)
		if err != nil {
			return err
		}
		if len(unknown) > 0 {
			return fmt.Errorf("sqlite %s: the file already holds tables %s, so it belongs to another database", path, strings.Join(unknown, ", "))
		}
		if err := setPragmaInt(db, path, "application_id", want.ApplicationID); err != nil {
			return err
		}
	case want.ApplicationID:
	default:
		return fmt.Errorf("sqlite %s: application_id 0x%X is not 0x%X, so this file belongs to another database; check db_path and chat_db_path", path, current, want.ApplicationID)
	}

	version, err := readPragmaInt(db, path, "user_version")
	if err != nil {
		return err
	}
	if version > want.SchemaVersion {
		return fmt.Errorf("sqlite %s: schema version %d is newer than the supported %d", path, version, want.SchemaVersion)
	}
	if version < want.SchemaVersion {
		if err := setPragmaInt(db, path, "user_version", want.SchemaVersion); err != nil {
			return err
		}
	}
	return nil
}

func foreignTables(db *sql.DB, path string, allowed []string) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite %s: list tables: %w", path, err)
	}
	defer rows.Close()

	known := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		known[name] = true
	}

	var unknown []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("sqlite %s: scan a table name: %w", path, err)
		}
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite %s: list tables: %w", path, err)
	}
	return unknown, nil
}

func readPragmaInt(db *sql.DB, path, name string) (int64, error) {
	var value int64
	if err := db.QueryRow("PRAGMA " + name).Scan(&value); err != nil {
		return 0, fmt.Errorf("sqlite %s: read %s: %w", path, name, err)
	}
	return value, nil
}

func setPragmaInt(db *sql.DB, path, name string, value int64) error {
	switch name {
	case "application_id", "user_version":
	default:
		return fmt.Errorf("sqlite %s: refusing to set pragma %q", path, name)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA %s = %d", name, value)); err != nil {
		return fmt.Errorf("sqlite %s: set %s: %w", path, name, err)
	}
	return nil
}
