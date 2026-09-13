package health

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func alertKeys(alerts []Alert) map[string]string {
	keys := make(map[string]string, len(alerts))
	for _, alert := range alerts {
		keys[alert.Key] = alert.Level
	}
	return keys
}

func TestTrackerStartsHealthy(t *testing.T) {
	tracker := NewDefaultTracker()

	if alerts := tracker.Alerts(); len(alerts) != 0 {
		t.Fatalf("fresh tracker should be healthy, got %+v", alerts)
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.Runs != 0 || snapshot.Calendar.Runs != 0 || snapshot.API.Requests != 0 {
		t.Errorf("unexpected counters: %+v", snapshot)
	}
}

func TestParserFailureAlerts(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.ParserFailures = 3
	tracker := NewTracker(thresholds)

	for i := 0; i < 2; i++ {
		tracker.ParserFailure(errors.New("site down"))
	}
	if alerts := tracker.Alerts(); len(alerts) != 0 {
		t.Fatalf("should not alert before the threshold, got %+v", alerts)
	}

	tracker.ParserFailure(errors.New("site down"))
	alerts := alertKeys(tracker.Alerts())
	if alerts[AlertParserFailures] != LevelCritical {
		t.Fatalf("expected parser failure alert, got %+v", alerts)
	}

	tracker.ParserSuccess(time.Second)
	if alerts := tracker.Alerts(); len(alerts) != 0 {
		t.Fatalf("success should clear the failure alert, got %+v", alerts)
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.ConsecutiveFailures != 0 || snapshot.Parser.Errors != 3 || snapshot.Parser.Runs != 4 {
		t.Errorf("unexpected parser stats: %+v", snapshot.Parser)
	}
	if snapshot.Parser.LastError != "site down" {
		t.Errorf("last error = %q", snapshot.Parser.LastError)
	}
	if snapshot.Parser.LastSuccessAt == "" || snapshot.Parser.LagSeconds != 0 {
		t.Errorf("last success = %+v", snapshot.Parser)
	}
}

func TestParserStaleAlert(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.ParserStale = time.Minute
	tracker := NewTracker(thresholds)

	tracker.ParserSuccess(time.Millisecond)
	tracker.mu.Lock()
	tracker.parserLastSuccess = time.Now().Add(-2 * time.Minute)
	tracker.mu.Unlock()

	alerts := alertKeys(tracker.Alerts())
	if alerts[AlertParserStale] != LevelWarning {
		t.Fatalf("expected parser stale alert, got %+v", alerts)
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.LagSeconds < 60 {
		t.Errorf("lag = %d, want at least 60", snapshot.Parser.LagSeconds)
	}
}

func TestCalendarAlerts(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.CalendarFailures = 2
	tracker := NewTracker(thresholds)

	tracker.CalendarSuccess(3)
	if alerts := tracker.Alerts(); len(alerts) != 0 {
		t.Fatalf("expected no alerts after success, got %+v", alerts)
	}

	tracker.CalendarFailure(errors.New("quota exceeded"))
	alerts := alertKeys(tracker.Alerts())
	if _, ok := alerts[AlertCalendarFailures]; ok {
		t.Fatalf("should not alert after a single failure, got %+v", alerts)
	}

	tracker.CalendarFailure(errors.New("quota exceeded"))
	alerts = alertKeys(tracker.Alerts())
	if alerts[AlertCalendarFailures] != LevelCritical {
		t.Fatalf("expected calendar failure alert, got %+v", alerts)
	}

	tracker.CalendarSuccess(1)
	snapshot := tracker.Snapshot()
	if snapshot.Calendar.DaysSynced != 4 || snapshot.Calendar.Errors != 2 {
		t.Errorf("unexpected calendar stats: %+v", snapshot.Calendar)
	}
}

func TestCalendarStaleRequiresObservedRuns(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.CalendarStale = time.Minute
	tracker := NewTracker(thresholds)

	alerts := alertKeys(tracker.Alerts())
	if _, ok := alerts[AlertCalendarStale]; ok {
		t.Fatalf("calendar alerts must wait for a first sync, got %+v", alerts)
	}

	tracker.CalendarSuccess(1)
	tracker.mu.Lock()
	tracker.calendarLastSuccess = time.Now().Add(-2 * time.Minute)
	tracker.mu.Unlock()

	alerts = alertKeys(tracker.Alerts())
	if alerts[AlertCalendarStale] != LevelWarning {
		t.Fatalf("expected calendar stale alert, got %+v", alerts)
	}
}

func TestAPIErrorsUseWindow(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APIErrors = 3
	thresholds.APIWindow = time.Minute
	tracker := NewTracker(thresholds)

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/groups", Status: 200, Duration: 5 * time.Millisecond})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/teacher/:name", Status: 404, Duration: 2 * time.Millisecond})
	for i := 0; i < 2; i++ {
		tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/health", Status: 500, Duration: 10 * time.Millisecond, Message: `{"error":"database is locked"}`})
	}

	if alerts := alertKeys(tracker.Alerts()); alerts[AlertAPIErrors] != "" {
		t.Fatalf("two errors should not cross a threshold of three, got %+v", alerts)
	}

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/info", Status: 503, Duration: 15 * time.Millisecond})
	if alerts := alertKeys(tracker.Alerts()); alerts[AlertAPIErrors] != LevelCritical {
		t.Fatalf("expected api error alert, got %+v", alerts)
	}

	snapshot := tracker.Snapshot()
	if snapshot.API.Requests != 5 || snapshot.API.Errors != 3 || snapshot.API.RecentErrors != 3 {
		t.Errorf("unexpected api stats: %+v", snapshot.API)
	}
	if snapshot.API.LastStatus != 503 || snapshot.API.SlowestMillis != 15 {
		t.Errorf("unexpected api latency: %+v", snapshot.API)
	}
	if snapshot.API.LastErrorAt == "" {
		t.Errorf("last error timestamp missing: %+v", snapshot.API)
	}

	tracker.mu.Lock()
	tracker.apiRecentErrors = []time.Time{time.Now().Add(-2 * time.Minute), time.Now().Add(-2 * time.Minute), time.Now().Add(-2 * time.Minute)}
	tracker.mu.Unlock()

	if alerts := alertKeys(tracker.Alerts()); alerts[AlertAPIErrors] != "" {
		t.Fatalf("errors outside the window must be pruned, got %+v", alerts)
	}
}

func TestParserLayoutAlertNeedsRepeatedRuns(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.ParserLayout = 2
	tracker := NewTracker(thresholds)

	issue := LayoutIssue{Source: "groups", Selector: "td lesson cells", Expected: "groups with at least one lesson"}
	tracker.ParserReport("groups", []LayoutIssue{issue}, nil)

	if alert := findAlert(tracker.Alerts(), AlertParserLayout); alert != nil {
		t.Errorf("a single run must not alert yet: %+v", alert)
	}

	tracker.ParserReport("groups", []LayoutIssue{issue}, nil)
	alert := findAlert(tracker.Alerts(), AlertParserLayout)
	if alert == nil {
		t.Fatal("expected a layout alert after two runs")
	}
	if !strings.Contains(alert.Detail, "td lesson cells") {
		t.Errorf("alert does not name the selector: %q", alert.Detail)
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.LayoutFailures != 2 {
		t.Errorf("layout failures = %d", snapshot.Parser.LayoutFailures)
	}
	if len(snapshot.Parser.Layout) != 1 || snapshot.Parser.Layout[0].Selector != "td lesson cells" {
		t.Errorf("layout issues = %+v", snapshot.Parser.Layout)
	}

	tracker.ParserReport("groups", nil, nil)
	if alert := findAlert(tracker.Alerts(), AlertParserLayout); alert != nil {
		t.Errorf("alert must clear after a clean run: %+v", alert)
	}
	if snapshot := tracker.Snapshot(); snapshot.Parser.LayoutFailures != 0 || len(snapshot.Parser.Layout) != 0 {
		t.Errorf("layout state was not cleared: %+v", snapshot.Parser)
	}
}

func TestParserGuardAlertsOnTheFirstTrip(t *testing.T) {
	tracker := NewTracker(DefaultThresholds())

	guard := []GuardIssue{{Source: "groups", Reason: "shrink", Detail: "35 -> 6 (dropped 82%, limit 80%)"}}
	tracker.ParserReport("groups", nil, guard)

	alert := findAlert(tracker.Alerts(), AlertParserGuard)
	if alert == nil {
		t.Fatal("a guard trip must alert on the first run")
	}
	if alert.Level != LevelCritical {
		t.Errorf("guard alert level = %s", alert.Level)
	}
	if !strings.Contains(alert.Detail, "groups: shrink: 35 -> 6") {
		t.Errorf("alert must carry the counts: %q", alert.Detail)
	}
	if findAlert(tracker.Alerts(), AlertParserLayout) != nil {
		t.Error("a guard trip is not a layout failure")
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.GuardFailures != 1 || len(snapshot.Parser.Guard) != 1 {
		t.Errorf("guard stats = %+v", snapshot.Parser)
	}

	tracker.ParserReport("groups", nil, nil)
	if alert := findAlert(tracker.Alerts(), AlertParserGuard); alert != nil {
		t.Errorf("guard alert must clear after a healthy parse: %+v", alert)
	}
	if snapshot := tracker.Snapshot(); snapshot.Parser.GuardFailures != 0 || len(snapshot.Parser.Guard) != 0 {
		t.Errorf("guard state was not cleared: %+v", snapshot.Parser)
	}
}

func TestParserGuardThresholdCountsRunsPerSource(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.ParserGuard = 3
	tracker := NewTracker(thresholds)

	guard := []GuardIssue{{Source: "calls", Reason: "empty", Detail: "the page produced nothing"}}
	tracker.ParserReport("calls", nil, guard)
	tracker.ParserReport("calls", nil, guard)

	if alert := findAlert(tracker.Alerts(), AlertParserGuard); alert != nil {
		t.Errorf("threshold is three runs: %+v", alert)
	}

	tracker.ParserReport("calls", nil, guard)
	if alert := findAlert(tracker.Alerts(), AlertParserGuard); alert == nil {
		t.Error("expected the third run to alert")
	}
}

func TestParserGuardCountsAnyGuardReason(t *testing.T) {
	tracker := NewTracker(DefaultThresholds())

	tracker.ParserReport("calls", nil, []GuardIssue{{Source: "calls", Reason: "fallback", Detail: "bell schedule read from text"}})

	if alert := findAlert(tracker.Alerts(), AlertParserGuard); alert == nil {
		t.Fatal("a fallback path must reach the admins")
	} else if !strings.Contains(alert.Detail, "calls: fallback: bell schedule read from text") {
		t.Errorf("alert detail = %q", alert.Detail)
	}
}

func TestParserLayoutThresholdUsesWorstSource(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.ParserLayout = 3
	tracker := NewTracker(thresholds)

	issue := []LayoutIssue{{Source: "groups", Selector: "table"}}
	tracker.ParserReport("groups", issue, nil)
	tracker.ParserReport("groups", issue, nil)
	tracker.ParserReport("teachers", issue, nil)

	if alert := findAlert(tracker.Alerts(), AlertParserLayout); alert != nil {
		t.Errorf("the threshold counts runs per source: %+v", alert)
	}

	tracker.ParserReport("groups", issue, nil)
	if alert := findAlert(tracker.Alerts(), AlertParserLayout); alert == nil {
		t.Error("expected the third groups run to alert")
	}
}

func findAlert(alerts []Alert, key string) *Alert {
	for i := range alerts {
		if alerts[i].Key == key {
			return &alerts[i]
		}
	}
	return nil
}

func TestSnapshotCountersAreMonotonic(t *testing.T) {
	tracker := NewDefaultTracker()

	tracker.ParserSuccess(time.Millisecond)
	tracker.ParserFailure(errors.New("boom"))
	tracker.CalendarSuccess(2)
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/health", Status: 200, Duration: time.Millisecond})

	first := tracker.Snapshot()
	second := tracker.Snapshot()

	if second.Parser.Runs != first.Parser.Runs || second.Parser.Errors != first.Parser.Errors {
		t.Errorf("snapshot should not mutate counters: %+v vs %+v", first.Parser, second.Parser)
	}
	if second.Calendar.DaysSynced != 2 {
		t.Errorf("days synced = %d", second.Calendar.DaysSynced)
	}
}

func TestAPIAlertNamesEndpointsAndLastErrors(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APIErrors = 3
	thresholds.APIWindow = time.Minute
	tracker := NewTracker(thresholds)

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/health", Status: 500, Message: `{"error":"database is locked"}`})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/health", Status: 500, Message: `{"error":"database is locked"}`})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/groups", Status: 503, Message: "upstream\ntimeout"})

	alert := findAlert(tracker.Alerts(), AlertAPIErrors)
	if alert == nil {
		t.Fatal("expected an api alert")
	}
	for _, want := range []string{
		"errors=3 window=1m0s",
		"paths: GET /api/health x2 (500), GET /api/groups x1 (503)",
		"last: 503 GET /api/groups: upstream timeout",
		"last: 500 GET /api/health: database is locked",
	} {
		if !strings.Contains(alert.Detail, want) {
			t.Errorf("alert detail %q misses %q", alert.Detail, want)
		}
	}

	snapshot := tracker.Snapshot()
	if len(snapshot.API.Endpoints) != 2 {
		t.Fatalf("endpoints = %+v", snapshot.API.Endpoints)
	}
	if snapshot.API.Endpoints[0].Path != "/api/health" || snapshot.API.Endpoints[0].Errors != 2 {
		t.Errorf("endpoints should be ordered by errors: %+v", snapshot.API.Endpoints)
	}
	if snapshot.API.Endpoints[0].Message != "database is locked" {
		t.Errorf("the json error body should be unwrapped: %+v", snapshot.API.Endpoints[0])
	}
	if snapshot.API.Endpoints[0].LastAt == "" {
		t.Errorf("endpoints need a timestamp: %+v", snapshot.API.Endpoints[0])
	}
	if len(snapshot.API.LastErrors) != 3 || snapshot.API.LastErrors[0].Status != 503 {
		t.Errorf("last errors should be newest first: %+v", snapshot.API.LastErrors)
	}
}

func TestAPIDiagnosticsFollowTheWindow(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APIWindow = time.Minute
	tracker := NewTracker(thresholds)

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/health", Status: 500, Message: "boom"})

	tracker.mu.Lock()
	for key, state := range tracker.apiEndpoints {
		state.lastAt = time.Now().Add(-2 * time.Minute)
		state.errorTimes = []time.Time{time.Now().Add(-2 * time.Minute)}
		tracker.apiEndpoints[key] = state
	}
	tracker.apiRecentErrors = []time.Time{time.Now().Add(-2 * time.Minute)}
	tracker.mu.Unlock()

	snapshot := tracker.Snapshot()
	if len(snapshot.API.Endpoints) != 1 {
		t.Fatalf("recent endpoints stay within the retention: %+v", snapshot.API.Endpoints)
	}
	if snapshot.API.Endpoints[0].Errors != 0 || snapshot.API.Endpoints[0].Requests != 1 {
		t.Errorf("errors outside the window are dropped, requests stay: %+v", snapshot.API.Endpoints[0])
	}
	if len(snapshot.API.LastErrors) != 1 {
		t.Errorf("the last error samples stay readable: %+v", snapshot.API.LastErrors)
	}
	if alerts := alertKeys(tracker.Alerts()); alerts[AlertAPIErrors] != "" {
		t.Errorf("stale errors must not alert, got %+v", alerts)
	}

	tracker.mu.Lock()
	for key, state := range tracker.apiEndpoints {
		state.lastAt = time.Now().Add(-2 * apiEndpointRetention)
		tracker.apiEndpoints[key] = state
	}
	tracker.mu.Unlock()

	if endpoints := tracker.Snapshot().API.Endpoints; len(endpoints) != 0 {
		t.Errorf("endpoints older than the retention must be pruned: %+v", endpoints)
	}
}

func TestAPIEndpointLatencyIsTrackedForEveryRequest(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APISlow = 20 * time.Millisecond
	tracker := NewTracker(thresholds)

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/groups", Status: 200, Duration: 10 * time.Millisecond})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/groups", Status: 200, Duration: 20 * time.Millisecond})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/groups", Status: 500, Duration: 30 * time.Millisecond, Message: "boom"})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/info", Status: 200, Duration: 3 * time.Millisecond})

	snapshot := tracker.Snapshot()
	if len(snapshot.API.Endpoints) != 2 {
		t.Fatalf("endpoints = %+v", snapshot.API.Endpoints)
	}

	groups := snapshot.API.Endpoints[0]
	if groups.Path != "/api/groups" {
		t.Fatalf("the failing endpoint must come first: %+v", snapshot.API.Endpoints)
	}
	if groups.Requests != 3 || groups.Errors != 1 {
		t.Errorf("requests and errors must add up: %+v", groups)
	}
	if groups.LastMillis != 30 || groups.AvgMillis != 20 || groups.MaxMillis != 30 {
		t.Errorf("latency of every request must be kept: %+v", groups)
	}
	if !groups.Slow {
		t.Errorf("20ms average against a 20ms limit must look slow: %+v", groups)
	}

	info := snapshot.API.Endpoints[1]
	if info.Requests != 1 || info.Errors != 0 || info.AvgMillis != 3 || info.Slow {
		t.Errorf("a healthy fast endpoint = %+v", info)
	}
}

