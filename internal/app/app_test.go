package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	parserpkg "github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func TestHealthThresholdsCarryTheParserGuardLimit(t *testing.T) {
	cfg := &config.Config{Health: &config.HealthConfig{
		ParserGuardFailures:  4,
		ParserLayoutFailures: 3,
		ParserFailures:       2,
	}}

	thresholds := healthThresholds(cfg)
	if thresholds.ParserGuard != 4 {
		t.Errorf("guard threshold = %d, want 4", thresholds.ParserGuard)
	}
	if thresholds.ParserLayout != 3 || thresholds.ParserFailures != 2 {
		t.Errorf("other thresholds = %+v", thresholds)
	}

	fallback := healthThresholds(&config.Config{})
	if fallback.ParserGuard != health.DefaultThresholds().ParserGuard {
		t.Errorf("without a health section the default guard threshold applies, got %d", fallback.ParserGuard)
	}
}

func TestAPISlowThresholdComesFromTheConfig(t *testing.T) {
	cfg := &config.Config{Health: &config.HealthConfig{APISlowMS: 250}}
	if got := healthThresholds(cfg).APISlow; got != 250*time.Millisecond {
		t.Errorf("slow threshold = %s, want 250ms", got)
	}

	fallback := healthThresholds(&config.Config{})
	if fallback.APISlow != health.DefaultThresholds().APISlow {
		t.Errorf("without a health section the default slow threshold applies, got %s", fallback.APISlow)
	}
}

func TestAPIProbeBaseURLFollowsTheConfiguredPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.Port = 8081

	if got := apiProbeBaseURL(cfg); got != "http://127.0.0.1:8081" {
		t.Errorf("probe base url = %q", got)
	}
}

func TestAPIAlertReachesTheAdminsWithTheProbeButton(t *testing.T) {
	thresholds := health.DefaultThresholds()
	thresholds.APIErrors = 2
	thresholds.APISlow = 500 * time.Millisecond
	tracker := health.NewTracker(thresholds)
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/groups", Status: 500, Duration: 600 * time.Millisecond, Message: "boom"})
	tracker.RecordAPI(health.APIRequest{Method: "GET", Path: "/api/groups", Status: 500, Duration: 700 * time.Millisecond, Message: "boom"})

	sender := &recordingSender{}
	incidents := health.NewIncidentLog(nil, 0)
	notifier := notification.NewHealthNotifier(tracker, logger.New("error", nil), sender, adminFinder{}, time.Minute, nil)
	notifier.SetIncidents(incidents)

	notifier.Check()

	delivered := sender.delivered()
	if len(delivered) != 1 {
		t.Fatalf("expected one admin alert, got %+v", delivered)
	}
	if len(delivered[0].buttons) != 1 || delivered[0].buttons[0].Data != notification.APIProbeCallback {
		t.Fatalf("the API alert must offer the live probe, got %+v", delivered[0].buttons)
	}
	for _, want := range []string{
		"Ошибки HTTP API",
		"paths: GET /api/groups x2 (500)",
		"slow: GET /api/groups avg=650ms max=700ms n=2 (last 700ms)",
	} {
		if !strings.Contains(delivered[0].text, want) {
			t.Errorf("API alert %q misses %q", delivered[0].text, want)
		}
	}

	open := incidents.Open()
	if len(open) != 1 || open[0].Key != health.AlertAPIErrors {
		t.Fatalf("the alert must enter the incident history, got %+v", open)
	}
}

func TestGuardIssuesReportEveryKeptCache(t *testing.T) {
	report := parserpkg.Report{
		Source: parserpkg.SourceGroups,
		Keep: &parserpkg.Keep{
			Reason:       parserpkg.KeepReasonShrink,
			Previous:     35,
			Current:      6,
			LimitPercent: 80,
		},
	}

	issues := guardIssues(report)
	if len(issues) != 1 {
		t.Fatalf("issues = %+v", issues)
	}
	if issues[0].Source != parserpkg.SourceGroups || issues[0].Reason != parserpkg.KeepReasonShrink {
		t.Errorf("issue = %+v", issues[0])
	}
	if !strings.Contains(issues[0].Detail, "35 -> 6") {
		t.Errorf("issue detail = %q", issues[0].Detail)
	}
}

func TestGuardIssuesReportFallbacks(t *testing.T) {
	report := parserpkg.Report{
		Source:    parserpkg.SourceCalls,
		Fallbacks: []string{"bell schedule read from the page text"},
	}

	issues := guardIssues(report)
	if len(issues) != 1 || issues[0].Reason != "fallback" {
		t.Fatalf("issues = %+v", issues)
	}
	if issues[0].Detail != "bell schedule read from the page text" {
		t.Errorf("issue detail = %q", issues[0].Detail)
	}
}

