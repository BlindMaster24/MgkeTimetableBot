package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func doRequest(t *testing.T, srv *Server, path, header string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	srv.Handler().ServeHTTP(w, req)
	return w
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return body["error"]
}

func TestProtectedEndpointsRejectMissingKeys(t *testing.T) {
	srv := setupTestServer(t)

	for _, path := range []string{
		"/api/info",
		"/api/groups",
		"/api/teachers",
		"/api/group/63",
		"/api/teacher/Иванов",
		"/api/parser-health",
	} {
		w := doRequest(t, srv, path, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, w.Code)
		}
		if got := errorBody(t, w); got != "Неверный ключ авторизации" {
			t.Fatalf("%s: unexpected error %q", path, got)
		}
	}
}

func TestProtectedEndpointsRejectBrokenKeys(t *testing.T) {
	srv := setupTestServer(t)

	cases := map[string]string{
		"garbage":     "Bearer nonsense",
		"empty":       "Bearer ",
		"bareToken":   testToken,
		"doubleSpace": "Bearer  " + testToken,
		"truncated":   "Bearer " + testToken[:len(testToken)-6],
		"recased":     "Bearer " + strings.ToUpper(testToken),
	}

	for name, header := range cases {
		w := doRequest(t, srv, "/api/groups", header)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, w.Code)
		}
	}
}

func TestAuthorizationSchemeIsIgnoredLikeInTheOldBot(t *testing.T) {
	srv := setupTestServer(t)

	w := doRequest(t, srv, "/api/groups", "Token "+testToken)
	if w.Code != http.StatusOK {
		t.Fatalf("the old bot took the second word of the header, expected 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestProtectedEndpointsAcceptARealKey(t *testing.T) {
	srv := setupTestServer(t)

	w := doRequest(t, srv, "/api/groups", "Bearer "+testToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestHealthEndpointStaysOpen(t *testing.T) {
	srv := setupTestServer(t)

	w := doRequest(t, srv, healthRoute, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequestMarksTheKeyAsUsed(t *testing.T) {
	srv := setupTestServer(t)

	key, err := srv.keys.ByToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	if key.Used {
		t.Fatal("a fresh key must not be marked as used")
	}

	started := time.Now().Add(-time.Second)
	if w := doRequest(t, srv, "/api/groups", "Bearer "+testToken); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	updated, err := srv.keys.ByToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Used || updated.LastUsed.Before(started) {
		t.Fatalf("unexpected usage state: %+v", updated)
	}
}

func TestRequestsOverTheLimitAreRejected(t *testing.T) {
	srv := setupTestServer(t)
	srv.limiter = apikey.NewLimiter(func() time.Time { return time.UnixMilli(0) })
	srv.keys.SetLimit(1, 2)

	for i := 0; i < 2; i++ {
		if w := doRequest(t, srv, "/api/groups", "Bearer "+testToken); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRequest(t, srv, "/api/groups", "Bearer "+testToken)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if got := errorBody(t, w); got != "Превышен лимит запросов" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestUnlimitedKeyNeverHitsTheLimiter(t *testing.T) {
	srv := setupTestServer(t)
	srv.limiter = apikey.NewLimiter(func() time.Time { return time.UnixMilli(0) })
	srv.keys.SetLimit(1, 0)

	for i := 0; i < 10; i++ {
		if w := doRequest(t, srv, "/api/groups", "Bearer "+testToken); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRotatedKeyStopsWorking(t *testing.T) {
	srv := setupTestServer(t)

	if err := srv.keys.Rotate(1); err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, srv, "/api/groups", "Bearer "+testToken)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestServerWithoutAStoreRejectsEverything(t *testing.T) {
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(c, 0, nil, build.New("test", "abcdef1234567890", "2026-01-02T03:04:05Z"), nil, nil)

	w := doRequest(t, srv, "/api/groups", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if got := errorBody(t, w); got != "api_unauthorized" {
		t.Fatalf("without a localizer the key itself is reported, got %q", got)
	}

	if w := doRequest(t, srv, healthRoute, ""); w.Code != http.StatusOK {
		t.Fatalf("the health route must stay open, got %d", w.Code)
	}
}
