package telegram

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/tgstub"
	"github.com/mymmrac/telego"
)

func newAPIServerBot(t *testing.T, cfg *config.Config) *Bot {
	t.Helper()

	chatRepo, err := New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chatRepo.Close() })

	raspCache, err := cache.New(t.TempDir() + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	bot, err := NewBot(cfg, logger.New("error", nil), i18n.New("ru"), chatRepo, raspCache, nil)
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}

	return bot
}

func TestNewBotRejectsABrokenAPIServer(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.Token = e2eBotToken
	cfg.Telegram.APIBaseURL = "api.example.test"

	chatRepo, err := New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer chatRepo.Close()

	raspCache, err := cache.New(t.TempDir() + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := NewBot(cfg, logger.New("error", nil), i18n.New("ru"), chatRepo, raspCache, nil); err == nil {
		t.Fatal("NewBot accepted telegram.api_base_url without a scheme")
	}
}

func TestNewBotTalksToTheConfiguredAPIServer(t *testing.T) {
	stub := tgstub.New()
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	cfg := &config.Config{}
	cfg.Telegram.Token = e2eBotToken
	cfg.Telegram.APIBaseURL = server.URL + "/"

	bot := newAPIServerBot(t, cfg)

	webhook := "https://bot.example.test/telegram/webhook"
	if err := bot.Client().SetWebhook(context.Background(), &telego.SetWebhookParams{URL: webhook, SecretToken: "s3cret"}); err != nil {
		t.Fatalf("call the configured api server: %v", err)
	}

	if stub.WebhookURL() != webhook {
		t.Fatal("the bot ignored telegram.api_base_url and called the official Bot API instead")
	}

	info, err := bot.Client().GetWebhookInfo(context.Background())
	if err != nil {
		t.Fatalf("getWebhookInfo: %v", err)
	}
	if info.URL != webhook {
		t.Errorf("getWebhookInfo url = %q, want %q", info.URL, webhook)
	}
}
