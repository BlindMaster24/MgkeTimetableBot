package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/gin-gonic/gin"
)

const testMetricsToken = "metrics-token-123"

type metricLine struct {
	name   string
	labels map[string]string
	value  float64
}

var metricLabelRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)="([^"]*)"`)

var metricLabelNames = map[string]bool{
	"method":  true,
	"path":    true,
	"version": true,
	"commit":  true,
	"mode":    true,
}

func setupMetricsServer(t *testing.T, tracker *health.Tracker, token string) *Server {
	t.Helper()

	srv := setupTestServerWith(t, tracker)
	srv.SetMetricsConfig(MetricsConfig{Token: token})
	srv.SetMetricsSources(MetricsSources{
		Archive: func() (archive.Stats, error) {
			return archive.Stats{Total: 42, WithGroup: 40, WithTeacher: 39}, nil
		},
		Bounds: func() (archive.Bounds, error) {
			return archive.Bounds{Min: 100, Max: 250}, nil
		},
		Webhook: func(context.Context) WebhookMetrics {
			return WebhookMetrics{
				Enabled:    true,
				Mode:       "webhook",
				Registered: true,
				Secret:     true,
				Reachable:  true,
				Pending:    7,
			}
		},
		Scheduler: func() int { return 12 },
		Reminders: func() int { return 3 },
	})
	return srv
}

func metricsRequest(t *testing.T, srv *Server, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, DefaultMetricsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	srv.Handler().ServeHTTP(w, req)
	return w
}

func scrapeMetrics(t *testing.T, srv *Server) (string, map[string]float64) {
	t.Helper()

	w := metricsRequest(t, srv, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != metricsContentType {
		t.Errorf("content type = %q, want %q", got, metricsContentType)
	}

	body := w.Body.String()
	samples := map[string]float64{}
	for _, line := range metricLines(t, body) {
		samples[line.name] = line.value
	}
	return body, samples
}

func metricLines(t *testing.T, body string) []metricLine {
	t.Helper()

	var lines []metricLine
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, " ") {
			t.Fatalf("metric line has no value: %q", line)
		}

		name, rest, hasLabels := strings.Cut(line, "{")
		labels := map[string]string{}
		if hasLabels {
			labelText, tail, found := strings.Cut(rest, "}")
			if !found {
				t.Fatalf("unterminated label set: %q", line)
			}
			labels = metricLabels(t, labelText)
			rest = tail
		} else {
			name, rest, _ = strings.Cut(line, " ")
		}

		value, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			t.Fatalf("metric %q has a non-numeric value: %v", line, err)
		}
		lines = append(lines, metricLine{name: name, labels: labels, value: value})
	}
	return lines
}

func metricLabels(t *testing.T, text string) map[string]string {
	t.Helper()

	labels := map[string]string{}
	matches := metricLabelRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		t.Fatalf("label set %q has no labels", text)
	}
	for _, match := range matches {
		if !metricLabelNames[match[1]] {
			t.Errorf("metric exposes the unexpected label %q", match[1])
		}
		labels[match[1]] = match[2]
	}
	return labels
}

