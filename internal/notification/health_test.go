package notification

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

type memoryStateStore struct {
	values map[string]string
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{values: make(map[string]string)}
}

func (s *memoryStateStore) LoadState(key string) (string, bool, error) {
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *memoryStateStore) SaveState(key, value string) error {
	s.values[key] = value
	return nil
}

func newTestHealthNotifier(t *testing.T, tracker *health.Tracker, cooldown time.Duration) (*HealthNotifier, *mockEventSender, *mockEventChatFinder) {
	t.Helper()

	sender := &mockEventSender{}
	finder := &mockEventChatFinder{admins: []*EventChat{{ID: 42, PeerID: 4242}}}
	notifier := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, cooldown, nil)
	return notifier, sender, finder
}

func failingTracker() *health.Tracker {
	thresholds := health.DefaultThresholds()
	thresholds.ParserFailures = 1
	tracker := health.NewTracker(thresholds)
	tracker.ParserFailure(errors.New("site down"))
	return tracker
}

func TestHealthNotifierAlertsAdmins(t *testing.T) {
	notifier, sender, _ := newTestHealthNotifier(t, failingTracker(), time.Minute)

	notifier.Check()

	if len(sender.sent) != 1 {
		t.Fatalf("expected one alert, got %+v", sender.sent)
	}
	if sender.sent[0].chatID != 4242 {
		t.Errorf("alert chat = %d, want the peer id", sender.sent[0].chatID)
	}
	if !strings.Contains(sender.sent[0].text, "Парсер расписания падает") {
		t.Errorf("alert text = %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[0].text, "consecutiveFailures=1") {
		t.Errorf("alert text should carry the detail: %q", sender.sent[0].text)
	}
}

func TestHealthNotifierParserAlertsCarryTheReparseButton(t *testing.T) {
	notifier, sender, _ := newTestHealthNotifier(t, failingTracker(), time.Minute)

	notifier.Check()

	buttoned := sender.buttoned()
	if len(buttoned) != 1 {
		t.Fatalf("expected the parser alert to carry a button, got %+v", sender.sent)
	}
	if len(buttoned[0].buttons) != 1 {
		t.Fatalf("expected a single button, got %+v", buttoned[0].buttons)
	}
	if buttoned[0].buttons[0].Data != ParserReparseCallback {
		t.Errorf("button data = %q, want %q", buttoned[0].buttons[0].Data, ParserReparseCallback)
	}
	if buttoned[0].buttons[0].Text != parserReparseButton {
		t.Errorf("button text = %q", buttoned[0].buttons[0].Text)
	}
}

func TestHealthNotifierCalendarAlertsCarryTheSyncButton(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.CalendarFailures = 1
	tracker := health.NewTracker(thresholds)
	tracker.CalendarFailure(errors.New("calendar down"))

	notifier, sender, _ := newTestHealthNotifier(t, tracker, time.Minute)
	notifier.Check()

	if len(sender.sent) != 1 {
		t.Fatalf("expected one calendar alert, got %+v", sender.sent)
	}
	buttoned := sender.buttoned()
	if len(buttoned) != 1 {
		t.Fatalf("expected the calendar alert to carry a button, got %+v", sender.sent)
	}
	if buttoned[0].buttons[0].Data != CalendarSyncCallback {
		t.Errorf("button data = %q, want %q", buttoned[0].buttons[0].Data, CalendarSyncCallback)
	}
	if buttoned[0].buttons[0].Text != calendarSyncButton {
		t.Errorf("button text = %q", buttoned[0].buttons[0].Text)
	}
}

func TestHealthNotifierAPIAlertsCarryTheProbeButton(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.APIErrors = 1
	tracker := health.NewTracker(thresholds)
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/health", Status: 500, Duration: time.Millisecond, Message: "database is locked"})

	notifier, sender, _ := newTestHealthNotifier(t, tracker, time.Minute)
	notifier.Check()

	if len(sender.sent) != 1 {
		t.Fatalf("expected one API alert, got %+v", sender.sent)
	}
	buttoned := sender.buttoned()
	if len(buttoned) != 1 || len(buttoned[0].buttons) != 1 {
		t.Fatalf("expected the API alert to carry a button, got %+v", sender.sent)
	}
	if buttoned[0].buttons[0].Data != APIProbeCallback {
		t.Errorf("button data = %q, want %q", buttoned[0].buttons[0].Data, APIProbeCallback)
	}
	if buttoned[0].buttons[0].Text != apiProbeButton {
		t.Errorf("button text = %q", buttoned[0].buttons[0].Text)
	}
	for _, want := range []string{"Ошибки HTTP API", "paths: GET /api/health x1 (500)", "last: 500 GET /api/health: database is locked"} {
		if !strings.Contains(sender.sent[0].text, want) {
			t.Errorf("API alert %q misses %q", sender.sent[0].text, want)
		}
	}
}

func TestHealthAlertButtonsMatchTheAlertScope(t *testing.T) {
	for _, key := range []string{
		health.AlertParserFailures,
		health.AlertParserStale,
		health.AlertParserLayout,
		health.AlertParserGuard,
	} {
		buttons := HealthAlertButtons(key)
		if len(buttons) != 1 {
			t.Errorf("%s: expected one button, got %+v", key, buttons)
			continue
		}
		if buttons[0].Data != ParserReparseCallback {
			t.Errorf("%s: button data = %q", key, buttons[0].Data)
		}
		if !IsParserAlert(key) {
			t.Errorf("%s: IsParserAlert = false", key)
		}
	}

	for _, key := range []string{health.AlertCalendarFailures, health.AlertCalendarStale} {
		buttons := HealthAlertButtons(key)
		if len(buttons) != 1 {
			t.Errorf("%s: expected the sync button, got %+v", key, buttons)
			continue
		}
		if buttons[0].Data != CalendarSyncCallback {
			t.Errorf("%s: button data = %q", key, buttons[0].Data)
		}
		if !IsCalendarAlert(key) {
			t.Errorf("%s: IsCalendarAlert = false", key)
		}
	}

	buttons := HealthAlertButtons(health.AlertAPIErrors)
	if len(buttons) != 1 || buttons[0].Data != APIProbeCallback {
		t.Errorf("API alerts must carry the probe button, got %+v", buttons)
	}
	if !IsAPIAlert(health.AlertAPIErrors) {
		t.Error("the api_errors alert must belong to the API scope")
	}
	if IsParserAlert(health.AlertCalendarFailures) || IsCalendarAlert(health.AlertParserGuard) || IsAPIAlert(health.AlertParserFailures) {
		t.Error("parser, calendar and API scopes must stay apart")
	}
}

func TestHealthNotifierReportsTheParserGuard(t *testing.T) {
	tracker := health.NewDefaultTracker()
	tracker.ParserReport("groups", nil, []health.GuardIssue{{
		Source: "groups",
		Reason: "shrink",
		Detail: "35 -> 6 (dropped 82%, limit 80%)",
	}})

	notifier, sender, _ := newTestHealthNotifier(t, tracker, time.Minute)
	notifier.Check()

	if len(sender.sent) != 1 {
		t.Fatalf("expected one guard alert, got %+v", sender.sent)
	}
	if !strings.Contains(sender.sent[0].text, "Парсер резко потерял данные") {
		t.Errorf("alert text = %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[0].text, "groups: shrink: 35 -> 6") {
		t.Errorf("alert must carry the counts: %q", sender.sent[0].text)
	}

	tracker.ParserReport("groups", nil, nil)
	notifier.Check()

	if len(sender.sent) != 2 {
		t.Fatalf("expected a recovery message, got %+v", sender.sent)
	}
	if !strings.Contains(sender.sent[1].text, "восстановлено") {
		t.Errorf("recovery text = %q", sender.sent[1].text)
	}
}

func TestHealthNotifierRespectsCooldown(t *testing.T) {
	notifier, sender, _ := newTestHealthNotifier(t, failingTracker(), time.Hour)

	notifier.Check()
	notifier.Check()

	if len(sender.sent) != 1 {
		t.Fatalf("expected the second check to be muted by the cooldown, got %+v", sender.sent)
	}

	notifier.cooldown = 0
	notifier.Check()
	if len(sender.sent) != 2 {
		t.Fatalf("expected a repeated alert once the cooldown expires, got %+v", sender.sent)
	}
}

func TestHealthNotifierSendsRecovery(t *testing.T) {
	tracker := failingTracker()
	notifier, sender, _ := newTestHealthNotifier(t, tracker, time.Minute)

	notifier.Check()
	tracker.ParserSuccess(time.Millisecond)
	notifier.Check()

	if len(sender.sent) != 2 {
		t.Fatalf("expected an alert and a recovery, got %+v", sender.sent)
	}
	if !strings.Contains(sender.sent[1].text, "восстановлено") {
		t.Errorf("recovery text = %q", sender.sent[1].text)
	}

	notifier.Check()
	if len(sender.sent) != 2 {
		t.Errorf("recovery should be sent once, got %+v", sender.sent)
	}
}

func TestHealthNotifierIgnoresHealthyTracker(t *testing.T) {
	notifier, sender, _ := newTestHealthNotifier(t, health.NewDefaultTracker(), time.Minute)

	notifier.Check()

	if len(sender.sent) != 0 {
		t.Errorf("healthy tracker should not notify, got %+v", sender.sent)
	}
}

func TestHealthNotifierCooldownSurvivesRestart(t *testing.T) {
	store := newMemoryStateStore()
	tracker := failingTracker()
	sender := &mockEventSender{}
	finder := &mockEventChatFinder{admins: []*EventChat{{ID: 42, PeerID: 4242}}}

	first := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, time.Hour, store)
	first.Check()
	if len(sender.sent) != 1 {
		t.Fatalf("expected the first alert, got %+v", sender.sent)
	}

	restarted := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, time.Hour, store)
	restarted.Check()
	if len(sender.sent) != 1 {
		t.Errorf("the cooldown must survive a restart, got %+v", sender.sent)
	}
}

