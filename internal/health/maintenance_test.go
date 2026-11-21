package health

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestArchiveBusyAndMaintenanceCountersReachTheSnapshot(t *testing.T) {
	tracker := NewTracker(DefaultThresholds())

	tracker.ArchiveBusyRetry()
	tracker.ArchiveBusyRetry()
	tracker.ArchiveBusyFailure()
	tracker.MaintenanceSuccess()
	tracker.MaintenanceFailure(errors.New("disk full"))

	snapshot := tracker.Snapshot()
	if snapshot.Archive.BusyRetries != 2 || snapshot.Archive.BusyFailures != 1 {
		t.Fatalf("archive counters = %+v, want 2 retries and 1 failure", snapshot.Archive)
	}
	if snapshot.Maintenance.Runs != 2 || snapshot.Maintenance.Failures != 1 {
		t.Fatalf("maintenance counters = %+v, want 2 runs and 1 failure", snapshot.Maintenance)
	}
	if snapshot.Maintenance.LastError != "disk full" {
		t.Fatalf("last error = %q, want the failure text", snapshot.Maintenance.LastError)
	}
	if snapshot.Maintenance.LastSuccessUnix == 0 {
		t.Fatal("a successful run must leave a timestamp")
	}

	tracker.MaintenanceSuccess()
	if after := tracker.Snapshot(); after.Maintenance.Failures != 0 {
		t.Fatalf("a success must reset the failure streak, got %d", after.Maintenance.Failures)
	}
}

func TestMaintenanceDetailBoundsTheError(t *testing.T) {
	detail := maintenanceDetail(3, 5, "  line one\nline two  ")
	if !strings.Contains(detail, "consecutiveFailures=3 threshold=5") {
		t.Fatalf("detail = %q, want the counters", detail)
	}
	if !strings.Contains(detail, "error=line one line two") {
		t.Fatalf("detail = %q, want the error on one line", detail)
	}

	long := maintenanceDetail(1, 2, strings.Repeat("x", 300))
	if len(long) > 200 || !strings.HasSuffix(long, strings.Repeat("x", 160)) {
		t.Fatalf("a long error must be truncated to 160 chars, got %d", len(long))
	}
	if got := maintenanceDetail(1, 2, "   "); strings.Contains(got, "error=") {
		t.Fatalf("a blank error must not be printed, got %q", got)
	}
}

func TestSlowThresholdFollowsTheConfig(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.APISlow = 750 * time.Millisecond
	tracker := NewTracker(thresholds)

	if got := tracker.SlowThreshold(); got != 750*time.Millisecond {
		t.Fatalf("SlowThreshold = %s, want the configured value", got)
	}
}
