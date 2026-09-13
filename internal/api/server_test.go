package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/gin-gonic/gin"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	return setupTestServerWith(t, health.NewDefaultTracker())
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
	return NewServer(c, 0, tracker)
}

func TestHealthEndpointReportsTrackerState(t *testing.T) {
	tracker := health.NewDefaultTracker()
	tracker.ParserSuccess(2 * time.Second)
	tracker.APIRequest(200, time.Millisecond)
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

func TestHealthEndpointExposesParserLayout(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.ParserLayout = 1
	tracker := health.NewTracker(thresholds)
	tracker.ParserReport("groups", []health.LayoutIssue{{
		Source:   "groups",
		Selector: "td lesson cells",
		Expected: "groups with at least one lesson",
	}}, false)
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/group/missing", nil)
	srv.Handler().ServeHTTP(w, req)
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
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/info", nil)
	srv.Handler().ServeHTTP(w, req)

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
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/groups", nil)
	srv.Handler().ServeHTTP(w, req)

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
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teachers", nil)
	srv.Handler().ServeHTTP(w, req)

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
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/group/63", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleGroupByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/group/999", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTeacherByNameFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teacher/Иванов", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleTeacherByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teacher/Несуществующий", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleParserHealth(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/parser-health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body cache.Stats
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.GroupsCount != 2 {
		t.Errorf("expected 2 groups in stats, got %d", body.GroupsCount)
	}
	if body.TeachersCount != 1 {
		t.Errorf("expected 1 teacher in stats, got %d", body.TeachersCount)
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