func TestGuardIssuesStayEmptyForAHealthyReport(t *testing.T) {
	if issues := guardIssues(parserpkg.Report{Source: parserpkg.SourceTeachers}); len(issues) != 0 {
		t.Errorf("a healthy report must not raise a guard issue: %+v", issues)
	}
}

func TestParserGuardComesFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Parser.Guard = config.GuardConfig{Disabled: true, MinItems: 30, MaxDropPercent: 60}

	guard := parserGuard(cfg)
	if !guard.Disabled || guard.MinItems != 30 || guard.MaxDropPercent != 60 {
		t.Errorf("guard = %+v", guard)
	}

	resolved := guard.WithDefaults()
	if !resolved.Disabled || resolved.MinItems != 30 || resolved.MaxDropPercent != 60 {
		t.Errorf("resolved guard = %+v", resolved)
	}

	defaults := parserGuard(&config.Config{}).WithDefaults()
	if defaults.Disabled || defaults.MinItems != 10 || defaults.MaxDropPercent != 80 {
		t.Errorf("a guard without configured limits must fall back to the defaults: %+v", defaults)
	}
}

func TestResolveConfigPathPrefersTheFlag(t *testing.T) {
	t.Setenv("CONFIG_PATH", "env.yaml")
	if got := ResolveConfigPath("custom.yaml"); got != "custom.yaml" {
		t.Errorf("path = %q, want custom.yaml", got)
	}
	if got := ResolveConfigPath(""); got != "env.yaml" {
		t.Errorf("path = %q, want env.yaml", got)
	}
	t.Setenv("CONFIG_PATH", "")
	if got := ResolveConfigPath(""); got != "configs/config.yaml" {
		t.Errorf("path = %q, want the default", got)
	}
}

type recordingMessage struct {
	text    string
	buttons []notification.KeyboardButton
}

type recordingSender struct {
	mu   sync.Mutex
	sent []recordingMessage
}

func (s *recordingSender) SendText(chatID int64, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, recordingMessage{text: text})
	return nil
}

func (s *recordingSender) SendTextWithButtons(chatID int64, text string, buttons []notification.KeyboardButton) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, recordingMessage{text: text, buttons: buttons})
	return nil
}

func (s *recordingSender) delivered() []recordingMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]recordingMessage(nil), s.sent...)
}

func (s *recordingSender) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, 0, len(s.sent))
	for _, message := range s.sent {
		out = append(out, message.text)
	}
	return out
}

type adminFinder struct{}

func (adminFinder) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindChatsWithNotice(service string, notice string) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindChatsWithNoticeIgnoringFilter(service string, notice string) ([]*notification.EventChat, error) {
	return nil, nil
}

func (adminFinder) FindAdminChats(service string) ([]*notification.EventChat, error) {
	return []*notification.EventChat{{ID: 1, PeerID: 100}}, nil
}

func entriesPage(label string, count int) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="entry"><div class="content">`)
	for i := 0; i < count; i++ {
		switch label {
		case "Группа":
			fmt.Fprintf(&b, `<h2>Группа - %d</h2>`, 100+i)
		default:
			fmt.Fprintf(&b, `<h2>Преподаватель - Иванов И.И. %d</h2>`, i)
		}
		b.WriteString(`<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td></tr>
