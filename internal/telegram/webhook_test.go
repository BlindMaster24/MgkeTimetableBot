package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

func webhookConfig(url, secret string) *config.Config {
	cfg := &config.Config{}
	cfg.Telegram.Webhook.Enabled = true
	cfg.Telegram.Webhook.URL = url
	cfg.Telegram.Webhook.SecretToken = secret
	return cfg
}

func TestWebhookSettingsRejectBrokenConfiguration(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "missing.pem")

	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{"no url", webhookConfig("", "secret")},
		{"http url", webhookConfig("http://mgke.example.com", "secret")},
		{"no host", webhookConfig("https://", "secret")},
		{"secret token with forbidden characters", webhookConfig("https://mgke.example.com", "bad secret!")},
		{"certificate without key", func() *config.Config {
			cfg := webhookConfig("https://mgke.example.com", "secret")
			cfg.Telegram.Webhook.Certificate = missingFile
			return cfg
		}()},
		{"missing certificate file", func() *config.Config {
			cfg := webhookConfig("https://mgke.example.com", "secret")
			cfg.Telegram.Webhook.Certificate = missingFile
			cfg.Telegram.Webhook.Key = missingFile
			return cfg
		}()},
		{"invalid ip address", func() *config.Config {
			cfg := webhookConfig("https://mgke.example.com", "secret")
			cfg.Telegram.Webhook.IPAddress = "not-an-ip"
			return cfg
		}()},
		{"too many connections", func() *config.Config {
			cfg := webhookConfig("https://mgke.example.com", "secret")
			cfg.Telegram.Webhook.MaxConnections = 101
			return cfg
		}()},
		{"path without slash", func() *config.Config {
			cfg := webhookConfig("https://mgke.example.com", "secret")
			cfg.Telegram.Webhook.Path = "telegram"
			return cfg
		}()},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := webhookSettingsFrom(testCase.cfg); err == nil {
				t.Fatal("expected the configuration to be rejected")
			}
		})
	}
}

