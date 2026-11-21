package telegram

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
)

func adapterService(t *testing.T) calendarServiceAdapter {
	t.Helper()
	cfg := &config.Config{}
	cfg.Google.OAuth.ClientID = "client-id"
	cfg.Google.OAuth.ClientSecret = "client-secret"
	cfg.Google.RedirectDomain = "https://bot.example"
	cfg.Google.URL = "/google/oauth"
	return calendarServiceAdapter{service: google.NewCalendarService(cfg)}
}

func TestTheCalendarAdapterForwardsTheConfiguration(t *testing.T) {
	adapter := adapterService(t)

	if !adapter.Configured() {
		t.Fatal("a complete OAuth config must count as configured")
	}
	if !adapter.SyncEnabled() {
		// No service account is configured in this fixture, so the sync stays off.
	} else {
		t.Fatal("a missing service account must keep the sync off")
	}
	if url := adapter.AuthURL("state-1"); url == "" {
		t.Fatal("AuthURL must build a URL")
	}
}

func TestTheCalendarAdapterSurfacesExchangeAndUserErrors(t *testing.T) {
	adapter := adapterService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := adapter.Exchange(ctx, "code"); err == nil {
		t.Fatal("a cancelled exchange must be an error")
	}
	if err := adapter.SyncDay(ctx, "calendar", "07.09.2026", nil, google.Schedule{}); err == nil {
		t.Fatal("a cancelled sync must be an error")
	}
	api, err := adapter.UserClient(context.Background(), google.Credentials{}, nil)
	if err != nil {
		t.Fatalf("building the user client must not touch the network: %v", err)
	}
	if api == nil {
		t.Fatal("the user client must be returned")
	}
}

func TestCallsCampusLineSpeaksOnlyWhenACampusIsSelected(t *testing.T) {
	bot, _ := setupE2EBot(t)
	bot.cache.SetCallsSiteVariants(t.Context(), []cache.CallsVariant{{Name: "Кн"}, {Name: "Р"}})

	chat := &Chat{ID: 1, PeerID: 1, Accepted: true}
	if line := bot.callsCampusLine(chat); line != "" {
		t.Fatalf("a chat without a campus must have no line, got %q", line)
	}
	chat.CallsCampus = "Кн"
	if line := bot.callsCampusLine(chat); line == "" {
		t.Fatal("a chat with a campus must show the current campus")
	}
}

func TestCleanupTempFilesDropsOnlyOldFiles(t *testing.T) {
	bot, _ := setupE2EBot(t)
	dir := t.TempDir()

	stale := filepath.Join(dir, "stale.png")
	fresh := filepath.Join(dir, "fresh.png")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatal(err)
	}

	bot.CleanupTempFiles(dir, time.Hour)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("a file older than the limit must be removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("a fresh file must stay: %v", err)
	}
	bot.CleanupTempFiles(filepath.Join(dir, "missing"), time.Hour)
}