</table>`)
	}
	b.WriteString(`</div></div></body></html>`)
	return b.String()
}

func TestGuardTripReachesTheAdminChatsEndToEnd(t *testing.T) {
	var page struct {
		sync.Mutex
		groups   int
		teachers int
	}
	page.groups = 35
	page.teachers = 35

	serve := func(label string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			page.Lock()
			count := page.groups
			if label == "Преподаватель" {
				count = page.teachers
			}
			page.Unlock()

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(entriesPage(label, count)))
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "teacher") {
			serve("Преподаватель")(w, r)
			return
		}
		serve("Группа")(w, r)
	}))
	defer srv.Close()

	cfg := &config.Config{Health: &config.HealthConfig{ParserGuardFailures: 1}}
	cfg.Parser.Guard = config.GuardConfig{MinItems: 10, MaxDropPercent: 80}

	raspper := builderCache(t)
	tracker := health.NewTracker(healthThresholds(cfg))
	fetcher := parserpkg.NewFetcher(logger.New("error", nil), raspper, parserpkg.Options{
		Guard: parserGuard(cfg),
		OnReport: func(report parserpkg.Report) {
			tracker.ParserReport(report.Source, parserLayoutIssues(report), guardIssues(report))
		},
	})

	sender := &recordingSender{}
	incidents := health.NewIncidentLog(nil, 0)
	notifier := notification.NewHealthNotifier(tracker, logger.New("error", nil), sender, adminFinder{}, time.Minute, nil)
	notifier.SetIncidents(incidents)

	if err := fetcher.Timetable(t.Context(), srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	if len(raspper.GetGroups()) != 35 || len(raspper.GetTeachers()) != 35 {
		t.Fatalf("the first parse must seed the cache: %d groups, %d teachers", len(raspper.GetGroups()), len(raspper.GetTeachers()))
	}

	notifier.Check()
	if messages := sender.messages(); len(messages) != 0 {
		t.Fatalf("a healthy parse must stay silent, got %+v", messages)
	}

	page.Lock()
	page.groups = 3
	page.teachers = 3
	page.Unlock()

	if err := fetcher.Timetable(t.Context(), srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	if len(raspper.GetGroups()) != 35 || len(raspper.GetTeachers()) != 35 {
		t.Errorf("the guard must keep the cache: %d groups, %d teachers", len(raspper.GetGroups()), len(raspper.GetTeachers()))
	}

	notifier.Check()
	messages := sender.messages()
	if len(messages) != 1 {
		t.Fatalf("expected one admin alert, got %+v", messages)
	}
	if !strings.Contains(messages[0], "Парсер резко потерял данные") {
		t.Errorf("alert text = %q", messages[0])
	}
	if !strings.Contains(messages[0], "35 -> 3") {
		t.Errorf("alert must carry the counts: %q", messages[0])
	}

	delivered := sender.delivered()
	if len(delivered[0].buttons) != 1 {
		t.Fatalf("the alert must offer one action button, got %+v", delivered[0].buttons)
	}
	if delivered[0].buttons[0].Data != notification.ParserReparseCallback {
		t.Errorf("alert button = %+v, want the reparse callback", delivered[0].buttons[0])
	}

	open := incidents.Open()
	if len(open) != 1 || open[0].Key != health.AlertParserGuard {
		t.Fatalf("the trip must enter the incident history, got %+v", open)
	}
	if !strings.Contains(open[0].Detail, "35 -> 3") {
		t.Errorf("the incident must keep the counts: %q", open[0].Detail)
	}

	page.Lock()
	page.groups = 35
	page.teachers = 35
	page.Unlock()

	if err := fetcher.Timetable(t.Context(), srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	notifier.Check()

	records := incidents.Recent(1)
	if len(records) != 1 || records[0].Open() {
		t.Fatalf("a recovered parser must close the incident: %+v", records)
	}
	if records[0].Resolution != health.ResolutionAuto {
		t.Errorf("resolution = %q, want %q", records[0].Resolution, health.ResolutionAuto)
	}
}

func TestRunRejectsAMissingConfigFile(t *testing.T) {
	if code := Run(context.Background(), filepath.Join(t.TempDir(), "absent.yaml"), build.New("", "", "")); code != 1 {
		t.Errorf("Run() = %d, want 1 for a missing config file", code)
	}
}

func TestRunRejectsAnInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  port: 0\ntelegram:\n  token: \"x\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if code := Run(context.Background(), path, build.New("", "", "")); code != 1 {
		t.Errorf("Run() = %d, want 1 for an invalid config", code)
	}
}

func builderCache(t *testing.T) *cache.RaspCache {
	t.Helper()

	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return c
}

type mutablePage struct {
	mu   sync.Mutex
	html string
}

func (p *mutablePage) set(html string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.html = html
}

func (p *mutablePage) serve(w http.ResponseWriter) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(p.html))
}

const undatedGroupsPage = `<html><body><div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th><th>Вторник</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td><td>Физика<br>(Лек)<br>Петров П.П.<br>2-101</td></tr>
</table>
</div></div></body></html>`

const datedGroupsPage = `<html><body><div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th><th>Вторник, 08.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td><td>Физика<br>(Лек)<br>Петров П.П.<br>2-101</td></tr>
</table>
</div></div></body></html>`

const undatedCallsPage = `<html><body><div class="entry"><div class="content">
<h2>Расписание звонков</h2>
<table>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br />10.45 &ndash; 11.30</td></tr>
</table>
</div></div></body></html>`

const datedCallsPage = `<html><body><div class="entry"><div class="content" date-updated="07.09.2026 08:00">
<h2>Расписание звонков</h2>
<table>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br />10.45 &ndash; 11.30</td></tr>
</table>
</div></div></body></html>`

func parserAlertHarness(t *testing.T, cfg *config.Config) (*parserpkg.Fetcher, *cache.RaspCache, *health.Tracker, *notification.HealthNotifier, *recordingSender, *health.IncidentLog) {
	t.Helper()

	raspper := builderCache(t)
	tracker := health.NewTracker(healthThresholds(cfg))
	fetcher := parserpkg.NewFetcher(logger.New("error", nil), raspper, parserpkg.Options{
		Guard: parserGuard(cfg),
		OnReport: func(report parserpkg.Report) {
			tracker.ParserReport(report.Source, parserLayoutIssues(report), guardIssues(report))
		},
	})

	sender := &recordingSender{}
	incidents := health.NewIncidentLog(nil, 0)
	notifier := notification.NewHealthNotifier(tracker, logger.New("error", nil), sender, adminFinder{}, time.Minute, nil)
	notifier.SetIncidents(incidents)

	return fetcher, raspper, tracker, notifier, sender, incidents
}

func TestUndatedDayColumnReachesTheAdminsEndToEnd(t *testing.T) {
	page := &mutablePage{html: undatedGroupsPage}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "teacher") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(entriesPage("Преподаватель", 1)))
			return
		}
		page.serve(w)
	}))
	defer srv.Close()

	cfg := &config.Config{Health: &config.HealthConfig{ParserLayoutFailures: 1}}
	fetcher, raspper, _, notifier, sender, incidents := parserAlertHarness(t, cfg)

	if err := fetcher.Timetable(t.Context(), srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	entry, ok := raspper.GetGroups()["63ТП"].(map[string]any)
	if !ok {
		t.Fatalf("the cache must keep the group: %+v", raspper.GetGroups())
	}
	if days, _ := entry["days"].([]any); len(days) != 1 {
		t.Fatalf("the undated column must be dropped from the cache: %+v", entry["days"])
	}

	notifier.Check()

	delivered := sender.delivered()
	if len(delivered) != 1 {
		t.Fatalf("expected one admin alert for the skipped column, got %+v", delivered)
	}
	for _, want := range []string{
		notification.HealthAlertTitle(health.AlertParserLayout),
		"th: day header without dd.MM.yyyy",
		"skipped 1",
	} {
		if !strings.Contains(delivered[0].text, want) {
			t.Errorf("alert %q misses %q", delivered[0].text, want)
		}
	}
	if len(delivered[0].buttons) != 1 || delivered[0].buttons[0].Data != notification.ParserReparseCallback {
		t.Fatalf("the alert must offer the reparse button, got %+v", delivered[0].buttons)
	}

	open := incidents.Open()
	if len(open) != 1 || open[0].Key != health.AlertParserLayout {
		t.Fatalf("the skipped column must enter the incident history, got %+v", open)
	}
	if !strings.Contains(open[0].Detail, "skipped 1") {
		t.Errorf("the incident must keep the skipped count: %q", open[0].Detail)
	}

	page.set(datedGroupsPage)
	if err := fetcher.Timetable(t.Context(), srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	notifier.Check()

	messages := sender.messages()
	if len(messages) != 2 || !strings.Contains(messages[1], "восстановлено") {
		t.Fatalf("a dated page must recover the alert, got %+v", messages)
	}
}

func TestUndatedBellScheduleStaysVisibleWithoutAnAlert(t *testing.T) {
	page := &mutablePage{html: undatedCallsPage}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page.serve(w)
	}))
	defer srv.Close()

	cfg := &config.Config{Health: &config.HealthConfig{ParserLayoutFailures: 1}}
	fetcher, _, tracker, notifier, sender, incidents := parserAlertHarness(t, cfg)

	if err := fetcher.Calls(t.Context(), srv.URL); err != nil {
		t.Fatalf("calls: %v", err)
	}

	notifier.Check()

	if delivered := sender.delivered(); len(delivered) != 0 {
		t.Fatalf("the page date is advisory, so no admin alert may fire: %+v", delivered)
	}
	if open := incidents.Open(); len(open) != 0 {
		t.Fatalf("an advisory issue must not enter the incident history: %+v", open)
	}

	snapshot := tracker.Snapshot()
	if len(snapshot.Alerts) != 0 {
		t.Fatalf("a missing page date must keep the bot healthy: %+v", snapshot.Alerts)
	}
	if snapshot.Parser.LayoutFailures != 0 {
		t.Errorf("layout failures = %d, want 0 for an advisory issue", snapshot.Parser.LayoutFailures)
	}
	advisory, ok := layoutIssue(snapshot.Parser.Layout, parserpkg.SourceCalls, "[date-updated] with dd.MM.yyyy")
	if !ok {
		t.Fatalf("the missing date must stay in the health snapshot: %+v", snapshot.Parser.Layout)
	}
	if !advisory.Advisory {
		t.Errorf("the snapshot must mark the issue as advisory: %+v", advisory)
	}

	page.set(datedCallsPage)
	if err := fetcher.Calls(t.Context(), srv.URL); err != nil {
		t.Fatalf("calls: %v", err)
	}
	notifier.Check()

	if layout := tracker.Snapshot().Parser.Layout; len(layout) != 0 {
		t.Fatalf("a dated page must clear the advisory issue: %+v", layout)
	}
}

func layoutIssue(issues []health.LayoutIssue, source, selector string) (health.LayoutIssue, bool) {
	for _, issue := range issues {
		if issue.Source == source && issue.Selector == selector {
			return issue, true
		}
	}
	return health.LayoutIssue{}, false
}
