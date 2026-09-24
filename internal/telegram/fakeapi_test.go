package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mymmrac/telego"
)

type fakeAPIRequest struct {
	Method string
	Body   map[string]any
}

type fakeTelegramAPI struct {
	server      *httptest.Server
	mu          sync.Mutex
	calls       []fakeAPIRequest
	updates     []string
	webhookInfo string
	infoFails   bool
}

func newFakeTelegramAPI(t *testing.T) *fakeTelegramAPI {
	t.Helper()

	api := &fakeTelegramAPI{}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)

	return api
}

func (api *fakeTelegramAPI) handle(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/__health" {
		writer.WriteHeader(http.StatusOK)
		return
	}

	method := request.URL.Path
	if idx := strings.LastIndexByte(method, '/'); idx >= 0 {
		method = method[idx+1:]
	}

	body := map[string]any{}
	if request.Body != nil {
		raw, _ := io.ReadAll(request.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
	}

	if method == "getUpdates" {
		api.mu.Lock()
		pending := api.updates
		api.updates = nil
		api.calls = append(api.calls, fakeAPIRequest{Method: method, Body: body})
		api.mu.Unlock()

		payload := "[" + strings.Join(pending, ",") + "]"
		writeFakeResult(writer, payload)
		return
	}

	api.mu.Lock()
	api.calls = append(api.calls, fakeAPIRequest{Method: method, Body: body})
	api.mu.Unlock()

	switch method {
	case "getWebhookInfo":
		api.mu.Lock()
		info, fails := api.webhookInfo, api.infoFails
		api.mu.Unlock()

		if fails {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprint(writer, `{"ok":false,"error_code":502,"description":"Bad Gateway"}`)
			return
		}
		if info == "" {
			info = `{"url":"","pending_update_count":0}`
		}
		writeFakeResult(writer, info)
	case "setWebhook", "deleteWebhook", "setMyCommands", "answerCallbackQuery", "sendChatAction":
		writeFakeResult(writer, "true")
	default:
		writeFakeResult(writer, `{"message_id":1,"chat":{"id":1,"type":"private"},"date":1758000000,"text":"ok"}`)
	}
}

func writeFakeResult(writer http.ResponseWriter, result string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(writer, `{"ok":true,"result":%s}`, result)
}

func (api *fakeTelegramAPI) setWebhookInfo(payload string) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.webhookInfo = payload
}

func (api *fakeTelegramAPI) failWebhookInfo(fail bool) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.infoFails = fail
}

func (api *fakeTelegramAPI) queueUpdate(update string) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.updates = append(api.updates, update)
}

func (api *fakeTelegramAPI) callsOf(method string) []fakeAPIRequest {
	api.mu.Lock()
	defer api.mu.Unlock()

	var found []fakeAPIRequest
	for _, call := range api.calls {
		if call.Method == method {
			found = append(found, call)
		}
	}
	return found
}

