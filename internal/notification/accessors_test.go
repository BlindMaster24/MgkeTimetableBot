package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
)

func TestSetNowDrivesTheNotifierClock(t *testing.T) {
	n := &EventNotifier{}
	if got := n.nowTime(); got.IsZero() {
		t.Fatal("without a clock the notifier must fall back to the site clock")
	}

	moment := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	n.SetNow(func() time.Time { return moment })
	if got := n.nowTime(); !got.Equal(moment) {
		t.Fatalf("nowTime = %s, want the injected %s", got, moment)
	}
}

func TestKindLetterMarksTeachers(t *testing.T) {
	if got := kindLetter("teachers"); got != "t" {
		t.Fatalf("kindLetter(teachers) = %q, want t", got)
	}
	if got := kindLetter("groups"); got != "g" {
		t.Fatalf("kindLetter(groups) = %q, want g", got)
	}
}

func TestHealthAlertTitleCoversEveryAlert(t *testing.T) {
	seen := map[string]bool{}
	for _, key := range health.AlertKeys {
		title := HealthAlertTitle(key, func(localeKey string) string { return "title:" + localeKey })
		if !strings.HasPrefix(title, "title:health_title_") {
			t.Fatalf("alert %q got title %q, want a localised health_title_* key", key, title)
		}
		if seen[title] {
			t.Fatalf("alert %q reuses the title %q", key, title)
		}
		seen[title] = true
	}
	// An unmapped key comes back as itself: the alert is unknown to the
	// notifier, and a machine name is more useful to an operator than a
	// generic "something happened".
	if got := HealthAlertTitle("not_an_alert", func(string) string { return "fallback" }); got != "not_an_alert" {
		t.Fatalf("an unknown alert = %q, want the key itself", got)
	}
}
