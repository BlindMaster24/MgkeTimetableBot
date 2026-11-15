package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTelegramReadsTheBellScheduleThroughTheChatCampus(t *testing.T) {
	dir := filepath.Join("..", "internal", "telegram")
	shared := filepath.Join(dir, "google_calendar.go")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the telegram package: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(source)
		if !strings.Contains(text, "GetCallsWeekdays(") && !strings.Contains(text, "GetCallsSaturday(") {
			continue
		}
		if path == shared {
			continue
		}
		t.Errorf("%s reads the global bell schedule, so a chat that picked its own campus sees the wrong times; use callsScheduleFor(chat)", name)
	}
}
