package telegram

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
)

func TestChatRepositoryRefusesTheArchiveDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timetable_archive.db")

	repo, err := archive.New(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	chatRepo, err := NewChatRepo(path, 0)
	if err == nil {
		chatRepo.Close()
		t.Fatal("opening the archive database as the chat database must fail")
	}
	if !strings.Contains(err.Error(), "belongs to another database") {
		t.Errorf("error %q must explain that the file is foreign", err)
	}
}

func TestArchiveRefusesTheChatDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bot_chats.db")

	chatRepo, err := NewChatRepo(path, 0)
	if err != nil {
		t.Fatalf("open chats: %v", err)
	}
	if err := chatRepo.Close(); err != nil {
		t.Fatal(err)
	}

	repo, err := archive.New(path)
	if err == nil {
		repo.Close()
		t.Fatal("opening the chat database as the archive must fail")
	}
	if !strings.Contains(err.Error(), "belongs to another database") {
		t.Errorf("error %q must explain that the file is foreign", err)
	}
}
