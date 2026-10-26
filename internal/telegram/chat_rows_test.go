package telegram

import (
	"path/filepath"
	"testing"
)

func TestChatQueriesReportScanFailures(t *testing.T) {
	repo, err := NewChatRepo(filepath.Join(t.TempDir(), "bot_chats.db"), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	if _, err := repo.DB().Exec(
		`INSERT INTO bot_chats (service, peer_id, accepted, allow_send_mess, mode) VALUES ('telegram', 'not-a-number', 1, 1, 'student')`,
	); err != nil {
		t.Fatal(err)
	}

	chats, err := repo.FindAllTGChats()
	if err == nil {
		t.Fatalf("FindAllTGChats() = %d chats, nil error; a row it cannot scan must be reported", len(chats))
	}
	if chats != nil {
		t.Errorf("chats = %v, want no partial result next to the error", chats)
	}
}

func TestChatQueriesReportBrokenRowsInFindAllWithNotifications(t *testing.T) {
	repo, err := NewChatRepo(filepath.Join(t.TempDir(), "bot_chats.db"), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	if _, err := repo.DB().Exec(
		`INSERT INTO bot_chats (service, peer_id, accepted, allow_send_mess, mode, notice_changes) VALUES ('telegram', 'broken', 1, 1, 'student', 1)`,
	); err != nil {
		t.Fatal(err)
	}

	chats, err := repo.FindAllWithNotifications("telegram")
	if err == nil {
		t.Fatalf("FindAllWithNotifications() = %d chats, nil error; a broken row must not be skipped", len(chats))
	}
}
