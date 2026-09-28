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
	callsPath  = "/_stub/calls"
	healthPath = "/_stub/health"
)

type Call struct {
	Method string         `json:"method"`
	Token  string         `json:"token"`
	Body   map[string]any `json:"body"`
	At     time.Time      `json:"at"`
}

type Stub struct {
	mu      sync.Mutex
	calls   []Call
	webhook string
}

func New() *Stub {
	return &Stub{}
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

func (s *Stub) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case healthPath:
		writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "calls": len(s.Calls())})
		return
	case callsPath:
		writeJSON(writer, http.StatusOK, map[string]any{"calls": s.Calls()})
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

	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "result": resultFor(method, body, webhook)})
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
	case "getUpdates":
		return []any{}
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

	return map[string]any{
		"message_id": 1,
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
