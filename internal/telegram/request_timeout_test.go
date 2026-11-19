package telegram

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

// slowServer answers far too late, so only the client timeout can end a call.
func slowServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
}

func TestBotAPICallsAreBoundedByTheRequestTimeout(t *testing.T) {
	server := slowServer(t)
	defer server.Close()

	previous := requestTimeout
	requestTimeout = 100 * time.Millisecond
	t.Cleanup(func() { requestTimeout = previous })

	cfg := &config.Config{}
	cfg.Telegram.Token = "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	cfg.Telegram.APIBaseURL = server.URL

	chatRepo, err := New(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chatRepo.Close() })

	bot, err := NewBot(cfg, logger.New("error", nil), i18n.New("ru"), chatRepo, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	err = bot.SendText(1, "привет")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("a call without an answer must fail")
	}
	// The server needs two seconds, so anything under a second proves the
	// client cut the call; the slack keeps the assertion honest on a loaded
	// -race runner.
	if elapsed >= time.Second {
		t.Fatalf("the call took %v, the client timeout must bound it", elapsed)
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "timeout") && !strings.Contains(message, "deadline") {
		t.Fatalf("the error must mention the timeout, got %v", err)
	}
}
