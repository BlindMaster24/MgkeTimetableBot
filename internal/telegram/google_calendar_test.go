package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

type recordedCall struct {
	Method string
	Text   string
	Raw    string
}

type recordingCaller struct {
	mu    sync.Mutex
	tests []string
	calls []recordedCall
}

func (c *recordingCaller) Call(_ context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	name := url
	if idx := strings.LastIndexByte(url, '/'); idx >= 0 {
		name = url[idx+1:]
	}
	record := recordedCall{Method: name, Raw: string(data.BodyRaw)}
	var payload map[string]any
	if err := json.Unmarshal(data.BodyRaw, &payload); err == nil {
		if text, ok := payload["text"].(string); ok {
			record.Text = text
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if record.Text != "" {
		c.tests = append(c.tests, record.Text)
	}
	c.calls = append(c.calls, record)
	return &telegoapi.Response{Ok: true, Result: []byte(`{"message_id": 1}`)}, nil
}

func (c *recordingCaller) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tests = nil
	c.calls = nil
}

func (c *recordingCaller) last() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.tests) == 0 {
		return ""
	}
	return c.tests[len(c.tests)-1]
}

func (c *recordingCaller) delivered() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, call := range c.calls {
		switch call.Method {
		case "sendMessage", "sendPhoto", "sendDocument", "editMessageText":
			return true
		}
	}
	return false
}

func (c *recordingCaller) deliveredText() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var text string
	for _, call := range c.calls {
		switch call.Method {
		case "sendMessage", "sendPhoto", "sendDocument", "editMessageText":
			text = call.Text
		}
	}
	return text
}

func (c *recordingCaller) payloads() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]string, 0, len(c.calls))
	for _, call := range c.calls {
		out = append(out, call.Raw)
	}
	return out
}

type fakeGoogleAPI struct {
	calendars   []string
	created     string
	added       []string
	removed     []string
	roles       []string
	listErr     error
	createCalls int
}

func (f *fakeGoogleAPI) ListCalendarIDs(context.Context) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.calendars, nil
}

func (f *fakeGoogleAPI) CreateCalendar(_ context.Context, _ string) (string, error) {
	f.createCalls++
	return f.created, nil
}

func (f *fakeGoogleAPI) AddCalendarToUser(_ context.Context, calendarID string) error {
	f.added = append(f.added, calendarID)
	return nil
}

func (f *fakeGoogleAPI) RemoveCalendarFromUser(_ context.Context, calendarID string) error {
	f.removed = append(f.removed, calendarID)
	return nil
}

func (f *fakeGoogleAPI) SetUserRole(_ context.Context, calendarID, email, role string) error {
	f.roles = append(f.roles, calendarID+"|"+email+"|"+role)
	return nil
}

type fakeGoogleService struct {
	api            *fakeGoogleAPI
	synced         []string
	syncedCalendar []string
	syncedLessons  []google.DayLesson
	userErr        error
	syncDisabled   bool
}

func (f *fakeGoogleService) Configured() bool { return true }

func (f *fakeGoogleService) AuthURL(state string) string {
	return "https://accounts.example/auth?state=" + state
}

func (f *fakeGoogleService) Exchange(context.Context, string) (google.Credentials, string, error) {
	return google.Credentials{RefreshToken: "refresh", AccessToken: "access"}, "user@example.com", nil
}

func (f *fakeGoogleService) UserClient(context.Context, google.Credentials, func(google.Credentials)) (googleUserAPI, error) {
	if f.userErr != nil {
		return nil, f.userErr
	}
	return f.api, nil
}

func (f *fakeGoogleService) SyncDay(_ context.Context, calendarID, date string, lessons []google.DayLesson, _ google.Schedule) error {
	f.synced = append(f.synced, date)
	f.syncedCalendar = append(f.syncedCalendar, calendarID)
	f.syncedLessons = append(f.syncedLessons, lessons...)
	return nil
}

func (f *fakeGoogleService) SyncEnabled() bool { return !f.syncDisabled }

func setupGoogleBot(t *testing.T) (*Bot, *Repository, *recordingCaller, *fakeGoogleService) {
	t.Helper()
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	service := &fakeGoogleService{api: &fakeGoogleAPI{created: "created-calendar@group.calendar.google.com"}}
	b.google = service
	return b, repo, caller, service
}

func linkedChat(t *testing.T, r *Repository, userID int64) *Chat {
	t.Helper()
	chat, err := r.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatalf("find chat: %v", err)
	}
	chat.GoogleEmail = "user@example.com"
	chat.Mode = ModeStudent
	chat.Group = "100"
	if err := r.Save(chat); err != nil {
		t.Fatalf("save chat: %v", err)
	}
	if err := r.SaveGoogleAccount(&GoogleAccount{Email: "user@example.com", RefreshToken: "refresh", AccessToken: "access"}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	return chat
}

