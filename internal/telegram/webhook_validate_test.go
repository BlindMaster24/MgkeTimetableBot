package telegram

import (
	"reflect"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func testWebhookConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Telegram.Webhook.URL = "https://bot.example.test/telegram/webhook"
	cfg.Telegram.Webhook.SecretToken = "s3cret-token_1"
	return cfg
}

func TestValidateWebhookAcceptsAUsableConfig(t *testing.T) {
	if err := ValidateWebhook(testWebhookConfig()); err != nil {
		t.Fatalf("a valid webhook config was rejected: %v", err)
	}
}

func TestValidateWebhookRejectsWhatTelegramCannotUse(t *testing.T) {
	cases := []struct {
		name   string
		change func(*config.Config)
		want   string
	}{
		{"bad token characters", func(c *config.Config) { c.Telegram.Webhook.SecretToken = "not allowed" }, "secret_token"},
		{"certificate without key", func(c *config.Config) { c.Telegram.Webhook.Certificate = "cert.pem" }, "must be set together"},
		{"key without certificate", func(c *config.Config) { c.Telegram.Webhook.Key = "key.pem" }, "must be set together"},
		{"bad ip", func(c *config.Config) { c.Telegram.Webhook.IPAddress = "not-an-ip" }, "ip_address"},
		{"bad path", func(c *config.Config) { c.Telegram.Webhook.Path = "telegram" }, "start with /"},
		{"too many connections", func(c *config.Config) { c.Telegram.Webhook.MaxConnections = 101 }, "max_connections"},
	}
	for _, c := range cases {
		cfg := testWebhookConfig()
		c.change(cfg)
		err := ValidateWebhook(cfg)
		if err == nil {
			t.Errorf("%s: the config was accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %q, want it to mention %q", c.name, err, c.want)
		}
	}
}

func TestSurfaceOfReportsTheSameParts(t *testing.T) {
	surface := SurfaceOf()
	if !reflect.DeepEqual(surface.Commands, SurfaceOfCommands()) {
		t.Fatal("SurfaceOf commands diverged from SurfaceOfCommands")
	}
	if !reflect.DeepEqual(surface.Callbacks, SurfaceOfCallbacks()) {
		t.Fatal("SurfaceOf callbacks diverged from SurfaceOfCallbacks")
	}
	if len(surface.Commands) == 0 || len(surface.Callbacks) == 0 {
		t.Fatalf("the surface is empty: %d commands, %d callbacks", len(surface.Commands), len(surface.Callbacks))
	}
}
