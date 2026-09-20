package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	_ "modernc.org/sqlite"
)

func keyStoreWithSecret(t *testing.T, secret string) *apikey.Store {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	store := apikey.NewStore(db, secret)
	if err := store.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}

func setupServerWithSecret(t *testing.T, secret string) *Server {
	t.Helper()

	srv := setupTestServerWith(t, health.NewDefaultTracker())
	srv.keys = keyStoreWithSecret(t, secret)
	return srv
}

func requestWithHeaders(t *testing.T, srv *Server, path string, headers []string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range headers {
		req.Header.Add("Authorization", header)
	}
	srv.Handler().ServeHTTP(w, req)
	return w
}

func TestAStoreWithoutASecretClosesEveryProtectedRoute(t *testing.T) {
	srv := setupServerWithSecret(t, "too-short-to-sign-anything")

	for _, path := range []string{"/api/info", "/api/groups", "/api/teachers", "/api/group/63", "/api/teacher/Иванов", "/api/parser-health"} {
		w := doRequest(t, srv, path, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, w.Code)
		}
		if got := errorBody(t, w); got != "Неверный ключ авторизации" {
			t.Fatalf("%s: unexpected error %q", path, got)
		}
	}

	if w := doRequest(t, srv, healthRoute, ""); w.Code != http.StatusOK {
		t.Fatalf("the health route must stay open, got %d", w.Code)
	}
}

func TestATokenSignedByAnotherSecretIsRejected(t *testing.T) {
	srv := setupServerWithSecret(t, "0123456789abcdef0123456789abcdef")

	other := keyStoreWithSecret(t, "fedcba9876543210fedcba9876543210")
	foreign, err := other.SystemToken()
	if err != nil {
		t.Fatal(err)
	}

	if w := doRequest(t, srv, "/api/groups", "Bearer "+foreign); w.Code != http.StatusUnauthorized {
		t.Fatalf("a token of another secret must be rejected, got %d", w.Code)
	}
}

func TestAuthIsNotSkippedByOddPaths(t *testing.T) {
	srv := setupTestServer(t)

	for _, path := range []string{"/api/groups", "/api/groups/", "/API/groups", "/api/GROUPS", "/api/info/../groups", "/api//groups"} {
		w := doRequest(t, srv, path, "")
		if w.Code == http.StatusOK {
			t.Fatalf("%s must not answer without a key", path)
		}
		if strings.Contains(w.Body.String(), "\"63\"") {
			t.Fatalf("%s leaked the group list: %s", path, w.Body.String())
		}
	}
}

func TestUnknownApiPathsNeverAnswerWithoutAKey(t *testing.T) {
	srv := setupServerWithSecret(t, "0123456789abcdef0123456789abcdef")

	for _, path := range []string{"/api", "/api/unknown", "/api/info/extra", "/api/group", "/api/health/extra"} {
		w := doRequest(t, srv, path, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: an unmatched api path must still be gated, got %d (%s)", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "\"63\"") {
			t.Fatalf("%s leaked the group list: %s", path, w.Body.String())
		}
	}

	if w := doRequest(t, srv, "/other", ""); w.Code != http.StatusNotFound {
		t.Fatalf("a non-api path stays a plain 404, got %d", w.Code)
	}
}

func TestForeignKeyShapesAreRejectedAtTheHttpLayer(t *testing.T) {
	srv := setupServerWithSecret(t, "0123456789abcdef0123456789abcdef")
	tool := apikey.NewTool("0123456789abcdef0123456789abcdef")

	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}

	missingID, err := tool.Encode(987654321, iv)
	if err != nil {
		t.Fatal(err)
	}
	if w := doRequest(t, srv, "/api/groups", "Bearer "+missingID); w.Code != http.StatusUnauthorized {
		t.Fatalf("a signed id without a stored key must be rejected, got %d", w.Code)
	}

	otherType, err := tool.EncodeAs(1, 1, iv)
	if err != nil {
		t.Fatal(err)
	}
	if w := doRequest(t, srv, "/api/groups", "Bearer "+otherType); w.Code != http.StatusUnauthorized {
		t.Fatalf("a token of another key type must be rejected, got %d", w.Code)
	}
}