func TestWebhookSettingsApplyDefaultsAndDeriveEndpoint(t *testing.T) {
	settings, err := webhookSettingsFrom(webhookConfig("https://mgke.example.com", "secret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Endpoint != "https://mgke.example.com/telegram/webhook" {
		t.Errorf("unexpected endpoint %q", settings.Endpoint)
	}
	if settings.Path != "/telegram/webhook" {
		t.Errorf("unexpected path %q", settings.Path)
	}
	if settings.Listen != webhookDefaultListen {
		t.Errorf("unexpected listen %q", settings.Listen)
	}
	if settings.Buffer != webhookDefaultBuffer {
		t.Errorf("unexpected buffer %d", settings.Buffer)
	}
	if settings.MaxConnections != webhookDefaultMaxConnections {
		t.Errorf("unexpected max connections %d", settings.MaxConnections)
	}
}

func TestWebhookSettingsKeepExplicitPathAndOptions(t *testing.T) {
	cfg := webhookConfig("https://mgke.example.com/tg/hook", "secret")
	cfg.Telegram.Webhook.Listen = "127.0.0.1:9000"
	cfg.Telegram.Webhook.Buffer = 8
	cfg.Telegram.Webhook.MaxConnections = 100
	cfg.Telegram.Webhook.DropPendingUpdates = true
	cfg.Telegram.Webhook.IPAddress = "10.0.0.1"
	cfg.Telegram.Webhook.AllowedUpdates = []string{"message", "callback_query"}

	settings, err := webhookSettingsFrom(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Endpoint != "https://mgke.example.com/tg/hook" {
		t.Errorf("unexpected endpoint %q", settings.Endpoint)
	}
	if settings.Path != "/tg/hook" {
		t.Errorf("explicit path was not kept: %q", settings.Path)
	}
	if settings.Listen != "127.0.0.1:9000" || settings.Buffer != 8 || settings.MaxConnections != 100 {
		t.Errorf("explicit options were not kept: %+v", settings)
	}
	if !settings.DropPendingUpdates {
		t.Error("drop_pending_updates was not kept")
	}
}

func TestWebhookSetParamsCarryTheCertificate(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyPath, []byte("-----BEGIN PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := webhookConfig("https://mgke.example.com", "secret")
	cfg.Telegram.Webhook.Certificate = certPath
	cfg.Telegram.Webhook.Key = keyPath
	cfg.Telegram.Webhook.AllowedUpdates = []string{"message"}
	cfg.Telegram.Webhook.DropPendingUpdates = true

	settings, err := webhookSettingsFrom(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params, closeCertificate, err := settings.setParams()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = closeCertificate() }()

	if params.URL != "https://mgke.example.com/telegram/webhook" {
		t.Errorf("unexpected url %q", params.URL)
	}
	if params.SecretToken != "secret" {
		t.Errorf("unexpected secret token %q", params.SecretToken)
	}
	if params.MaxConnections != webhookDefaultMaxConnections {
		t.Errorf("unexpected max connections %d", params.MaxConnections)
	}
	if !params.DropPendingUpdates || len(params.AllowedUpdates) != 1 {
		t.Errorf("unexpected update filters: %+v", params)
	}
	if params.Certificate == nil {
		t.Fatal("certificate was not attached")
	}
	if params.Certificate.File.Name() != certPath {
		t.Errorf("unexpected certificate %q", params.Certificate.File.Name())
	}
}

func TestWebhookHandlerAcceptsOnlySignedRequests(t *testing.T) {
	const secret = "top-secret"
	body := []byte(`{"update_id":1}`)

	cases := []struct {
		name       string
		method     string
		secret     string
		wantStatus int
		wantCalled bool
	}{
		{"signed post", http.MethodPost, secret, http.StatusOK, true},
		{"foreign secret", http.MethodPost, "other", http.StatusUnauthorized, false},
		{"missing secret", http.MethodPost, "", http.StatusUnauthorized, false},
		{"wrong method", http.MethodGet, secret, http.StatusMethodNotAllowed, false},
		{"put", http.MethodPut, secret, http.StatusMethodNotAllowed, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			called := false
			var received []byte
			handler := webhookHTTPHandler(func(_ context.Context, data []byte) error {
				called = true
				received = data
				return nil
			}, secret)

			request := httptest.NewRequest(testCase.method, "/telegram/webhook", bytes.NewReader(body))
			if testCase.secret != "" {
				request.Header.Set(telego.WebhookSecretTokenHeader, testCase.secret)
			}
			recorder := httptest.NewRecorder()
			handler(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("unexpected status %d", recorder.Code)
			}
			if called != testCase.wantCalled {
				t.Fatalf("handler called = %v", called)
			}
			if testCase.wantCalled && string(received) != string(body) {
				t.Errorf("unexpected payload %q", received)
			}
		})
	}
}

func TestWebhookHandlerRejectsOversizedBody(t *testing.T) {
	handler := webhookHTTPHandler(func(_ context.Context, _ []byte) error {
		return nil
	}, "")

	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook",
		bytes.NewReader(bytes.Repeat([]byte("a"), webhookMaxBodyBytes+1)))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected status %d", recorder.Code)
	}
}

func TestWebhookHandlerAsksTelegramToRetryOnFailure(t *testing.T) {
	handler := webhookHTTPHandler(func(_ context.Context, _ []byte) error {
		return context.DeadlineExceeded
	}, "")

	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", bytes.NewReader([]byte(`{}`)))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status %d", recorder.Code)
	}
}

func TestWebhookHandlerKeepsTheUpdateContextAlive(t *testing.T) {
	var (
		mu         sync.Mutex
		handlerCtx context.Context
	)
	handler := webhookHTTPHandler(func(ctx context.Context, _ []byte) error {
		mu.Lock()
		handlerCtx = ctx
		mu.Unlock()
		return nil
	}, "")

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", bytes.NewReader([]byte(`{}`))).WithContext(requestCtx)
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	cancelRequest()

	mu.Lock()
	defer mu.Unlock()
	if handlerCtx == nil {
		t.Fatal("handler did not receive a context")
	}
	if err := handlerCtx.Err(); err != nil {
		t.Fatalf("update context died with the request: %v", err)
	}
}