func TestAPIAlertNamesSlowEndpoints(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APIErrors = 1
	thresholds.APISlow = 50 * time.Millisecond
	tracker := NewTracker(thresholds)

	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/group/:name", Status: 200, Duration: 400 * time.Millisecond})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/group/:name", Status: 200, Duration: 900 * time.Millisecond})
	tracker.RecordAPI(APIRequest{Method: "GET", Path: "/api/info", Status: 500, Message: "boom"})

	alert := findAlert(tracker.Alerts(), AlertAPIErrors)
	if alert == nil {
		t.Fatal("expected an api alert")
	}
	for _, want := range []string{
		"paths: GET /api/info x1 (500)",
		"slow: GET /api/group/:name avg=650ms max=900ms n=2 (last 900ms)",
	} {
		if !strings.Contains(alert.Detail, want) {
			t.Errorf("alert detail %q misses %q", alert.Detail, want)
		}
	}
}

func TestAPIMessageIsCompact(t *testing.T) {
	if got := apiMessage(" {\"error\":\"  boo m  \"} "); got != "boo m" {
		t.Errorf("apiMessage = %q", got)
	}
	if got := apiMessage("upstream\ntimeout"); got != "upstream timeout" {
		t.Errorf("plain message = %q", got)
	}
	if got := apiMessage("   "); got != "" {
		t.Errorf("blank message = %q", got)
	}
	long := apiMessage(strings.Repeat("x", 400))
	if len(long) != apiMessageLimit+3 || !strings.HasSuffix(long, "...") {
		t.Errorf("long message should be clipped, got %d chars", len(long))
	}
}