func (api *fakeTelegramAPI) waitFor(t *testing.T, method string, timeout time.Duration) fakeAPIRequest {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if calls := api.callsOf(method); len(calls) > 0 {
			return calls[len(calls)-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the bot never called %s over HTTP", method)
	return fakeAPIRequest{}
}

func (api *fakeTelegramAPI) waitForText(t *testing.T, method, needle string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, call := range api.callsOf(method) {
			if strings.Contains(fmt.Sprint(call.Body["text"]), needle) {
				return call.Body
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	api.mu.Lock()
	var seen []string
	for _, call := range api.calls {
		seen = append(seen, fmt.Sprintf("%s=%v", call.Method, call.Body["text"]))
	}
	api.mu.Unlock()

	t.Fatalf("no %s call carried %q, recorded: %v", method, needle, seen)
	return nil
}

func setupE2EBotWithFakeAPI(t *testing.T, api *fakeTelegramAPI, adminIDs ...int64) (*Bot, *Repository) {
	t.Helper()

	return setupE2EBotWithClient(t, func() (*telego.Bot, error) {
		return telego.NewBot(e2eBotToken, telego.WithAPIServer(api.server.URL), telego.WithDiscardLogger())
	}, adminIDs...)
}

func fakeUpdateJSON(t *testing.T, updateID, userID int64, text string) string {
	t.Helper()

	data, err := json.Marshal(map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id": 11,
			"date":       1758000000,
			"chat":       map[string]any{"id": userID, "type": "private"},
			"from":       map[string]any{"id": userID, "is_bot": false, "first_name": "Test"},
			"text":       text,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBotOverRealHTTPPollsAndReplies(t *testing.T) {
	const userID = int64(5150)

	api := newFakeTelegramAPI(t)
	b, repo := setupE2EBotWithFakeAPI(t, api, 999)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Group = "100"
	chat.Mode = ModeStudent
	repo.Save(chat)

	api.queueUpdate(fakeUpdateJSON(t, 1, userID, "/start"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	sent := api.waitForText(t, "sendMessage", "Математика", 5*time.Second)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("polling stopped with an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the bot did not stop after the context was cancelled")
	}

	if got := sent["chat_id"]; fmt.Sprint(got) != fmt.Sprint(userID) {
		t.Fatalf("the reply went to chat %v instead of %d", got, userID)
	}
	if api.callsOf("setWebhook") != nil {
		t.Error("polling mode must not register a webhook")
	}
}

func TestBotOverRealHTTPWebhookReplies(t *testing.T) {
	const userID = int64(5250)

	api := newFakeTelegramAPI(t)
	b, repo := setupE2EBotWithFakeAPI(t, api, 999)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Group = "100"
	chat.Mode = ModeStudent
	repo.Save(chat)

	b.cfg.Telegram.Webhook.Enabled = true
	b.cfg.Telegram.Webhook.URL = "https://mgke.example.com"
	b.cfg.Telegram.Webhook.SecretToken = "webhook-secret"

	settings, err := webhookSettingsFrom(b.cfg)
	if err != nil {
		t.Fatal(err)
	}
	params, closeCertificate, err := settings.setParams()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeCertificate() }()

	mux := http.NewServeMux()
	register := func(handler telego.WebhookHandler) error {
		mux.HandleFunc(settings.Path, webhookHTTPHandler(handler, settings.SecretToken))
		return nil
	}
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates, err := b.client.UpdatesViaWebhook(ctx, register,
		telego.WithWebhookSet(ctx, params),
	)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = b.consumeUpdates(ctx, updates) }()

	request, err := http.NewRequest(http.MethodPost, server.URL+settings.Path,
		strings.NewReader(fakeUpdateJSON(t, 2, userID, "/start")))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(telego.WebhookSecretTokenHeader, settings.SecretToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the webhook answered %d", response.StatusCode)
	}

	sent := api.waitForText(t, "sendMessage", "Математика", 5*time.Second)
	if got := sent["chat_id"]; fmt.Sprint(got) != fmt.Sprint(userID) {
		t.Fatalf("the reply went to chat %v instead of %d", got, userID)
	}

	registered := api.waitFor(t, "setWebhook", 5*time.Second)
	if registered.Body["url"] != "https://mgke.example.com/telegram/webhook" {
		t.Errorf("unexpected webhook url %v", registered.Body["url"])
	}
	if registered.Body["secret_token"] != settings.SecretToken {
		t.Errorf("unexpected secret token %v", registered.Body["secret_token"])
	}
}

func TestBotOverRealHTTPSurvivesTelegramErrors(t *testing.T) {
	const userID = int64(5350)

	failing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(writer, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
	}))
	defer failing.Close()

	b, repo := setupE2EBotWithClient(t, func() (*telego.Bot, error) {
		return telego.NewBot(e2eBotToken, telego.WithAPIServer(failing.URL), telego.WithDiscardLogger())
	}, 999)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Group = "100"
	repo.Save(chat)

	err = b.SendText(userID, "hello")
	if err == nil {
		t.Fatal("expected the rate limit to surface as an error")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(strings.ToLower(err.Error()), "many requests") {
		t.Errorf("unexpected error: %v", err)
	}
}
