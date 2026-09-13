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

func apiFailureSnapshot() health.Snapshot {
	return health.Snapshot{
		API: health.APIStats{
			Errors:      16,
			LastStatus:  500,
			LastErrorAt: "2026-09-13T12:04:33Z",
			Endpoints: []health.APIEndpointStat{
				{Method: "GET", Path: "/api/health", Status: 500, Requests: 120, Errors: 12, AvgMillis: 8, MaxMillis: 40, Message: "database is locked"},
				{Method: "GET", Path: "/api/groups", Status: 503, Requests: 12, Errors: 4, AvgMillis: 640, MaxMillis: 1200, Slow: true},
			},
			LastErrors: []health.APIErrorSample{
				{Method: "GET", Path: "/api/health", Status: 500, At: "2026-09-13T12:04:33Z", Message: "database is locked"},
				{Method: "GET", Path: "/api/groups", Status: 503, At: "2026-09-13T12:04:12Z"},
			},
		},
		Alerts: []health.Alert{{
			Key:    health.AlertAPIErrors,
			Level:  health.LevelCritical,
			Detail: "errors=16 window=5m0s\npaths: GET /api/health x12 (500)",
		}},
	}
}

func TestIncidentsTextShowsAPIDiagnostics(t *testing.T) {
	b := setupTestBot(t)
	b.SetIncidentLog(incidentsSnapshot())
	b.SetHealthSource(&fakeHealthSource{snapshot: apiFailureSnapshot()})

	text := b.incidentsText()
	for _, want := range []string{
		"-- Ошибки HTTP API (свежие) --",
		"GET /api/health — запросов 120, ошибок 12, в среднем 8 мс (макс 40 мс) — database is locked",
		"🐌 GET /api/groups — запросов 12, ошибок 4, в среднем 640 мс (макс 1.20 с)",
		"Последние ошибки:",
		"GET /api/health 500 — database is locked",
		"GET /api/groups 503",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("incidents text omits %q:\n%s", want, text)
		}
	}
}

func TestIncidentsTextHidesAPIDiagnosticsWhenClean(t *testing.T) {
	b := setupTestBot(t)
	b.SetIncidentLog(incidentsSnapshot())
	b.SetHealthSource(&fakeHealthSource{snapshot: parserSnapshot()})

	if text := b.incidentsText(); strings.Contains(text, b.loc("api_diag_header")) {
		t.Errorf("a clean API must not get a diagnostics block:\n%s", text)
	}
}

func TestIncidentsTextIndentsMultilineDetails(t *testing.T) {
	b := setupTestBot(t)
	log := health.NewIncidentLog(nil, 0)
	log.Record(health.AlertAPIErrors, "errors=20 window=5m0s\npaths: GET /api/health x20 (500)")
	b.SetIncidentLog(log)

	text := b.incidentsText()
	if !strings.Contains(text, "   errors=20 window=5m0s\n   paths: GET /api/health x20 (500)") {
		t.Errorf("multiline details must keep the block indent:\n%s", text)
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

func TestIncidentsTextShowsTheFixWhileTheIncidentIsOpen(t *testing.T) {
	b := setupTestBot(t)
	log := health.NewIncidentLog(nil, 0)
	log.Record(health.AlertAPIErrors, "errors=20 window=5m0s\npaths: GET /api/groups x20 (500)")
	log.MarkManual(health.ScopeAPI, b.locData("incident_fix_api", map[string]interface{}{"Healthy": 6, "Total": 6}))
	b.SetIncidentLog(log)

	text := b.incidentsText()
	if !strings.Contains(text, "🔧") {
		t.Errorf("a recorded fix must be visible while the incident is open:\n%s", text)
	}
	if !strings.Contains(text, "Итог: помогла ручная проверка API (6 из 6 эндпоинтов отвечают)") {
		t.Errorf("the note must be the outcome:\n%s", text)
	}
	if strings.Contains(text, "ещё не устранено") {
		t.Errorf("a recorded fix must replace the open outcome:\n%s", text)
	}
}

func TestIncidentsTextReportsWhatChangedAtStartup(t *testing.T) {
	b := setupTestBot(t)
	log := health.NewIncidentLog(nil, 0)
	log.SetStartupStamp(health.StartupStamp{Build: "1.0.0 (aaaaaaa, 2026-09-01)", Config: "11111111"})
	log.Record(health.AlertAPIErrors, "errors=20 window=5m0s")
	log.NoteStartup(health.ScopeAPI, health.StartupStamp{Build: "1.0.0 (aaaaaaa, 2026-09-01)", Config: "22222222"}, func(change health.StartupChange, previous, current health.StartupStamp) string {
		if change == health.ChangeConfig {
			return b.locData("incident_note_config", map[string]interface{}{"Current": current.Config, "Previous": previous.Config})
		}
		return b.locData("incident_note_restart", map[string]interface{}{"Current": current.Build})
	})
	b.SetIncidentLog(log)

	text := b.incidentsText()
	if !strings.Contains(text, "Итог: после правки конфига: 22222222 (было 11111111)") {
		t.Errorf("the config note must be rendered:\n%s", text)
	}
}

func TestIncidentsTextReportsARestartNote(t *testing.T) {
	b := setupTestBot(t)
	log := health.NewIncidentLog(nil, 0)
	log.SetStartupStamp(health.StartupStamp{Build: "1.0.0 (aaaaaaa, 2026-09-01)", Config: "11111111"})
	log.Record(health.AlertAPIErrors, "errors=20 window=5m0s")
	log.NoteStartup(health.ScopeAPI, health.StartupStamp{Build: "1.0.0 (aaaaaaa, 2026-09-01)", Config: "11111111"}, func(change health.StartupChange, previous, current health.StartupStamp) string {
		return b.locData("incident_note_restart", map[string]interface{}{"Current": current.Build})
	})
	b.SetIncidentLog(log)

	text := b.incidentsText()
	if !strings.Contains(text, "Итог: после перезапуска бота: 1.0.0 (aaaaaaa, 2026-09-01)") {
		t.Errorf("the restart note must be rendered:\n%s", text)
	}
}

func TestIncidentsCommandOffersTheProbeForAnOpenAPIFailure(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	log := health.NewIncidentLog(nil, 0)
	log.Record(health.AlertAPIErrors, "errors=20 window=5m0s")
	b.SetIncidentLog(log)

	handler := &incidentsCmd{bot: b}
	u := &Update{Bot: b, ChatID: 4242, UserID: 4242}
	if err := handler.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	payloads := strings.Join(caller.payloads(), " ")
	if !strings.Contains(payloads, notification.APIProbeCallback) {
		t.Errorf("an open API incident must offer the live probe, payloads: %v", caller.payloads())
	}
	if strings.Contains(payloads, notification.ParserReparseCallback) {
		t.Errorf("an API incident must not offer the reparse button, payloads: %v", caller.payloads())
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
