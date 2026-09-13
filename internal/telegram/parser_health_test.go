package telegram

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/mymmrac/telego"
)

type fakeHealthSource struct {
	snapshot health.Snapshot
	slow     time.Duration
}

func (f *fakeHealthSource) Snapshot() health.Snapshot { return f.snapshot }

func (f *fakeHealthSource) SlowThreshold() time.Duration {
	if f.slow <= 0 {
		return health.DefaultThresholds().APISlow
	}
	return f.slow
}

func parserSnapshot() health.Snapshot {
	return health.Snapshot{
		Parser: health.ParserStats{
			Runs:                12,
			Errors:              2,
			ConsecutiveFailures: 1,
			LastSuccessAt:       "2026-09-13T12:00:00Z",
			LastErrorAt:         "2026-09-13T12:05:00Z",
			LastError:           "site down",
			LagSeconds:          180,
			LastDurationMS:      840,
			Layout: []health.LayoutIssue{{
				Source:   "groups",
				Selector: ".table",
				Found:    0,
			}},
			LayoutFailures: 1,
			Guard: []health.GuardIssue{{
				Source: "groups",
				Reason: "shrink",
				Detail: "35 -> 3 (dropped 91%, limit 80%)",
			}},
			GuardFailures: 1,
		},
		Alerts: []health.Alert{{
			Key:    health.AlertParserGuard,
			Level:  health.LevelCritical,
			Detail: "runs=1 threshold=1",
		}},
	}
}

func TestParserHealthTextRendersTheSnapshot(t *testing.T) {
	b := setupTestBot(t)
	b.SetHealthSource(&fakeHealthSource{snapshot: parserSnapshot()})

	text := b.parserHealthText()
	for _, want := range []string{
		"-- Парсер расписания --",
		"сбоев подряд: 1",
		"Разборов: 12, ошибок: 2",
		"(3m0s назад)",
		"site down",
		"Длительность последнего разбора: 840 мс",
		"groups: .table (найдено 0)",
		"Вёрстка сайта (подряд 1)",
		"groups: shrink: 35 -> 3 (dropped 91%, limit 80%)",
		"Защита данных (подряд 1)",
		"parser_guard",
		"Групп:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("parser health text omits %q:\n%s", want, text)
		}
	}
}

func TestParserHealthTextReportsAHealthyParser(t *testing.T) {
	b := setupTestBot(t)
	b.SetHealthSource(&fakeHealthSource{snapshot: health.Snapshot{
		Parser: health.ParserStats{Runs: 4, LastSuccessAt: "2026-09-13T12:00:00Z"},
	}})

	text := b.parserHealthText()
	if !strings.Contains(text, "Состояние: ✅ без сбоев") {
		t.Errorf("expected the healthy marker, got:\n%s", text)
	}
	if !strings.Contains(text, "Активных алертов нет") {
		t.Errorf("expected the no-alert marker, got:\n%s", text)
	}
	if strings.Contains(text, "Вёрстка сайта") || strings.Contains(text, "Защита данных") {
		t.Errorf("healthy parser must not render the failure sections:\n%s", text)
	}
}

func TestParserHealthTextWithoutAMetricsSource(t *testing.T) {
	b := setupTestBot(t)

	if text := b.parserHealthText(); text != "Метрики здоровья недоступны" {
		t.Errorf("text = %q", text)
	}
}

func TestParserHealthCommandSendsTheReparseButton(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetHealthSource(&fakeHealthSource{snapshot: parserSnapshot()})

	handler := &parserHealthCmd{bot: b}
	u := &Update{Bot: b, ChatID: 4242, UserID: 4242}
	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if !strings.Contains(caller.last(), "-- Парсер расписания --") {
		t.Fatalf("last message = %q", caller.last())
	}

	button := false
	for _, payload := range caller.payloads() {
		if strings.Contains(payload, notification.ParserReparseCallback) {
			button = true
		}
	}
	if !button {
		t.Errorf("the health command must offer the reparse button, payloads: %v", caller.payloads())
	}
}

func TestParserHealthCommandIsAdminOnly(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetHealthSource(&fakeHealthSource{snapshot: parserSnapshot()})

	handler := &parserHealthCmd{bot: b}
	u := &Update{Bot: b, ChatID: 999, UserID: 999}
	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Errorf("non-admin got %q", caller.last())
	}
}

func TestReparseCallbackTriggersTheParser(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	done := make(chan struct{})
	var calls int32
	b.SetParseFunc(func() error {
		atomic.AddInt32(&calls, 1)
		close(done)
		return nil
	})

	cb := &reparseCb{bot: b}
	u := &Update{
		Bot:    b,
		ChatID: 4242,
		UserID: 4242,
		Data:   notification.ParserReparseCallback,
		Callback: &telego.CallbackQuery{
			ID:   "cb",
			From: telego.User{ID: 4242},
		},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the reparse button did not trigger the parser")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("parse called %d times", got)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(caller.deliveredText(), b.loc("force_parse_done")) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the reparse result never reached the chat, messages: %v", caller.payloads())
}

func TestReparseCallbackRejectsNonAdmins(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	var calls int32
	b.SetParseFunc(func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	cb := &reparseCb{bot: b}
	u := &Update{
		Bot:    b,
		ChatID: 999,
		UserID: 999,
		Data:   notification.ParserReparseCallback,
		Callback: &telego.CallbackQuery{
			ID:   "cb",
			From: telego.User{ID: 999},
		},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("a non-admin triggered %d parses", got)
	}
	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Errorf("non-admin got %q", caller.last())
	}
}

func TestReparseCallbackWithoutAParser(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	cb := &reparseCb{bot: b}
	u := &Update{
		Bot:    b,
		ChatID: 4242,
		UserID: 4242,
		Data:   notification.ParserReparseCallback,
		Callback: &telego.CallbackQuery{
			ID:   "cb",
			From: telego.User{ID: 4242},
		},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !strings.Contains(caller.last(), b.loc("parse_not_available")) {
		t.Errorf("expected the unavailable notice, got %q", caller.last())
	}
}