func TestGoogleCalendarStoreRoundTrip(t *testing.T) {
	_, repo, _, _ := setupGoogleBot(t)

	account := &GoogleAccount{Email: "a@b.c", RefreshToken: "r1", AccessToken: "a1", AccessTokenExpires: 42}
	if err := repo.SaveGoogleAccount(account); err != nil {
		t.Fatalf("save account: %v", err)
	}
	loaded, err := repo.GoogleAccountByEmail("a@b.c")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if loaded.RefreshToken != "r1" || loaded.AccessToken != "a1" || loaded.AccessTokenExpires != 42 {
		t.Errorf("account mismatch: %+v", loaded)
	}

	account.AccessToken = "a2"
	if err := repo.SaveGoogleAccount(account); err != nil {
		t.Fatalf("update account: %v", err)
	}
	loaded, _ = repo.GoogleAccountByEmail("a@b.c")
	if loaded.AccessToken != "a2" {
		t.Errorf("account update lost: %+v", loaded)
	}

	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-1"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}
	found, err := repo.GoogleCalendarByTypeValue("group", "100")
	if err != nil || found == nil {
		t.Fatalf("calendar by type/value: %v", err)
	}
	if found.CalendarID != "cal-1" {
		t.Errorf("calendar id mismatch: %q", found.CalendarID)
	}

	byLocal, err := repo.GoogleCalendarByLocalID(found.ID)
	if err != nil || byLocal == nil || byLocal.Value != "100" {
		t.Fatalf("calendar by local id: %v %+v", err, byLocal)
	}

	byIDs, err := repo.GoogleCalendarsByIDs([]string{"cal-1", "missing"})
	if err != nil {
		t.Fatalf("calendars by ids: %v", err)
	}
	if len(byIDs) != 1 {
		t.Errorf("expected one calendar, got %d", len(byIDs))
	}

	if err := repo.DeleteGoogleCalendar("cal-1"); err != nil {
		t.Fatalf("delete calendar: %v", err)
	}
	missing, err := repo.GoogleCalendarByTypeValue("group", "100")
	if err != nil || missing != nil {
		t.Errorf("calendar should be gone: %v %+v", err, missing)
	}
}

func TestGoogleAuthStateRoundTrip(t *testing.T) {
	state := EncodeGoogleState("telegram", 12345)
	service, peerID, err := DecodeGoogleState(state)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if service != "telegram" || peerID != 12345 {
		t.Errorf("state mismatch: %q %d", service, peerID)
	}
	if _, _, err := DecodeGoogleState("not-base64"); err == nil {
		t.Error("expected decode error")
	}
}

func TestGoogleCalendarMenuRequiresLinkedAccount(t *testing.T) {
	b, repo, caller, _ := setupGoogleBot(t)
	chat, _ := repo.FindOrCreate("telegram", 5001)
	chat.Mode = ModeStudent
	repo.Save(chat)

	u := makeUpdate(5001, "")
	u.Bot = b
	if err := b.showGoogleCalendarMenu(u, chat); err != nil {
		t.Fatalf("menu: %v", err)
	}
	if caller.last() != "Гугл аккаунт не привязан, чтобы привязать, нажмите на кнопку ниже" {
		t.Errorf("auth text mismatch: %q", caller.last())
	}
}

func TestGoogleCalendarMenuText(t *testing.T) {
	b, repo, caller, _ := setupGoogleBot(t)
	chat := linkedChat(t, repo, 5002)

	u := makeUpdate(5002, "")
	u.Bot = b
	if err := b.showGoogleCalendarMenu(u, chat); err != nil {
		t.Fatalf("menu: %v", err)
	}
	want := "Привязанный гугл аккаунт: user@example.com.\nДействие с календарями:"
	if caller.last() != want {
		t.Errorf("menu text = %q, want %q", caller.last(), want)
	}

	rows := flattenInlineKeyboard(googleMenuKeyboard())
	if rows != "Список Добавить Права " {
		t.Errorf("menu keyboard = %q", rows)
	}
}

func TestGoogleCalendarListAndProgress(t *testing.T) {
	b, repo, caller, service := setupGoogleBot(t)
	chat := linkedChat(t, repo, 5003)
	service.api.calendars = []string{"cal-1"}
	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-1"}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	u := makeUpdate(5003, "")
	u.Bot = b
	if err := b.showGoogleCalendarList(u, chat); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(caller.last(), "Список календарей с расписанием:") {
		t.Errorf("list header missing: %q", caller.last())
	}
	if !strings.Contains(caller.last(), "1. group, 100 (Синхронизация: 100.00%)") {
		t.Errorf("list line mismatch: %q", caller.last())
	}

	service.api.calendars = nil
	if err := b.showGoogleCalendarList(u, chat); err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if caller.last() != "Нет добавленных календарей" {
		t.Errorf("empty list text = %q", caller.last())
	}
}