func TestWebhookSettingsComeFromTheEnvironment(t *testing.T) {
	env := map[string]string{
		"MGKE_TELEGRAM_WEBHOOK_ENABLED":              "true",
		"MGKE_TELEGRAM_WEBHOOK_URL":                  "https://mgke.example.com/hook",
		"MGKE_TELEGRAM_WEBHOOK_SECRET_TOKEN":         "env-secret",
		"MGKE_TELEGRAM_WEBHOOK_LISTEN":               "127.0.0.1:9100",
		"MGKE_TELEGRAM_WEBHOOK_MAX_CONNECTIONS":      "7",
		"MGKE_TELEGRAM_WEBHOOK_DROP_PENDING_UPDATES": "true",
		"MGKE_TELEGRAM_WEBHOOK_ALLOWED_UPDATES":      "message,callback_query",
	}

	cfg := &config.Config{}
	if err := config.ApplyEnv(cfg, func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}); err != nil {
		t.Fatalf("apply env: %v", err)
	}

	settings, err := webhookSettingsFrom(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Endpoint != "https://mgke.example.com/hook" || settings.Path != "/hook" {
		t.Errorf("unexpected endpoint %q / path %q", settings.Endpoint, settings.Path)
	}
	if settings.SecretToken != "env-secret" || settings.Listen != "127.0.0.1:9100" {
		t.Errorf("environment values were not applied: %+v", settings)
	}
	if settings.MaxConnections != 7 || !settings.DropPendingUpdates {
		t.Errorf("environment options were not applied: %+v", settings)
	}
	if len(settings.AllowedUpdates) != 2 {
		t.Errorf("allowed updates were not applied: %+v", settings.AllowedUpdates)
	}
}

func FuzzWebhookHandlerKeepsStatusesSane(f *testing.F) {
	f.Add("secret", "secret", `{"update_id":1}`)
	f.Add("", "secret", `{}`)
	f.Add("other", "secret", `not json`)
	f.Add("secret", "", ``)

	f.Fuzz(func(t *testing.T, header, secret, body string) {
		handler := webhookHTTPHandler(func(_ context.Context, data []byte) error {
			var update telego.Update
			if err := json.Unmarshal(data, &update); err != nil {
				return err
			}
			return nil
		}, secret)

		request := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
		if header != "" {
			request.Header.Set(telego.WebhookSecretTokenHeader, header)
		}
		recorder := httptest.NewRecorder()
		handler(recorder, request)

		switch recorder.Code {
		case http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError, http.StatusRequestEntityTooLarge:
		default:
			t.Fatalf("unexpected status %d for header %q", recorder.Code, header)
		}

		if secret != "" && header != secret && recorder.Code != http.StatusUnauthorized {
			t.Fatalf("a foreign secret %q was accepted with status %d", header, recorder.Code)
		}
	})
}

