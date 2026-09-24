package telegram

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

const fakeWebhookInfo = `{"url":"https://mgke.example.com/telegram/webhook","pending_update_count":7,"has_custom_certificate":false,"max_connections":12,"allowed_updates":["message","callback_query"],"last_error_date":1758000000,"last_error_message":"Wrong response from the webhook: 500 Internal Server Error"}`

func webhookBot(t *testing.T) (*Bot, *fakeTelegramAPI) {
	t.Helper()

	api := newFakeTelegramAPI(t)
	b, _ := setupE2EBotWithFakeAPI(t, api, 999)
	b.cfg.Telegram.Webhook.Enabled = true
	b.cfg.Telegram.Webhook.URL = "https://mgke.example.com"
	b.cfg.Telegram.Webhook.SecretToken = "webhook-secret"

	return b, api
}

func TestWebhookStatusReportsTheDeliveryState(t *testing.T) {
	b, api := webhookBot(t)
	api.setWebhookInfo(fakeWebhookInfo)

	status := b.WebhookStatus(context.Background())

	if !status.Enabled || status.Mode != webhookModeWebhook {
		t.Fatalf("unexpected mode: %+v", status)
	}
	if status.Endpoint != "https://mgke.example.com/telegram/webhook" {
		t.Errorf("endpoint = %q", status.Endpoint)
	}
	if status.Listen != webhookDefaultListen {
		t.Errorf("listen = %q", status.Listen)
	}
	if !status.SecretToken || status.Certificate {
		t.Errorf("secret/certificate flags = %v/%v", status.SecretToken, status.Certificate)
	}
	if status.PendingUpdates != 7 {
		t.Errorf("pending = %d", status.PendingUpdates)
	}
	if status.MaxConnections != 12 {
		t.Errorf("max connections = %d", status.MaxConnections)
	}
	if len(status.AllowedUpdates) != 2 {
		t.Errorf("allowed updates = %v", status.AllowedUpdates)
	}
	if !strings.Contains(status.LastError, "500") {
		t.Errorf("last error = %q", status.LastError)
	}
	if status.LastErrorAt != 1758000000 || status.CheckedAt == 0 {
		t.Errorf("timestamps = %d/%d", status.LastErrorAt, status.CheckedAt)
	}
	if status.InfoError != "" {
		t.Errorf("info error = %q", status.InfoError)
	}
}

func TestWebhookStatusKeepsTheLastKnownStateOnFailure(t *testing.T) {
	b, api := webhookBot(t)
	api.setWebhookInfo(fakeWebhookInfo)

	if status := b.WebhookStatus(context.Background()); status.PendingUpdates != 7 {
		t.Fatalf("the first read must come from Telegram, got %+v", status)
	}

	api.failWebhookInfo(true)

	status := b.WebhookStatus(context.Background())
	if status.InfoError == "" {
		t.Fatal("a failed webhook info request must be reported")
	}
	if status.PendingUpdates != 7 || status.LastError == "" {
		t.Errorf("the last known state was lost: %+v", status)
	}
	if status.CheckedAt == 0 {
		t.Error("the cached check time was lost")
	}
}

func TestWebhookStatusInLongPollingMode(t *testing.T) {
	api := newFakeTelegramAPI(t)
	b, _ := setupE2EBotWithFakeAPI(t, api, 999)

	status := b.WebhookStatus(context.Background())
	if status.Enabled || status.Mode != webhookModeLongPolling {
		t.Fatalf("unexpected mode: %+v", status)
	}
	if status.Endpoint != "" || status.SecretToken {
		t.Errorf("long polling must not report a webhook endpoint: %+v", status)
	}
}

func TestWebhookResetRejectsLongPollingMode(t *testing.T) {
	api := newFakeTelegramAPI(t)
	b, _ := setupE2EBotWithFakeAPI(t, api, 999)

	if _, err := b.ResetWebhook(context.Background()); err == nil {
		t.Fatal("resetting the webhook in long polling mode must fail")
	}
	if len(api.callsOf("setWebhook")) != 0 {
		t.Error("long polling mode must never call setWebhook")
	}
}

func TestWebhookCommandRendersTheStatusForAdmins(t *testing.T) {
	b, api := webhookBot(t)
	api.setWebhookInfo(fakeWebhookInfo)

	u := makeUpdate(999, "/webhook")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	sent := api.waitForText(t, "sendMessage", "-- Webhook --", 3*time.Second)
	reply := sent["text"].(string)

	for _, token := range []string{"https://mgke.example.com/telegram/webhook", "7", "Секрет: задан", "500"} {
		if !strings.Contains(reply, token) {
			t.Errorf("the status message misses %q:\n%s", token, reply)
		}
	}

	markup, ok := sent["reply_markup"].(map[string]any)
	if !ok {
		t.Fatalf("the status message carries no keyboard: %v", sent)
	}
	rows, ok := markup["inline_keyboard"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected keyboard %v", markup)
	}
	row, ok := rows[0].([]any)
	if !ok || len(row) != 1 {
		t.Fatalf("unexpected keyboard row %v", rows[0])
	}
	button, ok := row[0].(map[string]any)
	if !ok || button["callback_data"] != webhookResetCallback {
		t.Fatalf("unexpected reset button %v", row[0])
	}
	if button["text"] != b.loc("webhook_reset_button") {
		t.Errorf("unexpected button label %v", button["text"])
	}
}

func TestWebhookCommandDeniesNonAdmins(t *testing.T) {
	b, api := webhookBot(t)

	u := makeUpdate(4242, "/webhook")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	sent := api.waitForText(t, "sendMessage", "Команда не найдена", 3*time.Second)
	reply := sent["text"].(string)
	if strings.Contains(reply, "mgke.example.com") {
		t.Errorf("a non-admin must not see the webhook status:\n%s", reply)
	}
	if strings.Contains(fmt.Sprint(sent), webhookResetCallback) {
		t.Error("a non-admin must not receive the reset button")
	}
}

func TestWebhookResetCallbackRegistersTheWebhookAgain(t *testing.T) {
	b, api := webhookBot(t)
	api.setWebhookInfo(fakeWebhookInfo)

	b.handleCallback(context.Background(), callbackQuery(999, 12, webhookResetCallback))

	registered := api.waitFor(t, "setWebhook", 3*time.Second)
	if registered.Body["url"] != "https://mgke.example.com/telegram/webhook" {
		t.Errorf("unexpected setWebhook url %v", registered.Body["url"])
	}
	if registered.Body["secret_token"] != "webhook-secret" {
		t.Errorf("unexpected setWebhook secret %v", registered.Body["secret_token"])
	}

	reply := api.waitForText(t, "sendMessage", "Webhook переустановлен", 3*time.Second)
	if !strings.Contains(reply["text"].(string), "https://mgke.example.com/telegram/webhook") {
		t.Errorf("unexpected confirmation %q", reply["text"])
	}
}

func TestWebhookResetCallbackIsIgnoredForForeignPayloads(t *testing.T) {
	b, api := webhookBot(t)

	b.handleCallback(context.Background(), callbackQuery(999, 13, "webhook:unknown"))

	if len(api.callsOf("setWebhook")) != 0 {
		t.Error("an unknown webhook callback must not re-register the webhook")
	}
}
