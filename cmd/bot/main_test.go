package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	notifier := notification.NewHealthNotifier(tracker, logger.New("error", nil), sender, adminFinder{}, time.Minute, nil)

	if err := fetcher.Timetable(srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
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

	if err := fetcher.Timetable(srv.URL+"/groups", srv.URL+"/teachers"); err != nil {
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
}

func builderCache(t *testing.T) *cache.RaspCache {
	t.Helper()

	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return c
}
