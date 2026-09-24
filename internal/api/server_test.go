package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/gin-gonic/gin"

	_ "modernc.org/sqlite"
)

const testSecret = "0123456789abcdef0123456789abcdef"

var testToken string

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	return setupTestServerWith(t, health.NewDefaultTracker())
}

func setupTestStore(t *testing.T) *apikey.Store {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	store := apikey.NewStore(db, testSecret)
	if err := store.EnsureSchema(); err != nil {
		t.Fatal(err)
	}

	key, _, err := store.FindOrCreate(1)
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.Token(key)
	if err != nil {
		t.Fatal(err)
	}
	testToken = token

	return store
}

func call(t *testing.T, srv *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	srv.Handler().ServeHTTP(w, req)
	return w
}

func setupTestServerWith(t *testing.T, tracker *health.Tracker) *Server {
	t.Helper()
	dir := t.TempDir()
	c, err := cache.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(map[string]any{
		"63": map[string]any{"group": "63", "days": []any{}},
		"64": map[string]any{"group": "64", "days": []any{}},
	}, "hash1")
	c.SetTeachers(map[string]any{
		"Иванов": map[string]any{"teacher": "Иванов"},
	}, "hash2")
	gin.SetMode(gin.TestMode)
	return NewServer(c, 0, tracker, build.New("test", "abcdef1234567890", "2026-01-02T03:04:05Z"), setupTestStore(t), i18n.New("ru"))
}

func TestHealthEndpointReportsTrackerState(t *testing.T) {
	tracker := health.NewDefaultTracker()
	tracker.ParserSuccess(2 * time.Second)
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/health", Status: 200, Duration: time.Millisecond})
	srv := setupTestServerWith(t, tracker)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body health.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if body.Parser.Runs != 1 || body.Parser.LastSuccessAt == "" {
		t.Errorf("parser stats = %+v", body.Parser)
	}
	if body.API.Requests != 1 {
		t.Errorf("the health request itself is counted after the response, got %+v", body.API)
	}
	if after := tracker.Snapshot().API; after.Requests != 2 || after.LastStatus != http.StatusOK {
		t.Errorf("middleware should record the request, got %+v", after)
	}
	if len(body.Alerts) != 0 {
		t.Errorf("expected no alerts, got %+v", body.Alerts)
	}
}

func TestServerRecordsEndpointFailures(t *testing.T) {
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = io.Discard
	defer func() { gin.DefaultErrorWriter = previousWriter }()

	tracker := health.NewDefaultTracker()
	srv := setupTestServerWith(t, tracker)
	srv.engine.GET("/api/boom", func(c *gin.Context) { panic("index out of range") })
	srv.engine.GET("/api/fail", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is locked"})
	})

	for _, target := range []string{"/api/boom", "/api/fail"} {
		w := call(t, srv, "GET", target)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("%s: expected 500, got %d", target, w.Code)
		}
	}

	api := tracker.Snapshot().API
	if api.Errors != 2 || api.RecentErrors != 2 {
		t.Fatalf("api errors = %+v", api)
	}

	endpoints := map[string]health.APIEndpointStat{}
	for _, endpoint := range api.Endpoints {
		endpoints[endpoint.Path] = endpoint
	}
	if got := endpoints["/api/boom"]; got.Status != 500 || got.Method != "GET" || got.Message != "index out of range" {
		t.Errorf("panic endpoint = %+v", got)
	}
	if got := endpoints["/api/fail"]; got.Status != 500 || got.Message != "database is locked" {
		t.Errorf("failing endpoint = %+v", got)
	}
	if len(api.LastErrors) != 2 || api.LastErrors[0].At == "" {
		t.Errorf("last errors = %+v", api.LastErrors)
	}
}

func TestHealthProbeIsNotAnAPIError(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.ParserFailures = 1
	tracker := health.NewTracker(thresholds)
	tracker.ParserFailure(errors.New("site down"))
	srv := setupTestServerWith(t, tracker)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", healthRoute, nil)
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 while alerting, got %d", w.Code)
		}
	}

	api := tracker.Snapshot().API
	if api.Errors != 0 || api.RecentErrors != 0 || len(api.LastErrors) != 0 {
		t.Errorf("the deliberate 503 must not count as a failure: %+v", api)
	}
	if len(api.Endpoints) != 1 || api.Endpoints[0].Errors != 0 || api.Endpoints[0].Requests != 3 {
		t.Errorf("the probe is still a request without errors: %+v", api.Endpoints)
	}
	if api.Requests != 3 || api.LastStatus != http.StatusServiceUnavailable {
		t.Errorf("the probe is still a request: %+v", api)
	}
}

