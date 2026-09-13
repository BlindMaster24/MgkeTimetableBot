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

	tracker.APIRequest(200, 5*time.Millisecond)
	tracker.APIRequest(404, 2*time.Millisecond)
	for i := 0; i < 2; i++ {
		tracker.APIRequest(500, 10*time.Millisecond)
	}

	if alerts := alertKeys(tracker.Alerts()); alerts[AlertAPIErrors] != "" {
		t.Fatalf("two errors should not cross a threshold of three, got %+v", alerts)
	}

	tracker.APIRequest(503, 15*time.Millisecond)
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
	tracker.APIRequest(200, time.Millisecond)

	first := tracker.Snapshot()
	second := tracker.Snapshot()

	if second.Parser.Runs != first.Parser.Runs || second.Parser.Errors != first.Parser.Errors {
		t.Errorf("snapshot should not mutate counters: %+v vs %+v", first.Parser, second.Parser)
	}
	if second.Calendar.DaysSynced != 2 {
		t.Errorf("days synced = %d", second.Calendar.DaysSynced)
	}
}