func TestHealthNotifierSendsRecoveryAfterRestart(t *testing.T) {
	store := newMemoryStateStore()
	tracker := failingTracker()
	sender := &mockEventSender{}
	finder := &mockEventChatFinder{admins: []*EventChat{{ID: 42, PeerID: 4242}}}

	first := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, time.Hour, store)
	first.Check()

	tracker.ParserSuccess(time.Millisecond)

	restarted := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, time.Hour, store)
	restarted.Check()

	if len(sender.sent) != 2 {
		t.Fatalf("expected an alert and a recovery, got %+v", sender.sent)
	}
	if !strings.Contains(sender.sent[1].text, "восстановлено") {
		t.Errorf("recovery text = %q", sender.sent[1].text)
	}

	restarted.Check()
	if len(sender.sent) != 2 {
		t.Errorf("a recovered alert must not be reported twice, got %+v", sender.sent)
	}
}

func TestHealthNotifierToleratesBrokenStoredState(t *testing.T) {
	store := newMemoryStateStore()
	store.values[health.AlertsStateKey] = "{broken"

	notifier, sender, _ := newTestHealthNotifier(t, failingTracker(), time.Minute)
	notifier.store = store

	notifier.Check()
	if len(sender.sent) != 1 {
		t.Errorf("a broken stored state must not stop alerts, got %+v", sender.sent)
	}
}

