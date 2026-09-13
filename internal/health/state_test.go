package health

import (
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	values map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{values: make(map[string]string)}
}

func (s *memoryStore) LoadState(key string) (string, bool, error) {
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *memoryStore) SaveState(key, value string) error {
	s.values[key] = value
	return nil
}

func TestTrackerRoundTripsThroughTheStore(t *testing.T) {
	store := newMemoryStore()

	thresholds := DefaultThresholds()
	thresholds.ParserFailures = 2
	thresholds.CalendarFailures = 1
	tracker := NewTracker(thresholds)

	tracker.ParserSuccess(120 * time.Millisecond)
	tracker.ParserFailure(errors.New("site down"))
	tracker.CalendarSuccess(3)
	tracker.CalendarFailure(errors.New("quota exceeded"))
	tracker.APIRequest(500, 10*time.Millisecond)
	tracker.ParserReport("groups", []LayoutIssue{{Source: "groups", Selector: "table"}}, nil)
	tracker.ParserReport("teachers", nil, []GuardIssue{{Source: "teachers", Reason: "shrink", Detail: "81 -> 4 (dropped 95%, limit 80%)"}})

	if err := tracker.Flush(store); err != nil {
		t.Fatalf("flush: %v", err)
	}

	restored := NewTracker(thresholds)
	if err := restored.Restore(store); err != nil {
		t.Fatalf("restore: %v", err)
	}

	before := tracker.Snapshot()
	after := restored.Snapshot()

	if after.Parser.Runs != before.Parser.Runs || after.Parser.Errors != before.Parser.Errors {
		t.Errorf("parser counters: %+v vs %+v", before.Parser, after.Parser)
	}
	if after.Parser.ConsecutiveFailures != before.Parser.ConsecutiveFailures {
		t.Errorf("parser failures: %+v vs %+v", before.Parser, after.Parser)
	}
	if after.Parser.LastError != "site down" || after.Parser.LastErrorAt == "" {
		t.Errorf("last error must survive a restart: %+v", after.Parser)
	}
	if after.Parser.LastDurationMS != before.Parser.LastDurationMS {
		t.Errorf("duration: %d vs %d", after.Parser.LastDurationMS, before.Parser.LastDurationMS)
	}
	if after.Parser.LayoutFailures != 1 {
		t.Errorf("layout failures = %d, want 1", after.Parser.LayoutFailures)
	}
	if after.Parser.GuardFailures != 1 {
		t.Errorf("guard failures = %d, want 1", after.Parser.GuardFailures)
	}
	if alerts := alertKeys(restored.Alerts()); alerts[AlertParserGuard] != LevelCritical {
		t.Errorf("a guard trip must survive a restart, got %+v", alerts)
	}
	if after.Calendar.DaysSynced != before.Calendar.DaysSynced || after.Calendar.Errors != before.Calendar.Errors {
		t.Errorf("calendar counters: %+v vs %+v", before.Calendar, after.Calendar)
	}
	if after.Calendar.LastError != "quota exceeded" {
		t.Errorf("calendar error text lost: %+v", after.Calendar)
	}

	if alerts := alertKeys(restored.Alerts()); alerts[AlertCalendarFailures] != LevelCritical {
		t.Errorf("restored failures must keep alerting, got %+v", alerts)
	}
}

func TestTrackerRestoreKeepsStaleAlertsAfterRestart(t *testing.T) {
	store := newMemoryStore()

	thresholds := DefaultThresholds()
	thresholds.ParserStale = time.Minute
	thresholds.ParserFailures = 5

	tracker := NewTracker(thresholds)
	tracker.ParserSuccess(time.Millisecond)
	tracker.mu.Lock()
	tracker.parserLastSuccess = time.Now().Add(-time.Hour)
	tracker.mu.Unlock()
	if err := tracker.Flush(store); err != nil {
		t.Fatalf("flush: %v", err)
	}

	restored := NewTracker(thresholds)
	if err := restored.Restore(store); err != nil {
		t.Fatalf("restore: %v", err)
	}

	alert := findAlert(restored.Alerts(), AlertParserStale)
	if alert == nil {
		t.Fatal("a restart must not hide a parser that last succeeded an hour ago")
	}
	if lag := restored.Snapshot().Parser.LagSeconds; lag < 3600 {
		t.Errorf("lag = %d, want at least 3600", lag)
	}
}

func TestTrackerRestoreWithoutStateStaysEmpty(t *testing.T) {
	store := newMemoryStore()

	tracker := NewTracker(DefaultThresholds())
	tracker.ParserSuccess(time.Millisecond)
	if err := tracker.Restore(store); err != nil {
		t.Fatalf("restore: %v", err)
	}

	snapshot := tracker.Snapshot()
	if snapshot.Parser.Runs != 1 {
		t.Errorf("in-memory counters must be kept when nothing was stored: %+v", snapshot.Parser)
	}
}

func TestTrackerStateHelpersHandleNilStore(t *testing.T) {
	tracker := NewDefaultTracker()

	if err := tracker.Flush(nil); err != nil {
		t.Errorf("flush with a nil store: %v", err)
	}
	if err := tracker.Restore(nil); err != nil {
		t.Errorf("restore with a nil store: %v", err)
	}
}

func TestSaveStateRejectsUnencodableValues(t *testing.T) {
	store := newMemoryStore()

	if err := SaveState(store, TrackerStateKey, make(chan int)); err == nil {
		t.Error("expected an encode error for an unsupported value")
	}
}

func TestLoadStateReportsBrokenJson(t *testing.T) {
	store := newMemoryStore()
	store.values[TrackerStateKey] = "{not json"

	var state State
	if err := LoadState(store, TrackerStateKey, &state); err == nil {
		t.Error("expected a decode error")
	}
}
