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

func newTestHealthNotifier(t *testing.T, tracker *health.Tracker, cooldown time.Duration) (*HealthNotifier, *mockEventSender, *mockEventChatFinder) {
	t.Helper()

	sender := &mockEventSender{}
	finder := &mockEventChatFinder{admins: []*EventChat{{ID: 42, PeerID: 4242}}}
	notifier := NewHealthNotifier(tracker, logger.New("error", nil), sender, finder, cooldown)
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

func TestHealthNotifierWithoutAdmins(t *testing.T) {
	notifier, sender, finder := newTestHealthNotifier(t, failingTracker(), time.Minute)
	finder.admins = nil

	notifier.Check()

	if len(sender.sent) != 0 {
		t.Errorf("no admins means no messages, got %+v", sender.sent)
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
	s := NewScheduler(cfg, c, logger.New("error", nil), &mockEventSender{}, &mockEventChatFinder{}, health.NewDefaultTracker())
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
	offline := NewScheduler(cfg, c, logger.New("error", nil), &mockEventSender{}, &mockEventChatFinder{}, health.NewDefaultTracker())
	offline.Start()
	defer offline.Stop()
	if offline.health != nil {
		t.Error("disabled health checks should not schedule anything")
	}
}
