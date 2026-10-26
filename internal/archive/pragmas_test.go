package archive

import (
	"strings"
	"testing"
)

func TestArchiveConnectionUsesWALAndNormalSynchronous(t *testing.T) {
	repo := newTestRepo(t)

	var journal string
	if err := repo.DB().QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journal, "wal") {
		t.Errorf("journal_mode = %q, want wal", journal)
	}

	var synchronous int
	if err := repo.DB().QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if synchronous != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}

	var appID int64
	if err := repo.DB().QueryRow("PRAGMA application_id").Scan(&appID); err != nil {
		t.Fatal(err)
	}
	if appID != ApplicationID {
		t.Errorf("application_id = 0x%X, want 0x%X", appID, ApplicationID)
	}
}
