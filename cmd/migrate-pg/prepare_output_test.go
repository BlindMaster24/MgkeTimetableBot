package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareOutputNeedsForceToReplaceAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bot_chats.db")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := prepareOutput(path, false)
	if err == nil || !strings.Contains(err.Error(), "pass -force to replace it") {
		t.Fatalf("err = %v, want the force hint", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("the file must survive without -force: %v", statErr)
	}

	if err := prepareOutput(path, true); err != nil {
		t.Fatalf("force must replace the file: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("force must remove the old file, stat error = %v", statErr)
	}
}

func TestPrepareOutputAcceptsAMissingFile(t *testing.T) {
	if err := prepareOutput(filepath.Join(t.TempDir(), "new.db"), false); err != nil {
		t.Fatalf("a fresh path must pass: %v", err)
	}
}