func TestHealthEndpointExposesParserLayout(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.ParserLayout = 1
	tracker := health.NewTracker(thresholds)
	tracker.ParserReport("groups", []health.LayoutIssue{{
		Source:   "groups",
		Selector: "td lesson cells",
		Expected: "groups with at least one lesson",
	}}, nil)
	srv := setupTestServerWith(t, tracker)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while the layout alert is active, got %d", w.Code)
	}

	var body health.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(body.Parser.Layout) != 1 || body.Parser.Layout[0].Selector != "td lesson cells" {
		t.Errorf("layout issues = %+v", body.Parser.Layout)
	}
	if body.Parser.LayoutFailures != 1 {
		t.Errorf("layout failures = %d", body.Parser.LayoutFailures)
	}

	found := false
	for _, alert := range body.Alerts {
		if alert.Key == health.AlertParserLayout {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the parser layout alert, got %+v", body.Alerts)
	}
}

func TestHealthEndpointCarriesTheBuildInfo(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	srv.Handler().ServeHTTP(w, req)

	var body health.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if body.Build == nil {
		t.Fatal("expected the snapshot to carry the build info")
	}
	if body.Build.Version != "test" || body.Build.ShortCommit() != "abcdef1" {
		t.Errorf("build info = %+v", body.Build)
	}
	if body.Build.Go == "" || body.Build.OS == "" {
		t.Errorf("build info must name the toolchain: %+v", body.Build)
	}
}

func TestInfoEndpointCarriesTheBuildInfo(t *testing.T) {
	srv := setupTestServer(t)

	w := call(t, srv, "GET", "/api/info")

	var body struct {
		Name    string     `json:"name"`
		Version string     `json:"version"`
		Build   build.Info `json:"build"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if body.Build.Version != "test" || body.Build.Date != "2026-01-02T03:04:05Z" {
		t.Errorf("info build = %+v", body.Build)
	}
}

func TestHealthEndpointExposesTheParserGuard(t *testing.T) {
	tracker := health.NewDefaultTracker()
	tracker.ParserReport("groups", nil, []health.GuardIssue{{
		Source: "groups",
		Reason: "shrink",
		Detail: "35 -> 6 (dropped 82%, limit 80%)",
	}})
	srv := setupTestServerWith(t, tracker)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while the guard alert is active, got %d", w.Code)
	}

	var body health.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if body.Parser.GuardFailures != 1 {
		t.Errorf("guard failures = %d", body.Parser.GuardFailures)
	}
	if len(body.Parser.Guard) != 1 || !strings.Contains(body.Parser.Guard[0].Detail, "35 -> 6") {
		t.Errorf("guard issues = %+v", body.Parser.Guard)
	}

	found := false
	for _, alert := range body.Alerts {
		if alert.Key == health.AlertParserGuard {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the parser guard alert, got %+v", body.Alerts)
	}
}

func TestHealthEndpointFailsWhileAlerting(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.ParserFailures = 1
	tracker := health.NewTracker(thresholds)
	tracker.ParserFailure(errTest{})
	srv := setupTestServerWith(t, tracker)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while alerting, got %d", w.Code)
	}

	var body health.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(body.Alerts) != 1 || body.Alerts[0].Key != health.AlertParserFailures {
		t.Errorf("alerts = %+v", body.Alerts)
	}
}

func TestHealthMiddlewareCountsServerErrors(t *testing.T) {
	tracker := health.NewDefaultTracker()
	srv := setupTestServerWith(t, tracker)

	w := call(t, srv, "GET", "/api/group/missing")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	snapshot := tracker.Snapshot()
	if snapshot.API.Requests != 1 || snapshot.API.Errors != 0 || snapshot.API.LastStatus != http.StatusNotFound {
		t.Errorf("api stats = %+v", snapshot.API)
	}
}

type errTest struct{}

func (errTest) Error() string { return "test failure" }

func TestHandleInfo(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/info")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["name"] != "MgkeTimetableBot API" {
		t.Errorf("expected name MgkeTimetableBot API, got %s", body["name"])
	}
	if body["version"] != "2.0" {
		t.Errorf("expected version 2.0, got %s", body["version"])
	}
}

func TestHandleGroups(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/groups")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string][]string
	json.Unmarshal(w.Body.Bytes(), &body)
	groups := body["groups"]
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestHandleTeachers(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/teachers")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string][]string
	json.Unmarshal(w.Body.Bytes(), &body)
	teachers := body["teachers"]
	if len(teachers) != 1 {
		t.Errorf("expected 1 teacher, got %d", len(teachers))
	}
}

func TestHandleGroupByNameFound(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/group/63")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleGroupByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/group/999")

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTeacherByNameFound(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/teacher/Иванов")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleTeacherByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/teacher/Несуществующий")

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleParserHealth(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/parser-health")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Cache struct {
			GroupsCount   int `json:"groupsCount"`
			TeachersCount int `json:"teachersCount"`
		} `json:"cache"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Cache.GroupsCount != 2 {
		t.Errorf("expected 2 groups in stats, got %d", body.Cache.GroupsCount)
	}
	if body.Cache.TeachersCount != 1 {
		t.Errorf("expected 1 teacher in stats, got %d", body.Cache.TeachersCount)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{8080, "8080"},
		{300, "300"},
		{9999, "9999"},
	}
	for _, c := range cases {
		got := itoa(c.n)
		if got != c.want {
			t.Errorf("itoa(%d) = %s, want %s", c.n, got, c.want)
		}
	}
}
