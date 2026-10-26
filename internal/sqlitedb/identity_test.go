package sqlitedb

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

var (
	testAppID   = int64(0x4D474B54)
	testVersion = int64(1)
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path, Options{JournalMode: JournalWAL})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func identity(tables ...string) Identity {
	return Identity{ApplicationID: testAppID, SchemaVersion: testVersion, Tables: tables}
}

func TestCheckIdentityMarksAFreshFile(t *testing.T) {
	db, path := openTestDB(t)

	if err := CheckIdentity(db, path, identity("items")); err != nil {
		t.Fatalf("CheckIdentity() = %v, want nil", err)
	}

	appID, err := readPragmaInt(db, path, "application_id")
	if err != nil {
		t.Fatal(err)
	}
	if appID != testAppID {
		t.Errorf("application_id = 0x%X, want 0x%X", appID, testAppID)
	}
	version, err := readPragmaInt(db, path, "user_version")
	if err != nil {
		t.Fatal(err)
	}
	if version != testVersion {
		t.Errorf("user_version = %d, want %d", version, testVersion)
	}
}

func TestCheckIdentityAcceptsItsOwnDatabaseAgain(t *testing.T) {
	db, path := openTestDB(t)
	if err := CheckIdentity(db, path, identity("items")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	if err := CheckIdentity(db, path, identity("items")); err != nil {
		t.Errorf("reopening the same database failed: %v", err)
	}
}

func TestCheckIdentityAdoptsALegacyFile(t *testing.T) {
	db, path := openTestDB(t)
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	if err := CheckIdentity(db, path, identity("items")); err != nil {
		t.Fatalf("a zero application_id with our tables must be adopted: %v", err)
	}
}

func TestCheckIdentityRefusesForeignTables(t *testing.T) {
	db, path := openTestDB(t)
	if _, err := db.Exec(`CREATE TABLE other_app_rows (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	err := CheckIdentity(db, path, identity("items"))
	if err == nil {
		t.Fatal("CheckIdentity() = nil, want a refusal for foreign tables")
	}
	if !strings.Contains(err.Error(), "other_app_rows") {
		t.Errorf("error %q must name the foreign table", err)
	}
}

func TestCheckIdentityRefusesAnotherApplication(t *testing.T) {
	db, path := openTestDB(t)
	if err := CheckIdentity(db, path, Identity{ApplicationID: testAppID + 1, SchemaVersion: testVersion, Tables: []string{"items"}}); err != nil {
		t.Fatal(err)
	}

	err := CheckIdentity(db, path, identity("items"))
	if err == nil {
		t.Fatal("CheckIdentity() = nil, want a refusal for another application id")
	}
	if !strings.Contains(err.Error(), "belongs to another database") {
		t.Errorf("error %q must explain that the file is foreign", err)
	}
}

func TestCheckIdentityRefusesANewerSchema(t *testing.T) {
	db, path := openTestDB(t)
	if err := CheckIdentity(db, path, Identity{ApplicationID: testAppID, SchemaVersion: testVersion + 2, Tables: []string{"items"}}); err != nil {
		t.Fatal(err)
	}

	err := CheckIdentity(db, path, identity("items"))
	if err == nil {
		t.Fatal("CheckIdentity() = nil, want a refusal for a newer schema")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error %q must mention the newer schema", err)
	}
}

func TestSetPragmaIntRefusesAnUnknownPragma(t *testing.T) {
	db, path := openTestDB(t)

	if err := setPragmaInt(db, path, "journal_mode", 1); err == nil {
		t.Fatal("setPragmaInt() = nil, want a refusal for an unknown pragma")
	}
}