func TestMetricsEndpointServesOnlyAggregates(t *testing.T) {
	tracker := health.NewDefaultTracker()
	tracker.ParserSuccess(2 * time.Second)
	tracker.ParserReport("groups", []health.LayoutIssue{{
		Source:   "groups",
		Selector: "td lesson cells",
		Expected: "groups with at least one lesson",
	}}, []health.GuardIssue{{
		Source: "groups",
		Reason: "shrink",
		Detail: "35 -> 6 (dropped 82%, limit 80%)",
	}})
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/info", Status: 200, Duration: 3 * time.Millisecond})
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/health", Status: 503, Duration: 4 * time.Millisecond})
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/unknown-route", Status: 404, Duration: time.Millisecond})

	srv := setupMetricsServer(t, tracker, "")
	scrapeMetrics(t, srv)
	body, samples := scrapeMetrics(t, srv)

	for _, name := range []string{
		"mgke_build_info",
		"mgke_cache_groups",
		"mgke_cache_teachers",
		"mgke_cache_hits_total",
		"mgke_cache_misses_total",
		"mgke_cache_last_success",
		"mgke_cache_groups_last_update_seconds",
		"mgke_cache_teachers_last_update_seconds",
		"mgke_uptime_seconds",
		"mgke_parser_runs_total",
		"mgke_parser_errors_total",
		"mgke_parser_consecutive_failures",
		"mgke_parser_lag_seconds",
		"mgke_parser_last_duration_seconds",
		"mgke_parser_layout_failures",
		"mgke_parser_guard_failures",
		"mgke_calendar_runs_total",
		"mgke_calendar_errors_total",
		"mgke_calendar_consecutive_failures",
		"mgke_calendar_days_synced_total",
		"mgke_calendar_lag_seconds",
		"mgke_alerts_active",
		"mgke_api_requests_total",
		"mgke_api_errors_total",
		"mgke_api_slowest_seconds",
		"mgke_archive_rows",
		"mgke_archive_rows_with_group",
		"mgke_archive_rows_with_teacher",
		"mgke_archive_first_day_index",
		"mgke_archive_last_day_index",
		"mgke_scheduler_entries",
		"mgke_reminders_tracked",
		"mgke_webhook_enabled",
		"mgke_webhook_mode",
		"mgke_webhook_registered",
		"mgke_webhook_secret_configured",
		"mgke_webhook_reachable",
		"mgke_webhook_pending_updates",
	} {
		if !strings.Contains(body, "# TYPE "+name+" ") {
			t.Errorf("the scrape is missing %s", name)
		}
	}

	if samples["mgke_cache_groups"] != 2 || samples["mgke_cache_teachers"] != 1 {
		t.Errorf("cache sizes = %v / %v, want 2 / 1", samples["mgke_cache_groups"], samples["mgke_cache_teachers"])
	}
	if samples["mgke_parser_runs_total"] != 1 {
		t.Errorf("parser runs = %v, want 1", samples["mgke_parser_runs_total"])
	}
	if samples["mgke_parser_layout_failures"] != 1 || samples["mgke_parser_guard_failures"] != 1 {
		t.Errorf("parser diagnostics = %v / %v", samples["mgke_parser_layout_failures"], samples["mgke_parser_guard_failures"])
	}
	if samples["mgke_archive_rows"] != 42 || samples["mgke_archive_first_day_index"] != 100 || samples["mgke_archive_last_day_index"] != 250 {
		t.Errorf("archive gauges = %v / %v / %v", samples["mgke_archive_rows"], samples["mgke_archive_first_day_index"], samples["mgke_archive_last_day_index"])
	}
	if samples["mgke_scheduler_entries"] != 12 || samples["mgke_reminders_tracked"] != 3 {
		t.Errorf("notification gauges = %v / %v", samples["mgke_scheduler_entries"], samples["mgke_reminders_tracked"])
	}
	if samples["mgke_webhook_pending_updates"] != 7 || samples["mgke_webhook_enabled"] != 1 || samples["mgke_webhook_secret_configured"] != 1 {
		t.Errorf("webhook gauges = %v / %v / %v", samples["mgke_webhook_pending_updates"], samples["mgke_webhook_enabled"], samples["mgke_webhook_secret_configured"])
	}

	endpoints := map[string]metricLine{}
	for _, line := range metricLines(t, body) {
		if path, ok := line.labels["path"]; ok {
			endpoints[path+" "+line.labels["method"]] = line
		}
	}
	if _, ok := endpoints["/api/info GET"]; !ok {
		t.Errorf("the scrape lost the registered /api/info route: %v", endpoints)
	}
	if _, ok := endpoints["/api/health GET"]; !ok {
		t.Errorf("the scrape lost the registered /api/health route: %v", endpoints)
	}
	if _, ok := endpoints["/api/unknown-route GET"]; ok {
		t.Errorf("the scrape exposes a route the server does not serve: %v", endpoints)
	}
	if _, ok := endpoints["/metrics GET"]; !ok {
		t.Errorf("the scrape should report its own route: %v", endpoints)
	}
}

func TestMetricsEndpointIsSilentOnSourceFailures(t *testing.T) {
	srv := setupMetricsServer(t, health.NewDefaultTracker(), "")
	srv.SetMetricsSources(MetricsSources{
		Archive: func() (archive.Stats, error) { return archive.Stats{}, errors.New("archive is locked") },
		Bounds:  func() (archive.Bounds, error) { return archive.Bounds{}, errors.New("archive is locked") },
	})

	body, _ := scrapeMetrics(t, srv)
	for _, name := range []string{"mgke_archive_rows", "mgke_archive_first_day_index"} {
		if strings.Contains(body, name) {
			t.Errorf("%s must stay absent while its source fails", name)
		}
	}
	if !strings.Contains(body, "mgke_uptime_seconds") {
		t.Error("a failing source must not drop the rest of the scrape")
	}
}

