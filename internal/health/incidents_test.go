package health

import (
	"testing"
	"time"
)

func TestIncidentLogRecordsStartAndAutoRecovery(t *testing.T) {
	log := NewIncidentLog(nil, 0)

	log.Record(AlertParserLayout, "missing groups: .table")
	log.Record(AlertParserLayout, "missing groups: .table")

	open := log.Open()
	if len(open) != 1 {
		t.Fatalf("open incidents = %d, want 1", len(open))
	}
	if open[0].Scope != ScopeParser {
		t.Errorf("scope = %q, want %q", open[0].Scope, ScopeParser)
	}
	if !open[0].Open() {
		t.Error("a freshly recorded incident must stay open")
	}

	log.Resolve(AlertParserLayout)

	recent := log.Recent(5)
	if len(recent) != 1 {
		t.Fatalf("records = %d, want 1", len(recent))
	}
	if recent[0].Open() {
		t.Error("resolve must close the incident")
	}
	if recent[0].Resolution != ResolutionAuto {
		t.Errorf("resolution = %q, want %q", recent[0].Resolution, ResolutionAuto)
	}
	if len(log.Open()) != 0 {
		t.Error("no incident must stay open after a recovery")
	}
}

func TestStartupStampSpotsWhatChanged(t *testing.T) {
	deployed := StartupStamp{Build: "1.0.0 (aaaaaaa)", Config: "11111111"}

	cases := []struct {
		name     string
		previous StartupStamp
		current  StartupStamp
		want     StartupChange
	}{
		{"same build and config", deployed, deployed, ChangeRestart},
		{"new build", deployed, StartupStamp{Build: "1.1.0 (bbbbbbb)", Config: "11111111"}, ChangeDeploy},
		{"edited config", deployed, StartupStamp{Build: "1.0.0 (aaaaaaa)", Config: "22222222"}, ChangeConfig},
		{"only the config was known before", StartupStamp{Config: "11111111"}, deployed, ChangeRestart},
		{"nothing was known before", StartupStamp{}, deployed, ChangeRestart},
	}

	for _, tc := range cases {
		if got := tc.current.ChangeFrom(tc.previous); got != tc.want {
			t.Errorf("%s: change = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIncidentLogNotesWhatChangedAtStartup(t *testing.T) {
	log := NewIncidentLog(nil, 0)
	log.SetStartupStamp(StartupStamp{Build: "1.0.0 (aaaaaaa, 2026-09-01)", Config: "11111111"})

	log.Record(AlertAPIErrors, "errors=20 window=5m0s")
	log.Record(AlertParserGuard, "groups: shrink: 35 -> 3")

	current := StartupStamp{Build: "1.1.0 (bbbbbbb, 2026-09-13)", Config: "11111111"}
	var seen []StartupChange
	annotated := log.NoteStartup(ScopeAPI, current, func(change StartupChange, previous, current StartupStamp) string {
		seen = append(seen, change)
		return previous.Build + " -> " + current.Build
	})
	if len(annotated) != 1 {
		t.Fatalf("annotated = %+v", annotated)
	}
	if len(seen) != 1 || seen[0] != ChangeDeploy {
		t.Errorf("a new build must be recorded as a deploy: %v", seen)
	}
	if annotated[0].Note != "1.0.0 (aaaaaaa, 2026-09-01) -> 1.1.0 (bbbbbbb, 2026-09-13)" {
		t.Errorf("note = %q", annotated[0].Note)
	}

	records := log.Recent(5)
	for _, record := range records {
		switch record.Key {
		case AlertAPIErrors:
			if record.Resolution != ResolutionManual {
				t.Errorf("a deploy must be recorded as a manual fix: %+v", record)
			}
			if record.Build != current.Build || record.Config != current.Config {
				t.Errorf("the record must follow the running stamp: %+v", record)
			}
		case AlertParserGuard:
			if record.Resolution != "" || record.Note != "" {
				t.Errorf("other scopes must stay untouched: %+v", record)
			}
		}
	}

	log.Resolve(AlertAPIErrors)
	recent := log.Recent(2)
	if recent[1].Resolution != ResolutionManual {
		t.Errorf("a recovery must not overwrite a manual fix: %+v", recent[1])
	}
	if log.NoteStartup(ScopeAPI, current, func(StartupChange, StartupStamp, StartupStamp) string { return "again" }) != nil {
		t.Error("a closed record must not be annotated again")
	}
}

func TestIncidentLogKeepsTheManualFix(t *testing.T) {
	log := NewIncidentLog(nil, 0)

	log.Record(AlertCalendarFailures, "consecutiveFailures=3 threshold=3")
	if !log.MarkManual(ScopeCalendar, "ручная синхронизация") {
		t.Fatal("an open calendar incident must accept a manual marker")
	}

	log.Resolve(AlertCalendarFailures)

	recent := log.Recent(1)
	if recent[0].Resolution != ResolutionManual {
		t.Errorf("resolution = %q, want %q", recent[0].Resolution, ResolutionManual)
	}
	if recent[0].Note != "ручная синхронизация" {
		t.Errorf("note = %q", recent[0].Note)
	}
}

func TestIncidentLogIgnoresManualMarkersWithoutAnIncident(t *testing.T) {
	log := NewIncidentLog(nil, 0)

	if log.MarkManual(ScopeParser, "переразбор") {
		t.Error("a manual action without an open incident must not be recorded")
	}
	if len(log.Recent(0)) != 0 {
		t.Error("no incident must be invented for a healthy parser")
	}
}

func TestIncidentLogScopesAlerts(t *testing.T) {
	cases := map[string]string{
		AlertParserFailures:   ScopeParser,
		AlertParserStale:      ScopeParser,
		AlertParserLayout:     ScopeParser,
		AlertParserGuard:      ScopeParser,
		AlertCalendarFailures: ScopeCalendar,
		AlertCalendarStale:    ScopeCalendar,
		AlertAPIErrors:        ScopeAPI,
		"unknown":             "",
	}

	for key, want := range cases {
		if got := AlertScope(key); got != want {
			t.Errorf("AlertScope(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestIncidentLogSurvivesARestart(t *testing.T) {
	store := newMemoryStore()
	log := NewIncidentLog(store, 0)

	log.Record(AlertParserGuard, "groups: shrink: 35 -> 3")
	log.MarkManual(ScopeParser, "переразбор вручную")
	log.Resolve(AlertParserGuard)
	log.Record(AlertCalendarStale, "lag=7h0m0s threshold=6h0m0s")

	restarted := NewIncidentLog(store, 0)
	recent := restarted.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("records after a restart = %d, want 2", len(recent))
	}
	if recent[0].Key != AlertCalendarStale || !recent[0].Open() {
		t.Errorf("newest record = %+v", recent[0])
	}
	if recent[1].Resolution != ResolutionManual || recent[1].Note != "переразбор вручную" {
		t.Errorf("closed record = %+v", recent[1])
	}
	if open := restarted.Open(); len(open) != 1 || open[0].Key != AlertCalendarStale {
		t.Errorf("open incidents after a restart = %+v", open)
	}
}

func TestIncidentLogTrimsToTheLimit(t *testing.T) {
	log := NewIncidentLog(nil, 3)

	for _, key := range []string{AlertParserStale, AlertParserFailures, AlertParserLayout, AlertParserGuard} {
		log.Record(key, "detail")
		log.Resolve(key)
	}

	recent := log.Recent(10)
	if len(recent) != 3 {
		t.Fatalf("records = %d, want 3", len(recent))
	}
	if recent[0].Key != AlertParserGuard || recent[2].Key != AlertParserFailures {
		t.Errorf("newest first order lost: %+v", recent)
	}
}

func TestIncidentDurationStopsAtTheEnd(t *testing.T) {
	started := time.Now().Add(-time.Hour)
	closed := Incident{StartedAt: started, EndedAt: started.Add(10 * time.Minute)}
	if got := closed.Duration(time.Now()); got != 10*time.Minute {
		t.Errorf("closed duration = %s, want 10m", got)
	}

	open := Incident{StartedAt: started}
	if got := open.Duration(started.Add(5 * time.Minute)); got != 5*time.Minute {
		t.Errorf("open duration = %s, want 5m", got)
	}
	if got := open.Duration(started.Add(-time.Minute)); got != 0 {
		t.Errorf("duration before the start = %s, want 0", got)
	}
}

func TestIncidentLogHandlesNilReceiver(t *testing.T) {
	var log *IncidentLog

	log.Record(AlertParserStale, "detail")
	log.Resolve(AlertParserStale)
	if log.MarkManual(ScopeParser, "note") {
		t.Error("a nil log must not report a manual marker")
	}
	if log.Recent(5) != nil || log.Open() != nil {
		t.Error("a nil log must read as empty")
	}
}
