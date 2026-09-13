package apiprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func TestTargetsCoverTheReadOnlyRoutes(t *testing.T) {
	raspCache, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	raspCache.SetGroups(map[string]any{"ИС-21": map[string]any{}}, "hash")
	raspCache.SetTeachers(map[string]any{"Иванов И.И.": map[string]any{}}, "hash")

	targets := Targets(raspCache)
	if len(targets) != 7 {
		t.Fatalf("targets = %+v", targets)
	}

	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Method != http.MethodGet {
			t.Errorf("target %s must be a GET", target.Path)
		}
		paths = append(paths, target.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{"/api/info", "/api/groups", "/api/teachers", "/api/parser-health", HealthPath} {
		if !strings.Contains(joined, want) {
			t.Errorf("targets miss %s: %v", want, paths)
		}
	}
	if !strings.Contains(joined, "/api/group/") || !strings.Contains(joined, "/api/teacher/") {
		t.Errorf("targets must exercise the named routes with real cache data: %v", paths)
	}
}

func TestTargetsWithoutCacheData(t *testing.T) {
	raspCache, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}

	if targets := Targets(raspCache); len(targets) != 5 {
		t.Errorf("an empty cache has no named routes to probe: %+v", targets)
	}
	if targets := Targets(nil); len(targets) != 5 {
		t.Errorf("a missing cache must not break the targets: %+v", targets)
	}
}

func TestRunMeasuresEveryTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/groups":
			w.WriteHeader(http.StatusInternalServerError)
		case HealthPath:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	results := Run(context.Background(), server.Client(), server.URL+"/", []Target{
		{http.MethodGet, "/api/info"},
		{http.MethodGet, "/api/groups"},
		{http.MethodGet, HealthPath},
	})

	if len(results) != 3 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Status != http.StatusOK || results[0].Duration <= 0 || !results[0].Healthy() {
		t.Errorf("healthy probe = %+v", results[0])
	}
	if results[1].Status != http.StatusInternalServerError || results[1].Healthy() {
		t.Errorf("failing probe = %+v", results[1])
	}
	if !results[2].Healthy() {
		t.Errorf("the deliberate 503 of the health route is not a failure: %+v", results[2])
	}
	if got := Describe(results[1]); got != "GET /api/groups (500)" {
		t.Errorf("describe = %q", got)
	}

	if HealthyCount(results) != 2 {
		t.Errorf("healthy count = %d", HealthyCount(results))
	}
	if Healthy(results) {
		t.Error("a run with a 500 is not healthy")
	}
	failed := Failed(results)
	if len(failed) != 1 || failed[0].Path != "/api/groups" {
		t.Errorf("failed = %+v", failed)
	}
}

func TestRunReportsAnUnreachableServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	address := server.URL
	server.Close()

	results := Run(context.Background(), &http.Client{Timeout: time.Second}, address, []Target{{Method: http.MethodGet, Path: "/api/info"}})
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Err == "" || results[0].Healthy() {
		t.Errorf("an unreachable server must be reported: %+v", results[0])
	}
	if !strings.Contains(Describe(results[0]), "GET /api/info: ") {
		t.Errorf("describe = %q", Describe(results[0]))
	}
	if Healthy(results) || HealthyCount(results) != 0 || len(Failed(results)) != 1 {
		t.Errorf("results = %+v", results)
	}
}

func TestSlowestPicksTheLongestResponse(t *testing.T) {
	results := []Result{
		{Method: "GET", Path: "/api/info", Status: 200, Duration: 4 * time.Millisecond},
		{Method: "GET", Path: "/api/groups", Status: 200, Duration: 620 * time.Millisecond},
		{Method: "GET", Path: "/api/teachers", Status: 200, Duration: 11 * time.Millisecond},
	}

	slowest, ok := Slowest(results)
	if !ok || slowest.Path != "/api/groups" {
		t.Fatalf("slowest = %+v ok = %v", slowest, ok)
	}
	if slowest.Label() != "GET /api/groups" {
		t.Errorf("label = %q", slowest.Label())
	}
	if _, ok := Slowest(nil); ok {
		t.Error("nothing to measure means no slowest entry")
	}
	if Healthy(nil) {
		t.Error("an empty run is not healthy")
	}
}