func TestHealthNotifierWithoutAdmins(t *testing.T) {
	notifier, sender, finder := newTestHealthNotifier(t, failingTracker(), time.Minute)
	finder.admins = nil

	notifier.Check()

	if len(sender.sent) != 0 {
		t.Errorf("no admins means no messages, got %+v", sender.sent)
	}
}

func TestHealthNotifierRecordsIncidents(t *testing.T) {
	store := newMemoryStateStore()
	incidents := health.NewIncidentLog(store, 0)
	tracker := failingTracker()

	notifier, _, _ := newTestHealthNotifier(t, tracker, time.Minute)
	notifier.SetIncidents(incidents)

	notifier.Check()
	open := incidents.Open()
	if len(open) != 1 || open[0].Key != health.AlertParserFailures {
		t.Fatalf("open incidents = %+v", open)
	}
	if open[0].Detail == "" {
		t.Error("the incident must carry the alert detail")
	}

	tracker.ParserSuccess(time.Millisecond)
	notifier.Check()

	restarted := health.NewIncidentLog(store, 0)
	recent := restarted.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("records = %+v", recent)
	}
	if recent[0].Open() || recent[0].Resolution != health.ResolutionAuto {
		t.Errorf("a recovery must close the incident by itself: %+v", recent[0])
	}
}

func TestHealthNotifierKeepsAManualFixInTheHistory(t *testing.T) {
	incidents := health.NewIncidentLog(nil, 0)
	tracker := failingTracker()

	notifier, _, _ := newTestHealthNotifier(t, tracker, time.Minute)
	notifier.SetIncidents(incidents)
	notifier.Check()

	incidents.MarkManual(health.ScopeParser, "переразбор вручную")

	tracker.ParserSuccess(time.Millisecond)
	notifier.Check()

	recent := incidents.Recent(1)
	if recent[0].Resolution != health.ResolutionManual || recent[0].Note != "переразбор вручную" {
		t.Errorf("the manual fix must survive the recovery: %+v", recent[0])
	}
}

func TestHealthNotifierToleratesAMissingIncidentLog(t *testing.T) {
	notifier, sender, _ := newTestHealthNotifier(t, failingTracker(), time.Minute)

	notifier.Check()

	if len(sender.sent) != 1 {
		t.Errorf("alerts must work without an incident log, got %+v", sender.sent)
	}
}

func TestHealthCheckMinutesDefaults(t *testing.T) {
	if got := healthCheckMinutes(nil); got != 1 {
		t.Errorf("nil config = %d, want 1", got)
	}
	if got := healthCheckMinutes(&config.HealthConfig{}); got != 1 {
		t.Errorf("zero minutes = %d, want 1", got)
	}
	if got := healthCheckMinutes(&config.HealthConfig{CheckMinutes: 5}); got != 5 {
		t.Errorf("configured minutes = %d, want 5", got)
	}
}

func TestSchedulerRegistersHealthCheck(t *testing.T) {
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Timetable.Weekdays = [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}
	s := NewScheduler(cfg, c, logger.New("error", nil), &mockEventSender{}, &mockEventChatFinder{}, health.NewDefaultTracker(), nil)
	s.Start()
	defer s.Stop()

	if entries := s.cron.Entries(); len(entries) != 2 {
		t.Errorf("expected a slot plus a health entry, got %d", len(entries))
	}
	if s.health == nil {
		t.Error("health notifier should be wired")
	}

	disabled := &config.HealthConfig{Disabled: true}
	cfg.Health = disabled
	offline := NewScheduler(cfg, c, logger.New("error", nil), &mockEventSender{}, &mockEventChatFinder{}, health.NewDefaultTracker(), nil)
	offline.Start()
	defer offline.Stop()
	if offline.health != nil {
		t.Error("disabled health checks should not schedule anything")
	}
}
