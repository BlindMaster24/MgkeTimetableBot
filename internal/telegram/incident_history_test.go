package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/mymmrac/telego"
)

func incidentsSnapshot() *health.IncidentLog {
	log := health.NewIncidentLog(nil, 0)
	log.Record(health.AlertParserLayout, "runs=2 threshold=2 missing groups: .table")
	log.Resolve(health.AlertParserLayout)
	log.Record(health.AlertCalendarFailures, "consecutiveFailures=3 threshold=3")
	log.MarkManual(health.ScopeCalendar, "помогла ручная синхронизация")
	log.Resolve(health.AlertCalendarFailures)
	log.Record(health.AlertParserGuard, "groups: shrink: 35 -> 3 (dropped 91%, limit 80%)")
	return log
}

func TestIncidentsTextRendersTheHistory(t *testing.T) {
	b := setupTestBot(t)
	b.SetIncidentLog(incidentsSnapshot())

	text := b.incidentsText()
	for _, want := range []string{
		"-- История срабатываний (последние 20) --",
		"Парсер резко потерял данные",
		"parser_guard",
		"groups: shrink: 35 -> 3",
		"Итог: ещё не устранено",
		"Ошибки синхронизации Google Calendar",
		"Итог: помогла ручная синхронизация",
		"Парсер перестал находить данные на сайте",
		"Итог: восстановилось само",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("incidents text omits %q:\n%s", want, text)
		}
	}
}

func TestIncidentsTextReportsAnEmptyHistory(t *testing.T) {
	b := setupTestBot(t)
	b.SetIncidentLog(health.NewIncidentLog(nil, 0))

	if text := b.incidentsText(); !strings.Contains(text, "Срабатываний не было") {
		t.Errorf("empty history text = %q", text)
	}
}

func TestIncidentsTextWithoutALog(t *testing.T) {
	b := setupTestBot(t)

	if text := b.incidentsText(); text != b.loc("incidents_unavailable") {
		t.Errorf("text = %q", text)
	}
}

func TestIncidentsTextKeepsTheNewestFirst(t *testing.T) {
	b := setupTestBot(t)
	b.SetIncidentLog(incidentsSnapshot())

	text := b.incidentsText()
	if strings.Index(text, "parser_guard") > strings.Index(text, "calendar_failures") {
		t.Errorf("the newest incident must come first:\n%s", text)
	}
}

func TestIncidentsCommandOffersButtonsForOpenIncidents(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetIncidentLog(incidentsSnapshot())

	handler := &incidentsCmd{bot: b}
	u := &Update{Bot: b, ChatID: 4242, UserID: 4242}
	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if !strings.Contains(caller.last(), "-- История срабатываний") {
		t.Fatalf("last message = %q", caller.last())
	}

	payloads := strings.Join(caller.payloads(), " ")
	if !strings.Contains(payloads, notification.ParserReparseCallback) {
		t.Errorf("an open parser incident must offer the reparse button, payloads: %v", caller.payloads())
	}
	if strings.Contains(payloads, notification.CalendarSyncCallback) {
		t.Errorf("a resolved calendar incident must not offer the sync button, payloads: %v", caller.payloads())
	}
}

func TestIncidentsCommandIsAdminOnly(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetIncidentLog(incidentsSnapshot())

	handler := &incidentsCmd{bot: b}
	u := &Update{Bot: b, ChatID: 999, UserID: 999}
	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Errorf("non-admin got %q", caller.last())
	}
}

func TestCalendarSyncCallbackRunsTheSync(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	incidents := health.NewIncidentLog(nil, 0)
	incidents.Record(health.AlertCalendarFailures, "consecutiveFailures=3 threshold=3")
	b.SetIncidentLog(incidents)

	done := make(chan struct{})
	b.SetCalendarSyncFunc(func(context.Context) (int, error) {
		close(done)
		return 4, nil
	})

	cb := &calendarSyncCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.CalendarSyncCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the sync button did not trigger the calendar sync")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(caller.deliveredText(), "Синхронизация завершена: 4 дн.") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(caller.deliveredText(), "Синхронизация завершена: 4 дн.") {
		t.Fatalf("the sync result never reached the chat, messages: %v", caller.payloads())
	}

	recent := incidents.Recent(1)
	if recent[0].Resolution != health.ResolutionManual {
		t.Errorf("a successful manual sync must be recorded as the fix: %+v", recent[0])
	}
	if recent[0].Note != b.loc("incident_fix_calendar") {
		t.Errorf("note = %q", recent[0].Note)
	}
}

func TestCalendarSyncCallbackReportsFailures(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	b.SetCalendarSyncFunc(func(context.Context) (int, error) {
		return 2, errors.New("quota exceeded")
	})

	cb := &calendarSyncCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.CalendarSyncCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(caller.deliveredText(), "quota exceeded") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the sync error never reached the chat, messages: %v", caller.payloads())
}

func TestCalendarSyncCallbackRejectsNonAdmins(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	var calls int32
	b.SetCalendarSyncFunc(func(context.Context) (int, error) {
		calls++
		return 1, nil
	})

	cb := &calendarSyncCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   999,
		UserID:   999,
		Data:     notification.CalendarSyncCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 999}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if calls != 0 {
		t.Errorf("a non-admin triggered %d syncs", calls)
	}
	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Errorf("non-admin got %q", caller.last())
	}
}

func TestCalendarSyncCallbackWithoutASyncFunc(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	cb := &calendarSyncCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.CalendarSyncCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !strings.Contains(caller.last(), b.loc("calendar_sync_unavailable")) {
		t.Errorf("expected the unavailable notice, got %q", caller.last())
	}
}

func TestReparseCallbackRecordsTheManualFix(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	incidents := health.NewIncidentLog(nil, 0)
	incidents.Record(health.AlertParserGuard, "groups: shrink: 35 -> 3")
	b.SetIncidentLog(incidents)

	done := make(chan struct{})
	b.SetParseFunc(func() error {
		close(done)
		return nil
	})

	cb := &reparseCb{bot: b}
	u := &Update{
		Bot:      b,
		ChatID:   4242,
		UserID:   4242,
		Data:     notification.ParserReparseCallback,
		Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 4242}},
	}
	if err := cb.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the reparse button did not trigger the parser")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if incidents.Recent(1)[0].Resolution == health.ResolutionManual {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the manual reparse was not recorded: %+v", incidents.Recent(1))
}
