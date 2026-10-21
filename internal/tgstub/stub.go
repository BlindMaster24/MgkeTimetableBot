package tgstub

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	callsPath   = "/_stub/calls"
	healthPath  = "/_stub/health"
	updatesPath = "/_stub/updates"

	DefaultChatID = int64(4242)
)

type Call struct {
	Method string         `json:"method"`
	Token  string         `json:"token"`
	Body   map[string]any `json:"body"`
	At     time.Time      `json:"at"`
}

type Stub struct {
	mu       sync.Mutex
	calls    []Call
	webhook  string
	updates  []map[string]any
	updateID int64
	wake     chan struct{}
}

func New() *Stub {
	return &Stub{wake: make(chan struct{}, 1)}
}

func (s *Stub) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()

	calls := make([]Call, len(s.calls))
	copy(calls, s.calls)

	return calls
}

func (s *Stub) WebhookURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.webhook
}

func (s *Stub) QueueUpdate(text string, chatID, userID int64) int64 {
	s.mu.Lock()
	s.updateID++
	id := s.updateID
	if chatID == 0 {
		chatID = DefaultChatID
	}
	if userID == 0 {
		userID = chatID
	}
	s.updates = append(s.updates, updatePayload(id, chatID, userID, text))
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}

	return id
}

func (s *Stub) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case healthPath:
		writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "calls": len(s.Calls()), "updates": s.queuedUpdates()})
		return
	case callsPath:
		writeJSON(writer, http.StatusOK, map[string]any{"calls": s.Calls()})
		return
	case updatesPath:
		s.serveQueuedUpdate(writer, request)
		return
	}

	token, method, ok := botMethod(request.URL.Path)
	if !ok {
		writeJSON(writer, http.StatusNotFound, map[string]any{"ok": false, "description": "unknown path"})
		return
	}

	body := map[string]any{}
	if request.Body != nil {
		raw, _ := io.ReadAll(request.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
	}

	s.mu.Lock()
	s.calls = append(s.calls, Call{Method: method, Token: token, Body: body, At: time.Now().UTC()})
	switch method {
	case "setWebhook":
		s.webhook, _ = body["url"].(string)
	case "deleteWebhook":
		s.webhook = ""
	}
	webhook := s.webhook
	s.mu.Unlock()

	if method == "getUpdates" {
		writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "result": s.awaitUpdates(request, body)})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "result": resultFor(method, body, webhook)})
}

func (s *Stub) serveQueuedUpdate(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{"ok": false, "description": "POST only"})
		return
	}

	var payload struct {
		Text   string `json:"text"`
		ChatID int64  `json:"chat_id"`
		UserID int64  `json:"user_id"`
	}
	if request.Body != nil {
		raw, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(raw, &payload)
	}
	if strings.TrimSpace(payload.Text) == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"ok": false, "description": "text is required"})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "update_id": s.QueueUpdate(payload.Text, payload.ChatID, payload.UserID)})
}

func (s *Stub) queuedUpdates() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.updates)
}

func (s *Stub) awaitUpdates(request *http.Request, body map[string]any) []map[string]any {
	if updates := s.drainUpdates(); len(updates) > 0 {
		return updates
	}

	timeout := updateTimeout(body)
	if timeout <= 0 {
		return []map[string]any{}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-request.Context().Done():
	case <-s.wake:
	}

	if updates := s.drainUpdates(); updates != nil {
		return updates
	}
	return []map[string]any{}
}

func (s *Stub) drainUpdates() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.updates) == 0 {
		return nil
	}
	updates := s.updates
	s.updates = nil

	return updates
}

func updateTimeout(body map[string]any) time.Duration {
	seconds, ok := body["timeout"].(float64)
	if !ok || seconds <= 0 {
		return 0
	}

	timeout := time.Duration(seconds * float64(time.Second))
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	return timeout
}

func updatePayload(id, chatID, userID int64, text string) map[string]any {
	return map[string]any{
		"update_id": id,
		"message": map[string]any{
			"message_id": id,
			"date":       time.Now().Unix(),
			"text":       text,
			"chat":       map[string]any{"id": chatID, "type": "private"},
			"from":       map[string]any{"id": userID, "is_bot": false, "first_name": "Stub"},
		},
	}
}

func botMethod(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "bot") {
		return "", "", false
	}

	token := strings.TrimPrefix(parts[0], "bot")
	if token == "" || parts[1] == "" {
		return "", "", false
	}

	return token, parts[1], true
}

func resultFor(method string, body map[string]any, webhook string) any {
	switch method {
	case "getMe":
		return map[string]any{"id": 123456789, "is_bot": true, "first_name": "Stub", "username": "ci_stub_bot"}
	case "getWebhookInfo":
		return map[string]any{"url": webhook, "pending_update_count": 0, "max_connections": 40}
	case "answerCallbackQuery":
		return true
	case "sendMessage", "editMessageText", "copyMessage", "sendPhoto", "sendDocument", "sendVideo":
		return messageResult(body)
	default:
		return true
	}
}

func messageResult(body map[string]any) map[string]any {
	chatID := body["chat_id"]
	if number, ok := chatID.(float64); ok {
		chatID = int64(number)
	}

	text := body["text"]
	if text == nil {
		text = ""
	}

	messageID := any(1)
	if number, ok := body["message_id"].(float64); ok {
		messageID = int64(number)
	}

	return map[string]any{
		"message_id": messageID,
		"date":       time.Now().Unix(),
		"chat":       map[string]any{"id": chatID, "type": "private"},
		"text":       text,
	}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