func TestGoogleCalendarAddCreatesAndLinks(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	chat := linkedChat(t, repo, 5004)

	u := makeUpdate(5004, "")
	u.Bot = b
	u.Callback = &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 5004}}
	if err := b.showGoogleCalendarAdd(u, chat); err != nil {
		t.Fatalf("add: %v", err)
	}

	saved, err := repo.GoogleCalendarByTypeValue("group", "100")
	if err != nil || saved == nil {
		t.Fatalf("calendar not stored: %v", err)
	}
	if saved.CalendarID != "created-calendar@group.calendar.google.com" {
		t.Errorf("calendar id = %q", saved.CalendarID)
	}
	if len(service.api.added) != 1 {
		t.Errorf("calendar not added to user account: %v", service.api.added)
	}
}

func TestGoogleCalendarAddRequiresGroupSelection(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	chat, _ := repo.FindOrCreate("telegram", 5005)
	chat.GoogleEmail = "user@example.com"
	chat.Mode = ModeStudent
	repo.Save(chat)

	u := makeUpdate(5005, "")
	u.Bot = b
	u.Callback = &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 5005}}
	if err := b.showGoogleCalendarAdd(u, chat); err != nil {
		t.Fatalf("add: %v", err)
	}
	if service.api.createCalls != 0 {
		t.Error("calendar must not be created without a group")
	}
}

func TestGoogleCalendarPermissionsFlow(t *testing.T) {
	b, repo, caller, service := setupGoogleBot(t)
	chat := linkedChat(t, repo, 5006)
	calendar := &GoogleCalendar{Type: "teacher", Value: "Иванов И.И.", CalendarID: "cal-perm"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}
	service.api.calendars = []string{"cal-perm"}

	u := makeUpdate(5006, "")
	u.Bot = b
	u.Callback = &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 5006}}

	if err := b.showGooglePermissionsMenu(u, chat); err != nil {
		t.Fatalf("permissions menu: %v", err)
	}
	if caller.last() != "Выберите календарь для управления правами:" {
		t.Errorf("permissions menu text = %q", caller.last())
	}

	if err := b.showGooglePermissionsCalendar(u, chat, calendar.ID); err != nil {
		t.Fatalf("permissions calendar: %v", err)
	}
	if caller.last() != "Календарь: teacher, Иванов И.И.\nВыберите действие:" {
		t.Errorf("permissions calendar text = %q", caller.last())
	}

	if err := b.applyGooglePermissions(u, chat, "writer", calendar.ID); err != nil {
		t.Fatalf("apply permissions: %v", err)
	}
	if len(service.api.roles) != 1 || service.api.roles[0] != "cal-perm|user@example.com|writer" {
		t.Errorf("role not applied: %v", service.api.roles)
	}
	if caller.last() != "Права редактирования успешно обновлены для user@example.com." {
		t.Errorf("apply text = %q", caller.last())
	}
}

func TestGoogleCalendarDropRemovesCalendar(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	chat := linkedChat(t, repo, 5007)
	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-drop"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	u := makeUpdate(5007, "")
	u.Bot = b
	u.Callback = &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 5007}}
	if err := b.dropGoogleCalendar(u, chat, calendar.ID); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if len(service.api.removed) != 1 || service.api.removed[0] != "cal-drop" {
		t.Errorf("calendar not removed from account: %v", service.api.removed)
	}
	gone, err := repo.GoogleCalendarByLocalID(calendar.ID)
	if err != nil || gone != nil {
		t.Errorf("calendar still stored: %v %+v", err, gone)
	}
}

func TestGoogleCallbackRouting(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	linkedChat(t, repo, 5008)
	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-route"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}
	service.api.calendars = []string{"cal-route"}

	cb := &googleCalCb{bot: b}
	cases := []struct {
		data   string
		assert func(t *testing.T)
	}{
		{"gcal:list", func(t *testing.T) {
			if len(service.api.calendars) == 0 {
				t.Error("list not requested")
			}
		}},
		{"gcal:perm_cal:" + googleLocalID(calendar.ID), func(t *testing.T) {
			if len(service.api.roles) != 0 {
				t.Error("permissions applied too early")
			}
		}},
		{"gcal:perm_apply:writer:" + googleLocalID(calendar.ID), func(t *testing.T) {
			if len(service.api.roles) != 1 {
				t.Errorf("permissions not applied: %v", service.api.roles)
			}
		}},
		{"gcal:drop:" + googleLocalID(calendar.ID), func(t *testing.T) {
			if len(service.api.removed) != 1 {
				t.Errorf("calendar not removed: %v", service.api.removed)
			}
		}},
		{"gcal:unknown", func(t *testing.T) {}},
	}

	for _, tc := range cases {
		u := &Update{
			Bot:      b,
			ChatID:   5008,
			UserID:   5008,
			Data:     tc.data,
			Callback: &telego.CallbackQuery{ID: "cb", From: telego.User{ID: 5008}},
		}
		if err := cb.Handler(context.Background(), u); err != nil {
			t.Errorf("%s: %v", tc.data, err)
			continue
		}
		tc.assert(t)
	}
}

func flattenInlineKeyboard(kb *telego.InlineKeyboardMarkup) string {
	if kb == nil {
		return ""
	}
	var out string
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			out += button.Text + " "
		}
	}
	return out
}
