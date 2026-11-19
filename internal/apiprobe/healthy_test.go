package apiprobe

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthyClassesEveryStatus(t *testing.T) {
	cases := []struct {
		name   string
		result Result
		want   bool
	}{
		{"ok", Result{Path: "/api/info", Status: 200}, true},
		{"redirect boundary", Result{Path: "/api/info", Status: 399}, true},
		{"client error", Result{Path: "/api/info", Status: 400}, false},
		{"last client error", Result{Path: "/api/info", Status: 499}, false},
		{"server error", Result{Path: "/api/info", Status: 500}, false},
		{"health unavailable is fine", Result{Path: HealthPath, Status: 503}, true},
		{"health with other 5xx is not", Result{Path: HealthPath, Status: 500}, false},
		{"normal path with 503 is not", Result{Path: "/api/info", Status: 503}, false},
		{"transport error", Result{Path: "/api/info", Status: 200, Err: "connection refused"}, false},
	}
	for _, tc := range cases {
		if got := tc.result.Healthy(); got != tc.want {
			t.Errorf("%s: Healthy() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestHealthyNeedsEveryResultToPass(t *testing.T) {
	if Healthy(nil) {
		t.Error("an empty probe set proves nothing, so it must not report healthy")
	}
	ok := []Result{{Path: "/api/info", Status: 200}, {Path: HealthPath, Status: 503}}
	if !Healthy(ok) {
		t.Error("all healthy results must report healthy")
	}
	if Healthy(append(ok, Result{Path: "/api/info", Status: 500})) {
		t.Error("one broken result must fail the whole set")
	}
	if got := HealthyCount(append(ok, Result{Path: "/api/info", Status: 500})); got != 2 {
		t.Errorf("HealthyCount = %d, want 2", got)
	}
	if failed := Failed(append(ok, Result{Path: "/api/info", Status: 500})); len(failed) != 1 {
		t.Errorf("Failed = %+v, want one entry", failed)
	}
}

func TestSlowestHandlesEmptyAndTies(t *testing.T) {
	if _, ok := Slowest(nil); ok {
		t.Fatal("an empty set has no slowest probe")
	}
	results := []Result{
		{Path: "/first", Duration: 10},
		{Path: "/second", Duration: 10},
		{Path: "/third", Duration: 5},
	}
	slowest, ok := Slowest(results)
	if !ok || slowest.Path != "/first" {
		t.Fatalf("slowest = %+v, want the first of the equal probes", slowest)
	}
}

func TestRunUsesTheDefaultClientAndSendsTheToken(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	results := Run(t.Context(), nil, server.URL, []Target{{Method: http.MethodGet, Path: "/api/info"}}, "secret")
	if len(results) != 1 || !results[0].Healthy() {
		t.Fatalf("results = %+v", results)
	}
	if auth != "Bearer secret" {
		t.Fatalf("authorization = %q, want the bearer token", auth)
	}

	results = Run(t.Context(), nil, server.URL, []Target{{Method: http.MethodGet, Path: "/api/info"}}, "")
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if auth != "" {
		t.Fatalf("authorization = %q, want none without a token", auth)
	}
}
