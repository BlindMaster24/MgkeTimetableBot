package app

import (
	"context"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

func TestWebhookMetricsCarryFlagsAndCountsOnly(t *testing.T) {
	status := telegrambot.WebhookStatus{
		Enabled:        true,
		Mode:           "webhook",
		Endpoint:       "https://bot.example.com/telegram/webhook",
		URL:            "https://bot.example.com/telegram/webhook",
		Listen:         "127.0.0.1:8082",
		Path:           "/telegram/webhook",
		SecretToken:    true,
		Certificate:    true,
		AllowedUpdates: []string{"message", "callback_query"},
		PendingUpdates: 9,
		LastError:      "wrong response from the webhook: 401 Unauthorized",
		LastErrorAt:    1735689600,
	}

	got := webhookMetrics(status)

	want := api.WebhookMetrics{
		Enabled:    true,
		Mode:       "webhook",
		Registered: true,
		Secret:     true,
		Reachable:  true,
		Pending:    9,
	}
	if got != want {
		t.Errorf("webhook metrics = %+v, want %+v", got, want)
	}
}

func TestWebhookMetricsStayEmptyWhileTelegramIsUnreachable(t *testing.T) {
	status := telegrambot.WebhookStatus{
		Enabled:   true,
		Mode:      "webhook",
		Endpoint:  "https://bot.example.com/telegram/webhook",
		InfoError: "getWebhookInfo: context deadline exceeded",
	}

	got := webhookMetrics(status)

	if got.Registered || got.Reachable {
		t.Errorf("an unreachable Telegram must not look registered: %+v", got)
	}
	if !got.Enabled || got.Secret {
		t.Errorf("the configured flags survive the failed check: %+v", got)
	}
	if got.Mode != "webhook" {
		t.Errorf("mode = %q", got.Mode)
	}
}

func TestWebhookMetricsSourceThrottlesTheLiveCheck(t *testing.T) {
	fetches := 0
	source := newWebhookMetricsSource(func(context.Context) telegrambot.WebhookStatus {
		fetches++
		return telegrambot.WebhookStatus{Enabled: true, Mode: "webhook", SecretToken: true, PendingUpdates: 4}
	})

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	source.now = func() time.Time { return now }

	first := source.Get(context.Background())
	second := source.Get(context.Background())
	if fetches != 1 {
		t.Errorf("a scrape must not call Telegram again inside the interval, fetches = %d", fetches)
	}
	if first != second || second.Pending != 4 {
		t.Errorf("the cached snapshot = %+v / %+v", first, second)
	}

	now = now.Add(webhookMetricsInterval - time.Second)
	source.Get(context.Background())
	if fetches != 1 {
		t.Errorf("the interval has not elapsed, fetches = %d", fetches)
	}

	now = now.Add(2 * time.Second)
	third := source.Get(context.Background())
	if fetches != 2 {
		t.Errorf("the interval elapsed, fetches = %d", fetches)
	}
	if !third.Secret || !third.Enabled {
		t.Errorf("the refreshed snapshot = %+v", third)
	}
}

func TestWebhookMetricsSourceServesTheLastSnapshotWhileTelegramIsDown(t *testing.T) {
	calls := 0
	source := newWebhookMetricsSource(func(context.Context) telegrambot.WebhookStatus {
		calls++
		if calls == 1 {
			return telegrambot.WebhookStatus{Enabled: true, Mode: "webhook", URL: "https://bot.example.com/telegram/webhook", SecretToken: true, PendingUpdates: 6}
		}
		return telegrambot.WebhookStatus{Enabled: true, Mode: "webhook", SecretToken: true, InfoError: "getWebhookInfo: context deadline exceeded"}
	})

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	source.now = func() time.Time { return now }

	if got := source.Get(context.Background()); !got.Reachable || got.Pending != 6 {
		t.Fatalf("first snapshot = %+v", got)
	}

	now = now.Add(time.Minute)
	got := source.Get(context.Background())
	if got.Reachable || got.Registered {
		t.Errorf("the failed check must be visible: %+v", got)
	}
	if !got.Secret || !got.Enabled || got.Mode != "webhook" {
		t.Errorf("the configured flags must survive the outage: %+v", got)
	}
}

func TestWebhookMetricsKeepLongPollingHonest(t *testing.T) {
	idle := webhookMetrics(telegrambot.WebhookStatus{Mode: "long_polling"})
	if idle.Enabled || idle.Registered || idle.Pending != 0 {
		t.Errorf("a long-polling bot without a registration = %+v", idle)
	}

	stale := webhookMetrics(telegrambot.WebhookStatus{
		Mode: "long_polling",
		URL:  "https://bot.example.com/telegram/webhook",
	})
	if stale.Enabled || !stale.Registered {
		t.Errorf("a stale registration must stay visible without claiming webhook mode: %+v", stale)
	}
}
