package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestDevProfileStaysOffline(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "configs", "config.dev.yaml"))
	if err != nil {
		t.Fatalf("load the dev config: %v", err)
	}

	if cfg.Parser.Enabled {
		t.Error("the dev profile must not fetch the live timetable site")
	}
	if cfg.Telegram.Noticer {
		t.Error("the dev profile must not start the notification scheduler")
	}
	if cfg.Telegram.Webhook.Enabled {
		t.Error("the dev profile must stay on long polling against the stub")
	}
	if cfg.Metrics.Enabled {
		t.Error("the dev profile does not need the metrics route")
	}
	if cfg.Health == nil || !cfg.Health.Disabled {
		t.Error("the dev profile must not raise health alerts")
	}

	for name, path := range map[string]string{
		"db_path":      cfg.DBPath,
		"chat_db_path": cfg.ChatDBPath,
		"cache_dir":    cfg.CacheDir,
	} {
		if !strings.HasPrefix(path, "./.dev/") {
			t.Errorf("%s = %q, want state under ./.dev/", name, path)
		}
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "telegram.token is required") {
		t.Errorf("running the dev config without the devbot overlay must be refused, got %v", err)
	}

	if !strings.Contains(readRepoFile(t, "scripts/devbot/main.go"), `"configs/config.dev.yaml"`) {
		t.Error("the devbot runner must default to configs/config.dev.yaml")
	}
}
