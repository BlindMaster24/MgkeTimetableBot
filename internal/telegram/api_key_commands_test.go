package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/mymmrac/telego"
)

const apiKeyTestSecret = "0123456789abcdef0123456789abcdef"

var printedKeyExpr = regexp.MustCompile(`<code>([^<]+)</code>`)

func apiKeyStore(t *testing.T, repo *Repository, secret string) *apikey.Store {
	t.Helper()

	store := apikey.NewStore(repo.DB(), secret)
	if err := store.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}

func telegramMessage(userID int64, chatType, text string) *telego.Message {
	return &telego.Message{
		MessageID: 1,
		From:      &telego.User{ID: userID},
		Chat:      telego.Chat{ID: userID, Type: chatType},
		Text:      text,
	}
}

func sendMessage(t *testing.T, b *Bot, userID int64, chatType, text string) {
	t.Helper()

	b.handleMessage(context.Background(), telegramMessage(userID, chatType, text))
}

func printedKey(t *testing.T, text string) string {
	t.Helper()

	match := printedKeyExpr.FindStringSubmatch(text)
	if match == nil {
		t.Fatalf("no key in %q", text)
	}
	return match[1]
}

func createdKey(t *testing.T, text string) string {
	t.Helper()

	for _, line := range strings.Split(text, "\n") {
		if value, ok := strings.CutPrefix(line, "Ключ: "); ok {
			return value
		}
	}
	t.Fatalf("no key in %q", text)
	return ""
}

func setupApiKeyBot(t *testing.T, adminIDs ...int64) (*Bot, *Repository, *apikey.Store, *capturingCaller) {
	t.Helper()

	b, repo := setupE2EBot(t, adminIDs...)
	repo.SetDefaultAccepted(true)

	store := apiKeyStore(t, repo, apiKeyTestSecret)
	b.SetKeyStore(store)

	caller := &capturingCaller{}
	b.client = botWithCaller(t, caller)
	return b, repo, store, caller
}

func TestApiCommandPrintsTheChatKey(t *testing.T) {
	b, repo, store, caller := setupApiKeyBot(t)

	sendMessage(t, b, 5001, "private", "/api")
	text := caller.last()
	if !strings.Contains(text, "API токен #") {
		t.Fatalf("unexpected answer: %q", text)
	}
	if !strings.Contains(text, "Запросов в сек: 2") {
		t.Fatalf("the default limit must be shown: %q", text)
	}
	if !strings.Contains(text, "Последнее использование: нет") {
		t.Fatalf("a fresh key has never been used: %q", text)
	}

	key, err := store.ByToken(printedKey(t, text))
	if err != nil {
		t.Fatalf("the printed key must authenticate: %v", err)
	}

	chat, err := repo.FindOrCreate("telegram", 5001)
	if err != nil {
		t.Fatal(err)
	}
	if key.ChatID != chat.ID {
		t.Fatalf("the key belongs to chat %d, want %d", key.ChatID, chat.ID)
	}

	used := time.Date(2026, 9, 20, 10, 30, 0, 0, time.Local)
	if err := store.Touch(key.ID, used); err != nil {
		t.Fatal(err)
	}

	caller.reset()
	sendMessage(t, b, 5001, "private", "/api")
	if !strings.Contains(caller.last(), "Последнее использование: 20.09.2026 10:30") {
		t.Fatalf("the usage time must be shown: %q", caller.last())
	}
}

func TestApiNewRotatesTheKey(t *testing.T) {
	b, _, store, caller := setupApiKeyBot(t)

	sendMessage(t, b, 5002, "private", "/api")
	first := printedKey(t, caller.last())

	caller.reset()
	sendMessage(t, b, 5002, "private", "/api_new")
	text := caller.last()
	if !strings.Contains(text, "Новый API токен #") {
		t.Fatalf("the rotated key must say so: %q", text)
	}
	second := printedKey(t, text)
	if first == second {
		t.Fatal("the rotation must change the key")
	}

	if _, err := store.ByToken(first); !errors.Is(err, apikey.ErrInvalidKey) {
		t.Fatalf("the old key must stop working, got %v", err)
	}
	if _, err := store.ByToken(second); err != nil {
		t.Fatalf("the new key must work, got %v", err)
	}
}

func TestApiCommandIsClosedInGroupChats(t *testing.T) {
	b, _, _, caller := setupApiKeyBot(t)

	sendMessage(t, b, 5003, "supergroup", "/api")
	if caller.last() != "Команда недоступна в беседах" {
		t.Fatalf("unexpected answer: %q", caller.last())
	}
}

func TestApiCommandReportsDisabledKeys(t *testing.T) {
	b, repo, _, caller := setupApiKeyBot(t)

	b.SetKeyStore(apiKeyStore(t, repo, ""))
	sendMessage(t, b, 5004, "private", "/api")
	if !strings.Contains(caller.last(), "API-ключи недоступны") {
		t.Fatalf("a short secret must be reported: %q", caller.last())
	}

	caller.reset()
	b.SetKeyStore(nil)
	sendMessage(t, b, 5004, "private", "/api")
	if !strings.Contains(caller.last(), "API-ключи недоступны") {
		t.Fatalf("a missing store must be reported: %q", caller.last())
	}
}

