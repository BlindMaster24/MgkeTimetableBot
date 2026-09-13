package archive

import (
	"path/filepath"
	"testing"
)

func TestNewAppliesTheEmbeddedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")

	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.DayIndexBounds(); err != nil {
		t.Fatalf("a fresh archive must be usable right away: %v", err)
	}

	var tables int
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'timetable_archive'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 1 {
		t.Fatalf("timetable_archive missing, found %d", tables)
	}

	for _, index := range []string{"idx_group_day", "idx_teacher_day"} {
		var count int
		if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("index %s was not created", index)
		}
	}

	if err := repo.AppendDays([]AppendDay{{
		Type:  "group",
		Value: "100",
		Day:   map[string]any{"day": "12.02.2026", "lessons": []any{map[string]any{"lesson": "Математика"}}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(path)
	if err != nil {
		t.Fatalf("reopening an existing archive must not fail: %v", err)
	}
	defer reopened.Close()

	days, err := reopened.GroupDays("100", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].Day != "12.02.2026" {
		t.Fatalf("stored day was lost: %+v", days)
	}
}
