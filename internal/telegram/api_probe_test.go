package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/mymmrac/telego"
)

func TestAPIProbeTextRendersTheTable(t *testing.T) {
	b := setupTestBot(t)
	b.SetHealthSource(&fakeHealthSource{})

	text := b.apiProbeText([]apiprobe.Result{
		{Method: "GET", Path: "/api/info", Status: 200, Duration: 12 * time.Millisecond},
		{Method: "GET", Path: "/api/group/ИС-21", Status: 200, Duration: 820 * time.Millisecond},
		{Method: "GET", Path: "/api/teachers", Status: 500, Duration: 4 * time.Millisecond},
		{Method: "GET", Path: "/api/health", Duration: 3 * time.Millisecond, Err: "dial tcp 127.0.0.1:8080: connect: connection refused"},
	})

	if !strings.Contains(text, "-- Проверка API живыми запросами --") {
		t.Fatalf("header missing:\n%s", text)
	}
	if !strings.Contains(text, "<code>") || !strings.Contains(text, "</code>") {
		t.Errorf("the table must be monospaced:\n%s", text)
	}
	for _, want := range []string{
		"GET /api/info",
		"12 мс",
		"820 мс",
		"GET /api/group/ИС-21",
		"500",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("table misses %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "Итог: 2/4 отвечают, самая долгая — GET /api/group/ИС-21 — 820 мс") {
		t.Errorf("summary missing:\n%s", text)
	}
	if !strings.Contains(text, "GET /api/health          —     3 мс ⚠️") {
		t.Errorf("an unanswered endpoint must keep its place with the wait time:\n%s", text)
	}
	if !strings.Contains(text, "Проблемные: GET /api/teachers (500); GET /api/health: dial tcp") {
		t.Errorf("failures missing:\n%s", text)
	}
	if !strings.Contains(text, "медленнее 500 мс") {
		t.Errorf("the slow legend must name the threshold:\n%s", text)
	}
	if !strings.Contains(text, "🐌") || !strings.Contains(text, "⚠️") || !strings.Contains(text, "✅") {
		t.Errorf("status marks missing:\n%s", text)
	}

	block := text
	if start := strings.Index(text, "<code>"); start >= 0 {
		if end := strings.Index(text, "</code>"); end > start {
			block = text[start+len("<code>") : end]
		}
	}

	rows := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		for _, path := range []string{"/api/info", "/api/teachers", "/api/health"} {
			if strings.Contains(line, path) {
				rows[path] = strings.TrimSpace(line)
			}
		}
	}
	if !strings.HasSuffix(rows["/api/info"], "✅") {
		t.Errorf("a healthy endpoint must be marked as one: %q", rows["/api/info"])
	}
	if !strings.HasSuffix(rows["/api/teachers"], "⚠️") {
		t.Errorf("a failing endpoint must be marked as one: %q", rows["/api/teachers"])
	}
	if !strings.Contains(rows["/api/health"], "—") || !strings.HasSuffix(rows["/api/health"], "⚠️") {
		t.Errorf("an endpoint without an answer needs an empty status and a warning: %q", rows["/api/health"])
	}
}

func TestAPIProbeTextWithoutTargets(t *testing.T) {
	b := setupTestBot(t)

	if text := b.apiProbeText(nil); text != b.loc("api_probe_empty") {
		t.Errorf("text = %q", text)
	}
}

func TestAPIProbeTextMarksSlowEndpointsWithTheConfiguredThreshold(t *testing.T) {
	b := setupTestBot(t)
	b.SetHealthSource(&fakeHealthSource{slow: 100 * time.Millisecond})

	text := b.apiProbeText([]apiprobe.Result{
		{Method: "GET", Path: "/api/groups", Status: 200, Duration: 150 * time.Millisecond},
	})

	if !strings.Contains(text, "🐌") {
		t.Errorf("150ms against a 100ms threshold is slow:\n%s", text)
	}
	if !strings.Contains(text, "медленнее 100 мс") {
		t.Errorf("the legend must use the configured threshold:\n%s", text)
	}
}

func TestAPIProbeCallbackRunsTheProbeAndRecordsTheFix(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	incidents := health.NewIncidentLog(nil, 0)
	incidents.Record(health.AlertAPIErrors, "errors=20 window=5m0s")
	b.SetIncidentLog(incidents)

	done := make(chan struct{})
	b.SetAPIProbeFunc(func(context.Context) []apiprobe.Result {
		defer close(done)
		return []apiprobe.Result{{Method: "GET", Path: "/api/info", Status: 200, Duration: 5 * time.Millisecond}}
	})

	cb := &apiProbeCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.APIProbeCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the probe button never ran the probe")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(caller.deliveredText(), "Итог: 1/1 отвечают") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(caller.deliveredText(), "GET /api/info") {
		t.Fatalf("the table never reached the chat, messages: %v", caller.payloads())
	}
	if !strings.Contains(caller.deliveredText(), "Итог: 1/1 отвечают") {
		t.Fatalf("the summary never reached the chat, messages: %v", caller.payloads())
	}

	recent := incidents.Recent(1)
	if recent[0].Resolution != health.ResolutionManual {
		t.Errorf("a healthy probe must be recorded as the fix: %+v", recent[0])
	}
	want := b.locData("incident_fix_api", map[string]interface{}{"Healthy": 1, "Total": 1})
	if recent[0].Note != want {
		t.Errorf("note = %q, want %q", recent[0].Note, want)
	}
}

func TestAPIProbeCallbackKeepsFailuresOpen(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	incidents := health.NewIncidentLog(nil, 0)
	incidents.Record(health.AlertAPIErrors, "errors=20 window=5m0s")
	b.SetIncidentLog(incidents)

	done := make(chan struct{})
	b.SetAPIProbeFunc(func(context.Context) []apiprobe.Result {
		defer close(done)
		return []apiprobe.Result{{Method: "GET", Path: "/api/groups", Status: 500, Duration: 3 * time.Millisecond}}
	})

	cb := &apiProbeCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.APIProbeCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the probe button never ran the probe")
	}

	recent := incidents.Recent(1)
	if recent[0].Resolution != "" || recent[0].Note != "" {
		t.Errorf("a failing probe must not claim a fix: %+v", recent[0])
	}
}

func TestAPIProbeCallbackIsAdminOnly(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetAPIProbeFunc(func(context.Context) []apiprobe.Result { return nil })

	cb := &apiProbeCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   999,
		UserID:   999,
		Data:     notification.APIProbeCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 999}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Errorf("non-admin got %q", caller.last())
	}
}

func TestAPIProbeCallbackWithoutAProbe(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	cb := &apiProbeCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.APIProbeCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if !strings.Contains(caller.last(), b.loc("api_probe_unavailable")) {
		t.Errorf("message = %q", caller.last())
	}
}

func TestAPIProbeTargetsComeFromTheCache(t *testing.T) {
	raspCache, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	raspCache.SetGroups(map[string]any{"ИС-21": map[string]any{}}, "hash")

	targets := apiprobe.Targets(raspCache)
	found := false
	for _, target := range targets {
		if strings.HasPrefix(target.Path, "/api/group/") {
			found = true
		}
	}
	if !found {
		t.Errorf("targets = %+v", targets)
	}
}