func TestApiKeyFromTheBotOpensTheRestApi(t *testing.T) {
	b, _, store, caller := setupApiKeyBot(t)

	sendMessage(t, b, 5005, "private", "/api")
	token := printedKey(t, caller.last())

	srv := api.NewServer(b.cache, 0, nil, build.New("test", "abcdef1234567890", "2026-01-02T03:04:05Z"), store, i18n.New("ru"))

	key, err := store.ByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetLimit(key.ChatID, 1); err != nil {
		t.Fatal(err)
	}

	if got := apiCall(t, srv, token); got != http.StatusOK {
		t.Fatalf("the key printed by the bot must open the API, got %d", got)
	}
	if got := apiCall(t, srv, token); got != http.StatusTooManyRequests {
		t.Fatalf("the second request of the window must be limited, got %d", got)
	}

	if err := store.SetLimit(key.ChatID, 0); err != nil {
		t.Fatal(err)
	}

	caller.reset()
	sendMessage(t, b, 5005, "private", "/api_new")
	rotated := printedKey(t, caller.last())

	if got := apiCall(t, srv, token); got != http.StatusUnauthorized {
		t.Fatalf("the old key must be rejected after the rotation, got %d", got)
	}
	if got := apiCall(t, srv, rotated); got != http.StatusOK {
		t.Fatalf("the rotated key must work, got %d", got)
	}
}

func apiCall(t *testing.T, srv *api.Server, token string) int {
	t.Helper()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/groups", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Handler().ServeHTTP(w, req)
	return w.Code
}

func TestCreateApiKeyIssuesAndRotates(t *testing.T) {
	const admin = int64(7001)
	b, _, store, caller := setupApiKeyBot(t, admin)

	sendMessage(t, b, admin, "private", "/createApiKey 4242 5")
	text := caller.last()
	for _, want := range []string{"Апи токен создан (добавлен)", "ID: ", "Лимит: 5", "IV: "} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}

	key, err := store.ByChatID(4242)
	if err != nil {
		t.Fatal(err)
	}
	if key.LimitPerSec != 5 {
		t.Fatalf("limit = %d, want 5", key.LimitPerSec)
	}
	if createdKey(t, text) != mustToken(t, store, key) {
		t.Fatal("the printed key must be the stored one")
	}

	caller.reset()
	sendMessage(t, b, admin, "private", "/createApiKey 4242")
	text = caller.last()
	if !strings.Contains(text, "Апи токен создан (обновлён)") {
		t.Fatalf("a repeated call must report the update: %q", text)
	}

	updated, err := store.ByChatID(4242)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LimitPerSec != 5 {
		t.Fatalf("the limit must survive the rotation, got %d", updated.LimitPerSec)
	}
	if mustToken(t, store, updated) == mustToken(t, store, key) {
		t.Fatal("the repeated call must rotate the key")
	}
}

func TestCreateApiKeyAcceptsTheOldCommandSpellings(t *testing.T) {
	const admin = int64(7002)
	b, _, store, caller := setupApiKeyBot(t, admin)

	for _, text := range []string{"/createApiToken 4243 3", "!createApiKey 4244"} {
		caller.reset()
		sendMessage(t, b, admin, "private", text)
		if !strings.Contains(caller.last(), "Апи токен создан (добавлен)") {
			t.Fatalf("%s: unexpected answer %q", text, caller.last())
		}
	}

	if key, err := store.ByChatID(4243); err != nil || key.LimitPerSec != 3 {
		t.Fatalf("the token spelling must create the key: %+v (%v)", key, err)
	}
	if _, err := store.ByChatID(4244); err != nil {
		t.Fatalf("the shout spelling must create the key: %v", err)
	}
}

func TestCreateApiKeyShowsTheUsage(t *testing.T) {
	const admin = int64(7003)
	b, _, _, caller := setupApiKeyBot(t, admin)

	usage := "Создать апи токен:\n/createApiKey <chatId> [limit]"
	for _, text := range []string{"/createApiKey", "/createApiKey abc", "/createApiKey 1 2 3", "/createApiKey 1 -5"} {
		caller.reset()
		sendMessage(t, b, admin, "private", text)
		if caller.last() != usage {
			t.Fatalf("%s: got %q, want %q", text, caller.last(), usage)
		}
	}
}

func TestCreateApiKeyIsAdminOnly(t *testing.T) {
	const admin = int64(7004)
	b, _, store, caller := setupApiKeyBot(t, admin)

	sendMessage(t, b, 8005, "private", "/createApiKey 4245")
	if caller.last() != "Команда не найдена" {
		t.Fatalf("unexpected answer: %q", caller.last())
	}
	if _, err := store.ByChatID(4245); !errors.Is(err, apikey.ErrKeyNotFound) {
		t.Fatalf("a stranger must not create keys, got %v", err)
	}
}

func TestKeysSurviveANewStoreOverTheSameDatabase(t *testing.T) {
	_, repo, store, _ := setupApiKeyBot(t)

	before, _, err := store.FindOrCreate(9001)
	if err != nil {
		t.Fatal(err)
	}
	reopened := apiKeyStore(t, repo, apiKeyTestSecret)

	after, err := reopened.ByChatID(9001)
	if err != nil {
		t.Fatal(err)
	}
	if before.ID != after.ID {
		t.Fatalf("keys must survive a new store over the same file: %d and %d", before.ID, after.ID)
	}
}

func mustToken(t *testing.T, store *apikey.Store, key *apikey.Key) string {
	t.Helper()

	token, err := store.Token(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