func TestTheOAuthCallbackStaysReachableUnderTheApiPrefix(t *testing.T) {
	srv := setupServerWithSecret(t, "0123456789abcdef0123456789abcdef")

	reached := false
	srv.HandleGoogleOAuth("/api/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusBadRequest)
	})

	w := doRequest(t, srv, "/api/oauth/callback?code=x&state=y", "")
	if !reached {
		t.Fatalf("the google redirect must reach the callback, got %d (%s)", w.Code, w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected callback status %d", w.Code)
	}

	if w := doRequest(t, srv, "/api/groups", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("the rest of the api must stay closed, got %d", w.Code)
	}
	if w := doRequest(t, srv, "/api/oauth/callback/other", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("only the registered callback is public, got %d", w.Code)
	}
}

func TestAuthUsesOnlyTheFirstAuthorizationHeader(t *testing.T) {
	srv := setupTestServer(t)

	w := requestWithHeaders(t, srv, "/api/groups", []string{"Bearer nonsense", "Bearer " + testToken})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("a second header must not rescue the request, got %d", w.Code)
	}

	w = requestWithHeaders(t, srv, "/api/groups", []string{"Bearer " + testToken, "Bearer nonsense"})
	if w.Code != http.StatusOK {
		t.Fatalf("the first header must decide, got %d", w.Code)
	}
}

func TestAKeyInTheQueryIsNotAccepted(t *testing.T) {
	srv := setupTestServer(t)

	if w := doRequest(t, srv, "/api/groups?token="+testToken, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("a key in the query must be ignored, got %d", w.Code)
	}
}

func TestParallelRequestsCannotExceedTheLimit(t *testing.T) {
	srv := setupTestServer(t)
	srv.limiter = apikey.NewLimiter(func() time.Time { return time.UnixMilli(0) })
	srv.keys.SetLimit(1, 4)

	status := func() int {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/groups", nil)
		if err != nil {
			t.Error(err)
			return 0
		}
		req.Header.Set("Authorization", "Bearer "+testToken)
		srv.Handler().ServeHTTP(w, req)
		return w.Code
	}

	var allowed, limited, other atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch status() {
			case http.StatusOK:
				allowed.Add(1)
			case http.StatusTooManyRequests:
				limited.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()

	if other.Load() != 0 {
		t.Fatalf("unexpected statuses: %d", other.Load())
	}
	if got := allowed.Load(); got != 4 {
		t.Fatalf("allowed = %d, want 4", got)
	}
	if got := limited.Load(); got != 36 {
		t.Fatalf("limited = %d, want 36", got)
	}
}

func TestRejectedRequestsDoNotLeakTheKey(t *testing.T) {
	tracker := health.NewDefaultTracker()
	srv := setupTestServerWith(t, tracker)

	w := doRequest(t, srv, "/api/groups?token="+testToken, "Bearer "+testToken+"broken")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), testToken) {
		t.Fatalf("the rejection echoed the key: %s", w.Body.String())
	}

	srv.limiter = apikey.NewLimiter(func() time.Time { return time.UnixMilli(0) })
	srv.keys.SetLimit(1, 1)
	doRequest(t, srv, "/api/groups", "Bearer "+testToken)
	w = doRequest(t, srv, "/api/groups", "Bearer "+testToken)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), testToken) {
		t.Fatalf("the limit rejection echoed the key: %s", w.Body.String())
	}

	raw, err := json.Marshal(tracker.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), testToken) {
		t.Fatalf("the key reached the health snapshot: %s", raw)
	}
	if strings.Contains(string(raw), "token=") {
		t.Fatalf("the query string reached the health snapshot: %s", raw)
	}
}