func TestMetricsEndpointReportsAnEmptyCacheAsZero(t *testing.T) {
	fresh, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	srv := NewServer(fresh, 0, health.NewDefaultTracker(), build.New("test", "abcdef1234567890", "2026-01-02T03:04:05Z"), nil, nil)
	srv.SetMetricsConfig(MetricsConfig{})

	body, samples := scrapeMetrics(t, srv)
	if samples["mgke_cache_groups"] != 0 || samples["mgke_cache_teachers"] != 0 {
		t.Errorf("an empty cache must report zero, got %v / %v", samples["mgke_cache_groups"], samples["mgke_cache_teachers"])
	}
	if samples["mgke_cache_groups_last_update_seconds"] != 0 || samples["mgke_cache_teachers_last_update_seconds"] != 0 {
		t.Errorf("a cache that was never filled must report a zero timestamp")
	}
	if samples["mgke_cache_last_success"] != 1 {
		t.Errorf("the cache starts optimistic about its last update, got %v", samples["mgke_cache_last_success"])
	}
	if !strings.Contains(body, "mgke_build_info") {
		t.Error("the build info label is the only free-form value the scrape carries")
	}
}

func TestMetricsEndpointHonoursTheTokenGate(t *testing.T) {
	srv := setupMetricsServer(t, health.NewDefaultTracker(), testMetricsToken)

	w := metricsRequest(t, srv, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("a request without a token: expected 401, got %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("a rejected scrape should ask for a bearer token")
	}
	if strings.Contains(w.Body.String(), "mgke_") {
		t.Error("a rejected scrape must not carry metrics")
	}

	w = metricsRequest(t, srv, "Bearer wrong-token-value")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("a request with a foreign token: expected 401, got %d", w.Code)
	}

	w = metricsRequest(t, srv, "Basic "+testMetricsToken)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("a non-bearer scheme: expected 401, got %d", w.Code)
	}

	w = metricsRequest(t, srv, "bearer "+testMetricsToken)
	if w.Code != http.StatusOK {
		t.Fatalf("the bearer scheme is case-insensitive: expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), testMetricsToken) {
		t.Error("the scrape must never echo its own token")
	}
}

func TestMetricsEndpointMovesToTheConfiguredPath(t *testing.T) {
	srv := setupTestServerWith(t, health.NewDefaultTracker())
	srv.SetMetricsConfig(MetricsConfig{Path: "/internal/metrics"})
	if got := srv.MetricsPath(); got != "/internal/metrics" {
		t.Fatalf("metrics path = %q", got)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/internal/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("the configured path must answer, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "mgke_") {
		t.Error("the configured path must serve the scrape")
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodGet, DefaultMetricsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("moving the route must leave the default path free, got %d", w.Code)
	}
}

func TestMetricsEndpointStaysOutsideTheAPIKeyGate(t *testing.T) {
	srv := setupMetricsServer(t, health.NewDefaultTracker(), "")

	if w := metricsRequest(t, srv, ""); w.Code != http.StatusOK {
		t.Fatalf("the metrics route needs no REST API key, got %d", w.Code)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/groups", nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("the REST API stays key-protected, got %d", w.Code)
	}
}

func TestMetricsEndpointAnswersOverARealSocket(t *testing.T) {
	srv := setupMetricsServer(t, health.NewDefaultTracker(), testMetricsToken)
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	unauthorized, err := http.Get(server.URL + DefaultMetricsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a scrape without a token: expected 401, got %d", unauthorized.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+DefaultMetricsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testMetricsToken)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != metricsContentType {
		t.Errorf("content type over the wire = %q, want %q", got, metricsContentType)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "mgke_uptime_seconds") {
		t.Errorf("the served scrape carries no metrics: %s", body)
	}
}

func TestMetricsScrapeCarriesNoSecrets(t *testing.T) {
	srv := setupMetricsServer(t, health.NewDefaultTracker(), "")
	body, _ := scrapeMetrics(t, srv)

	for _, forbidden := range []string{testSecret, testMetricsToken, "Bearer", "https://", "http://", "@", "chat_id", "token="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the scrape leaks %q", forbidden)
		}
	}
}