func webhookUpdateJSON(t *testing.T, userID int64, text string) []byte {
	t.Helper()

	payload := map[string]any{
		"update_id": 100,
		"message": map[string]any{
			"message_id": 7,
			"date":       1758000000,
			"chat":       map[string]any{"id": userID, "type": "private"},
			"from":       map[string]any{"id": userID, "is_bot": false, "first_name": "Test"},
			"text":       text,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestWebhookDeliversUpdatesToTheBot(t *testing.T) {
	const userID = int64(4242)

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 999)
	b.cfg.Telegram.Webhook.Enabled = true
	b.cfg.Telegram.Webhook.URL = "https://mgke.example.com/telegram"
	b.cfg.Telegram.Webhook.SecretToken = "webhook-secret"

	settings, err := webhookSettingsFrom(b.cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	params, closeCertificate, err := settings.setParams()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = closeCertificate() }()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	register := func(handler telego.WebhookHandler) error {
		mux.HandleFunc(settings.Path, webhookHTTPHandler(handler, settings.SecretToken))
		return nil
	}

	updates, err := b.client.UpdatesViaWebhook(ctx, register,
		telego.WithWebhookBuffer(4),
		telego.WithWebhookSet(ctx, params),
	)
	if err != nil {
		t.Fatalf("start webhook: %v", err)
	}
	go func() { _ = b.consumeUpdates(ctx, updates) }()

	post := func(secret string) *http.Response {
		t.Helper()

		request, err := http.NewRequest(http.MethodPost, server.URL+settings.Path,
			bytes.NewReader(webhookUpdateJSON(t, userID, "/start")))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		if secret != "" {
			request.Header.Set(telego.WebhookSecretTokenHeader, secret)
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("post update: %v", err)
		}
		return response
	}

	response := post(settings.SecretToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", response.StatusCode)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if caller.calledMethod("sendMessage") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !caller.calledMethod("sendMessage") {
		t.Fatal("the update from the webhook never reached the bot")
	}
	if !caller.calledMethod("setWebhook") {
		t.Fatal("setWebhook was never called")
	}

	var registered string
	for _, call := range caller.calls {
		if call.Method == "setWebhook" {
			registered = call.Raw
		}
	}
	for _, token := range []string{"mgke.example.com/telegram", "webhook-secret"} {
		if !strings.Contains(registered, token) {
			t.Errorf("setWebhook payload misses %q: %s", token, registered)
		}
	}

	unauthorized := post("other-secret")
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status for the unsigned update %d", unauthorized.StatusCode)
	}
}

type webhookRunCaller struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (c *webhookRunCaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	name := url
	if idx := strings.LastIndexByte(url, '/'); idx >= 0 {
		name = url[idx+1:]
	}

	c.mu.Lock()
	c.calls = append(c.calls, recordedCall{Method: name, Raw: string(data.BodyRaw)})
	c.mu.Unlock()

	switch name {
	case "getUpdates":
		<-ctx.Done()
		return nil, ctx.Err()
	case "getWebhookInfo":
		return &telegoapi.Response{Ok: true, Result: []byte(`{"url":"https://mgke.example.com/telegram/webhook","pending_update_count":3,"allowed_updates":["message"]}`)}, nil
	default:
		return &telegoapi.Response{Ok: true, Result: []byte(`true`)}, nil
	}
}

func (c *webhookRunCaller) called(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, call := range c.calls {
		if call.Method == name {
			return true
		}
	}
	return false
}

func TestRunStartsTheWebhookWhenEnabled(t *testing.T) {
	caller := &webhookRunCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 999)
	b.cfg.Telegram.Webhook.Enabled = true
	b.cfg.Telegram.Webhook.URL = "https://mgke.example.com"
	b.cfg.Telegram.Webhook.Listen = "127.0.0.1:0"
	b.cfg.Telegram.Webhook.SecretToken = "webhook-secret"
	b.cfg.Telegram.Webhook.AllowedUpdates = []string{"message", "callback_query"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !caller.called("setWebhook") {
		time.Sleep(10 * time.Millisecond)
	}
	if !caller.called("setWebhook") {
		t.Fatal("the webhook was never registered")
	}
	if !caller.called("getWebhookInfo") {
		t.Error("the webhook status was never checked")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the bot did not stop after the context was cancelled")
	}
}

func TestRunRefusesToStartTheWebhookWithoutAURL(t *testing.T) {
	b, _ := setupE2EBot(t, 999)
	b.cfg.Telegram.Webhook.Enabled = true

	err := b.Run(context.Background())
	if err == nil {
		t.Fatal("expected the webhook to be rejected")
	}
	if !strings.Contains(err.Error(), "telegram.webhook.url") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDropsAStaleWebhookBeforePolling(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 999)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !caller.calledMethod("deleteWebhook") {
		time.Sleep(10 * time.Millisecond)
	}

	methods := caller.methods()
	if len(methods) == 0 || methods[0] != "deleteWebhook" {
		t.Fatalf("deleteWebhook was not called first: %v", methods)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the bot did not stop after the context was cancelled")
	}
}
