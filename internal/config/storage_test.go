package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvedDBPathDefaultsToTheArchiveName(t *testing.T) {
	cfg := &Config{}
	if got := cfg.ResolvedDBPath(); got != DefaultDBPath {
		t.Errorf("ResolvedDBPath() = %q, want %q", got, DefaultDBPath)
	}
	if DefaultDBPath != "./timetable_archive.db" {
		t.Errorf("DefaultDBPath = %q, want ./timetable_archive.db", DefaultDBPath)
	}
}

func TestResolvedDBPathKeepsTheConfiguredPath(t *testing.T) {
	cfg := &Config{DBPath: "/srv/mgke/archive.db"}
	if got := cfg.ResolvedDBPath(); got != "/srv/mgke/archive.db" {
		t.Errorf("ResolvedDBPath() = %q, want the configured path", got)
	}
}

func TestAdoptLegacyDBPathFindsTheOldName(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, LegacyDBName)
	if err := os.WriteFile(legacy, []byte("old archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, filepath.Base(DefaultDBPath))
	got, adopted := AdoptLegacyDBPath(target)
	if !adopted {
		t.Fatal("an existing sqlite3.db next to the archive name must be adopted")
	}
	if got != legacy {
		t.Errorf("adopted path = %q, want %q", got, legacy)
	}
}

func TestAdoptLegacyDBPathPrefersAnExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, filepath.Base(DefaultDBPath))
	for _, path := range []string{target, filepath.Join(dir, LegacyDBName)} {
		if err := os.WriteFile(path, []byte("db"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got, adopted := AdoptLegacyDBPath(target); adopted || got != target {
		t.Errorf("AdoptLegacyDBPath() = %q, %v, want the configured target untouched", got, adopted)
	}
}

func TestAdoptLegacyDBPathLeavesOtherNamesAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LegacyDBName), []byte("old archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, adopted := AdoptLegacyDBPath(filepath.Join(dir, "archive-prod.db")); adopted || got != filepath.Join(dir, "archive-prod.db") {
		t.Errorf("AdoptLegacyDBPath() = %q, %v, want a deliberate path untouched", got, adopted)
	}
}

func TestAdoptLegacyDBPathIgnoresTheLegacyNameItself(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, LegacyDBName)
	if err := os.WriteFile(legacy, []byte("old archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, adopted := AdoptLegacyDBPath(legacy); adopted || got != legacy {
		t.Errorf("AdoptLegacyDBPath(%q) = %q, %v, want no adoption", legacy, got, adopted)
	}
}
