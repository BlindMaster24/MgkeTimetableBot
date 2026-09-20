package telegram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/mymmrac/telego"
)

const (
	googleActionMenu      = "menu"
	googleActionList      = "list"
	googleActionAdd       = "add"
	googleActionDelete    = "delete"
	googleActionDrop      = "drop"
	googleActionPermMenu  = "perm_menu"
	googleActionPermCal   = "perm_cal"
	googleActionPermApply = "perm_apply"
)

var googleCalendarPattern = regexp.MustCompile(`(?i)^((!|/)?(g(oogle)?)?calendar|(📅\s*)?google calendar)$`)

type googleUserAPI interface {
	ListCalendarIDs(ctx context.Context) ([]string, error)
	CreateCalendar(ctx context.Context, summary string) (string, error)
	AddCalendarToUser(ctx context.Context, calendarID string) error
	RemoveCalendarFromUser(ctx context.Context, calendarID string) error
	SetUserRole(ctx context.Context, calendarID, email, role string) error
}

type googleService interface {
	Configured() bool
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (google.Credentials, string, error)
	UserClient(ctx context.Context, creds google.Credentials, save func(google.Credentials)) (googleUserAPI, error)
	SyncDay(ctx context.Context, calendarID string, date string, lessons []google.DayLesson, calls google.Schedule) error
	SyncEnabled() bool
}

type calendarServiceAdapter struct {
	service *google.CalendarService
}

func (a calendarServiceAdapter) Configured() bool { return a.service.Configured() }

func (a calendarServiceAdapter) AuthURL(state string) string { return a.service.AuthURL(state) }

func (a calendarServiceAdapter) Exchange(ctx context.Context, code string) (google.Credentials, string, error) {
	return a.service.Exchange(ctx, code)
}

func (a calendarServiceAdapter) UserClient(ctx context.Context, creds google.Credentials, save func(google.Credentials)) (googleUserAPI, error) {
	api, err := a.service.UserClient(ctx, creds, save)
	if err != nil {
		return nil, err
	}
	return api, nil
}

func (a calendarServiceAdapter) SyncDay(ctx context.Context, calendarID string, date string, lessons []google.DayLesson, calls google.Schedule) error {
	return a.service.SyncDay(ctx, calendarID, date, lessons, calls)
}

func (a calendarServiceAdapter) SyncEnabled() bool { return a.service.SyncEnabled() }

type googleAuthState struct {
	Service string `json:"service"`
	PeerID  int64  `json:"peerId"`
}

func EncodeGoogleState(service string, peerID int64) string {
	raw, err := json.Marshal(googleAuthState{Service: service, PeerID: peerID})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeGoogleState(state string) (string, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", 0, err
	}
	parsed := googleAuthState{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, err
	}
	if parsed.Service == "" || parsed.PeerID == 0 {
		return "", 0, errors.New("invalid google auth state")
	}
	return parsed.Service, parsed.PeerID, nil
}

type googleCalendarCmd struct{ bot *Bot }

func (c *googleCalendarCmd) Hidden() bool { return true }

func (c *googleCalendarCmd) Name() string        { return "/google_calendar" }
func (c *googleCalendarCmd) Description() string { return "Настройка Google Calendar" }

func (c *googleCalendarCmd) MatchText(text string) bool {
	trimmed := strings.TrimSpace(text)
	return googleCalendarPattern.MatchString(trimmed) || trimmed == c.bot.loc("button_google_calendar")
}

func (c *googleCalendarCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if !c.bot.googleReady() {
		return u.Bot.SendText(u.ChatID, "Google Calendar не настроен на сервере.")
	}
	return c.bot.showGoogleCalendarMenu(u, chat)
}

func (b *Bot) googleReady() bool {
	return b.google != nil && b.google.Configured()
}

func (b *Bot) SetGoogleService(svc *google.CalendarService) {
	if svc == nil {
		return
	}
	b.google = calendarServiceAdapter{service: svc}
}

func (b *Bot) googleUserClient(ctx context.Context, chat *Chat) (googleUserAPI, error) {
	if b.google == nil || chat.GoogleEmail == "" {
		return nil, errors.New("google account not linked")
	}
	account, err := b.chatRepo.GoogleAccountByEmail(chat.GoogleEmail)
	if err != nil {
		return nil, err
	}
	creds := google.Credentials{
		RefreshToken: account.RefreshToken,
		AccessToken:  account.AccessToken,
		Expiry:       account.AccessTokenExpires,
	}
	return b.google.UserClient(ctx, creds, func(refreshed google.Credentials) {
		account.RefreshToken = firstNonEmpty(refreshed.RefreshToken, account.RefreshToken)
		account.AccessToken = refreshed.AccessToken
		account.AccessTokenExpires = refreshed.Expiry
		b.chatRepo.SaveGoogleAccount(account)
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (b *Bot) showGoogleCalendarMenu(u *Update, chat *Chat) error {
	if chat.GoogleEmail == "" {
		return b.showGoogleAuth(u, chat)
	}
	text := fmt.Sprintf("Привязанный гугл аккаунт: %s.\nДействие с календарями:", chat.GoogleEmail)
	return b.sendOrEdit(u, text, googleMenuKeyboard())
}

func (b *Bot) showGoogleAuth(u *Update, chat *Chat) error {
	if !b.googleReady() {
		return u.Bot.SendText(u.ChatID, "Google Calendar не настроен на сервере.")
	}
	state := EncodeGoogleState("telegram", chat.PeerID)
	return b.sendOrEdit(u, "Гугл аккаунт не привязан, чтобы привязать, нажмите на кнопку ниже", googleAuthKeyboard(b.google.AuthURL(state)))
}

func googlePayload(action string, args ...string) string {
	parts := append([]string{"gcal:" + action}, args...)
	return strings.Join(parts, ":")
}

func googleProgress(calendar *GoogleCalendar, bounds archive.Bounds, hasBounds bool) float64 {
	if calendar.LastManualSyncedDay == 0 || !hasBounds {
		return 100
	}
	current := calendar.LastManualSyncedDay - bounds.Min
	if current < 0 {
		current = 0
	}
	span := bounds.Max - bounds.Min
	if span <= 0 {
		return 100
	}
	return float64(current) * 100 / float64(span)
}

func (b *Bot) googleDayBounds() (archive.Bounds, bool) {
	if b.archive == nil {
		return archive.Bounds{}, false
	}
	bounds, err := b.archive.DayIndexBounds()
	if err != nil {
		return archive.Bounds{}, false
	}
	return bounds, true
}

func (b *Bot) showGoogleCalendarList(u *Update, chat *Chat) error {
	api, err := b.googleUserClient(context.Background(), chat)
	if err != nil {
		return b.showGoogleAuth(u, chat)
	}

	current := b.googleCalendars(api)
	bounds, hasBounds := b.googleDayBounds()

	var lines []string
	for i, calendar := range current {
		lines = append(lines, fmt.Sprintf("%d. %s, %s (Синхронизация: %.2f%%)",
			i+1, calendar.Type, calendar.Value, googleProgress(calendar, bounds, hasBounds)))
	}

	text := "Нет добавленных календарей"
	if len(lines) > 0 {
		text = "Список календарей с расписанием:\n" + strings.Join(lines, "\n")
	}

	return b.sendOrEdit(u, text, googleListKeyboard(len(lines) > 0))
}

func (b *Bot) showGoogleCalendarDelete(u *Update, chat *Chat) error {
	api, err := b.googleUserClient(context.Background(), chat)
	if err != nil {
		return b.showGoogleAuth(u, chat)
	}

	current := b.googleCalendars(api)
	if len(current) == 0 {
		return b.sendOrEdit(u, "Нет добавленных календарей", googleBackKeyboard())
	}

	rows := make([][]telego.InlineKeyboardButton, 0, len(current)+1)
	for _, calendar := range current {
		rows = append(rows, []telego.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s, %s", calendar.Type, calendar.Value),
			CallbackData: googlePayload(googleActionDrop, googleLocalID(calendar.ID)),
		}})
	}
	rows = append(rows, []telego.InlineKeyboardButton{{Text: "Назад", CallbackData: googlePayload(googleActionList)}})

	return b.sendOrEdit(u, "Выберите календарь для удаления:", &telego.InlineKeyboardMarkup{InlineKeyboard: rows})
}

func (b *Bot) dropGoogleCalendar(u *Update, chat *Chat, localID int64) error {
	calendar, err := b.chatRepo.GoogleCalendarByLocalID(localID)
	if err != nil || calendar == nil {
		return b.sendOrEdit(u, "Календарь не найден", googleBackKeyboard())
	}

	if api, err := b.googleUserClient(context.Background(), chat); err == nil {
		api.RemoveCalendarFromUser(context.Background(), calendar.CalendarID)
	}
	if err := b.chatRepo.DeleteGoogleCalendar(calendar.CalendarID); err != nil {
		return u.Bot.SendText(u.ChatID, "Не удалось удалить календарь")
	}

	return b.showGoogleCalendarDelete(u, chat)
}

func (b *Bot) googleCalendars(api googleUserAPI) []*GoogleCalendar {
	ids, err := api.ListCalendarIDs(context.Background())
	if err != nil {
		return nil
	}
	calendars, err := b.chatRepo.GoogleCalendarsByIDs(ids)
	if err != nil {
		return nil
	}
	return calendars
}

func googleMenuKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Список", CallbackData: googlePayload(googleActionList)}, {Text: "Добавить", CallbackData: googlePayload(googleActionAdd)}},
			{{Text: "Права", CallbackData: googlePayload(googleActionPermMenu)}},
		},
	}
}

func googleListKeyboard(hasCalendars bool) *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		{{Text: "Обновить", CallbackData: googlePayload(googleActionList)}},
	}
	if hasCalendars {
		rows = append(rows, []telego.InlineKeyboardButton{{Text: "Удалить", CallbackData: googlePayload(googleActionDelete)}})
	}
	rows = append(rows, []telego.InlineKeyboardButton{{Text: "Назад", CallbackData: googlePayload(googleActionMenu)}})
	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func googleAuthKeyboard(url string) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Привязать Google аккаунт", URL: url}},
		},
	}
}

func googleControlCalendarKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Настроить календарь", CallbackData: googlePayload(googleActionMenu)}},
		},
	}
}

func googleBackKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Назад", CallbackData: googlePayload(googleActionMenu)}},
		},
	}
}

func (b *Bot) showGooglePermissionsMenu(u *Update, chat *Chat) error {
	api, err := b.googleUserClient(context.Background(), chat)
	if err != nil {
		return b.showGoogleAuth(u, chat)
	}

	calendars := b.googleCalendars(api)
	if len(calendars) == 0 {
		return b.sendOrEdit(u, "Нет добавленных календарей", googleBackKeyboard())
	}

	rows := make([][]telego.InlineKeyboardButton, 0, len(calendars)+1)
	for _, calendar := range calendars {
		rows = append(rows, []telego.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s, %s", calendar.Type, calendar.Value),
			CallbackData: googlePayload(googleActionPermCal, googleLocalID(calendar.ID)),
		}})
	}
	rows = append(rows, []telego.InlineKeyboardButton{{Text: "Назад", CallbackData: googlePayload(googleActionMenu)}})

	return b.sendOrEdit(u, "Выберите календарь для управления правами:",
		&telego.InlineKeyboardMarkup{InlineKeyboard: rows})
}

func googlePermissionsControl(localID int64) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Дать права редактирования", CallbackData: googlePayload(googleActionPermApply, "writer", googleLocalID(localID))}},
			{{Text: "Снять права редактирования", CallbackData: googlePayload(googleActionPermApply, "reader", googleLocalID(localID))}},
			{{Text: "Назад", CallbackData: googlePayload(googleActionPermMenu)}},
		},
	}
}

func (b *Bot) showGooglePermissionsCalendar(u *Update, chat *Chat, localID int64) error {
	calendar, err := b.chatRepo.GoogleCalendarByLocalID(localID)
	if err != nil || calendar == nil {
		return b.sendOrEdit(u, "Календарь не найден", googleBackKeyboard())
	}

	text := fmt.Sprintf("Календарь: %s, %s\nВыберите действие:", calendar.Type, calendar.Value)
	return b.sendOrEdit(u, text, googlePermissionsControl(calendar.ID))
}

func (b *Bot) applyGooglePermissions(u *Update, chat *Chat, role string, localID int64) error {
	calendar, err := b.chatRepo.GoogleCalendarByLocalID(localID)
	if err != nil || calendar == nil {
		return b.sendOrEdit(u, "Календарь не найден", googleBackKeyboard())
	}

	api, err := b.googleUserClient(context.Background(), chat)
	if err != nil {
		return b.showGoogleAuth(u, chat)
	}

	email := chat.GoogleEmail
	if err := api.SetUserRole(context.Background(), calendar.CalendarID, email, role); err != nil {
		return u.Bot.SendText(u.ChatID, "Не удалось обновить права доступа")
	}

	roleText := "чтения"
	if role == "writer" {
		roleText = "редактирования"
	}
	text := fmt.Sprintf("Права %s успешно обновлены для %s.", roleText, email)
	return b.sendOrEdit(u, text, googlePermissionsControl(calendar.ID))
}

func (b *Bot) showGoogleCalendarAdd(u *Update, chat *Chat) error {
	switch chat.Mode {
	case ModeParent, ModeStudent:
		if chat.Group == "" {
			return u.Bot.AnswerCallback(u.Callback.ID, "Вы ещё не выбрали группу")
		}
	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.AnswerCallback(u.Callback.ID, "Вы ещё не выбрали преподавателя")
		}
	default:
		return u.Bot.AnswerCallback(u.Callback.ID, "Режим чата не позволяет добавить календарь")
	}

	api, err := b.googleUserClient(context.Background(), chat)
	if err != nil {
		return b.showGoogleAuth(u, chat)
	}

	kind := "group"
	value := chat.Group
	if chat.Mode == ModeTeacher {
		kind = "teacher"
		value = chat.Teacher
	}

	calendar, err := b.chatRepo.GoogleCalendarByTypeValue(kind, value)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Не удалось получить календарь")
	}

	created := false
	if calendar == nil {
		if err := b.sendOrEdit(u, "Ожидайте... Идёт создание календаря...", nil); err != nil {
			return err
		}
		owner := "Группа"
		if kind == "teacher" {
			owner = "Преподаватель"
		}
		calendarID, err := api.CreateCalendar(context.Background(), fmt.Sprintf("Расписание занятий (%s - %s)", owner, value))
		if err != nil {
			return u.Bot.SendText(u.ChatID, "Не удалось создать календарь. Сообщите разработчику!")
		}
		calendar = &GoogleCalendar{Type: kind, Value: value, CalendarID: calendarID}
		if err := b.chatRepo.SaveGoogleCalendar(calendar); err != nil {
			return u.Bot.SendText(u.ChatID, "Не удалось сохранить календарь")
		}
		created = true
	}

	if created {
		go func() {
			if _, err := b.resyncGoogleCalendar(context.Background(), calendar); err != nil {
				b.SendText(u.ChatID, "Ошибка синхронизации календаря. Сообщите разработчику!")
			}
		}()
	}

	if err := api.AddCalendarToUser(context.Background(), calendar.CalendarID); err != nil {
		return u.Bot.SendText(u.ChatID, "Не удалось добавить календарь в ваш гугл аккаунт")
	}

	text := "Календарь успешно добавлен в ваш гугл аккаунт!"
	if created {
		text += "\n\nВажно! Так как календарь был создан только что, необходимо время для его полной синхронизации."
	}
	return b.sendOrEdit(u, text, googleBackKeyboard())
}

type GoogleDayChange struct {
	Type  string
	Value string
	Date  string
}

type googleTarget struct {
	Type  string
	Value string
}

func (b *Bot) SyncGoogleCalendars(ctx context.Context) (int, error) {
	if b.google == nil || !b.google.SyncEnabled() {
		return 0, nil
	}
	if !b.googleSyncMu.TryLock() {
		return 0, nil
	}
	defer b.googleSyncMu.Unlock()

	if b.archive == nil {
		return 0, nil
	}
	bounds, err := b.archive.DayIndexBounds()
	if err != nil || bounds.Max < bounds.Min {
		return 0, err
	}

	calendars, err := b.chatRepo.AllGoogleCalendars()
	if err != nil {
		return 0, err
	}

	synced := 0
	var failures []error
	for _, calendar := range calendars {
		if calendar.LastManualSyncedDay >= bounds.Max {
			continue
		}
		from := bounds.Min
		if calendar.LastManualSyncedDay >= bounds.Min {
			from = calendar.LastManualSyncedDay + 1
		}
		days, err := b.resyncGoogleCalendarFrom(ctx, calendar, from)
		synced += days
		if err != nil {
			b.log.Error().Err(err).Str("calendar", calendar.CalendarID).Msg("google calendar reconcile failed")
			failures = append(failures, err)
		}
	}
	return synced, errors.Join(failures...)
}

func (b *Bot) SyncGoogleCalendarChanges(ctx context.Context, changes []GoogleDayChange) (int, error) {
	if b.google == nil || !b.google.SyncEnabled() || len(changes) == 0 {
		return 0, nil
	}
	if !b.googleSyncMu.TryLock() {
		return 0, nil
	}
	defer b.googleSyncMu.Unlock()

	if b.archive == nil {
		return 0, nil
	}
	bounds, err := b.archive.DayIndexBounds()
	if err != nil || bounds.Max < bounds.Min {
		return 0, err
	}

	calendars, err := b.chatRepo.AllGoogleCalendars()
	if err != nil {
		return 0, err
	}
	if len(calendars) == 0 {
		return 0, nil
	}

	byTarget := groupGoogleChanges(changes)
	calls := b.activeCallsSchedule()

	synced := 0
	var failures []error
	for _, calendar := range calendars {
		dates := byTarget[googleTarget{Type: calendar.Type, Value: calendar.Value}]
		if len(dates) == 0 {
			continue
		}
		days, err := b.syncGoogleCalendarDays(ctx, b.archive, calendar, dates, calls, bounds)
		synced += days
		if err != nil {
			b.log.Error().Err(err).Str("calendar", calendar.CalendarID).Msg("google calendar day sync failed")
			failures = append(failures, err)
		}
	}
	return synced, errors.Join(failures...)
}

func groupGoogleChanges(changes []GoogleDayChange) map[googleTarget][]string {
	grouped := make(map[googleTarget][]string)
	seen := make(map[googleTarget]map[string]bool)

	for _, change := range changes {
		if change.Type == "" || change.Value == "" || change.Date == "" {
			continue
		}
		target := googleTarget{Type: change.Type, Value: change.Value}
		if seen[target] == nil {
			seen[target] = make(map[string]bool)
		}
		if seen[target][change.Date] {
			continue
		}
		seen[target][change.Date] = true
		grouped[target] = append(grouped[target], change.Date)
	}
	return grouped
}

func (b *Bot) syncGoogleCalendarDays(ctx context.Context, repo archiveStore, calendar *GoogleCalendar, dates []string, calls google.Schedule, bounds archive.Bounds) (int, error) {
	synced := 0
	var failures []error
	for _, date := range dates {
		dayIndex := archive.DateToDayIndex(date)
		if dayIndex < bounds.Min || dayIndex > bounds.Max {
			continue
		}

		lessons, err := archiveDayLessons(repo, calendar, dayIndex)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := b.google.SyncDay(ctx, calendar.CalendarID, date, lessons, calls); err != nil {
			failures = append(failures, fmt.Errorf("sync %s %s: %w", calendar.Value, date, err))
			continue
		}
		synced++
	}
	return synced, errors.Join(failures...)
}

func archiveDayLessons(repo archiveStore, calendar *GoogleCalendar, dayIndex int64) ([]google.DayLesson, error) {
	switch calendar.Type {
	case "group":
		day, err := repo.GroupDay(dayIndex, calendar.Value)
		if err != nil || day == nil {
			return nil, err
		}
		return groupDayLessons(*day), nil
	case "teacher":
		day, err := repo.TeacherDay(dayIndex, calendar.Value)
		if err != nil || day == nil {
			return nil, err
		}
		return teacherDayLessons(*day), nil
	}
	return nil, nil
}

func (b *Bot) resyncGoogleCalendar(ctx context.Context, calendar *GoogleCalendar) (int, error) {
	return b.resyncGoogleCalendarFrom(ctx, calendar, 0)
}

func (b *Bot) resyncGoogleCalendarFrom(ctx context.Context, calendar *GoogleCalendar, from int64) (int, error) {
	if b.google == nil || !b.google.SyncEnabled() {
		return 0, nil
	}
	if b.archive == nil {
		return 0, nil
	}

	bounds, err := b.archive.DayIndexBounds()
	if err != nil || bounds.Max < bounds.Min {
		return 0, err
	}
	if from < bounds.Min {
		from = bounds.Min
	}

	calls := b.activeCallsSchedule()
	synced := 0

	switch calendar.Type {
	case "group":
		days, err := b.archive.GroupDaysByRange(from, bounds.Max, calendar.Value)
		if err != nil {
			return synced, err
		}
		for _, day := range days {
			if err := b.google.SyncDay(ctx, calendar.CalendarID, day.Day, groupDayLessons(day), calls); err != nil {
				return synced, err
			}
			synced++
		}
	case "teacher":
		days, err := b.archive.TeacherDaysByRange(from, bounds.Max, calendar.Value)
		if err != nil {
			return synced, err
		}
		for _, day := range days {
			if err := b.google.SyncDay(ctx, calendar.CalendarID, day.Day, teacherDayLessons(day), calls); err != nil {
				return synced, err
			}
			synced++
		}
	}

	calendar.LastManualSyncedDay = bounds.Max
	if err := b.chatRepo.SaveGoogleCalendar(calendar); err != nil {
		return synced, err
	}
	return synced, nil
}

func (b *Bot) activeCallsSchedule() google.Schedule {
	campus := b.configuredCampus()
	if campus == "" {
		campus = b.defaultCampus()
	}

	weekdays := b.cache.GetCallsWeekdays()
	saturday := b.cache.GetCallsSaturday()
	if campus != "" {
		if variant, ok := b.cache.GetCallsVariant(campus); ok {
			weekdays = variant.Weekdays
			saturday = variant.Saturday
		}
	}
	if len(weekdays) == 0 {
		weekdays = b.cfg.Timetable.Weekdays
		saturday = b.cfg.Timetable.Saturday
	}
	if len(saturday) == 0 {
		saturday = weekdays
	}
	return google.Schedule{Weekdays: weekdays, Saturday: saturday}
}

func (b *Bot) defaultCampus() string {
	variants := b.cache.GetCalls().SiteVariants
	if len(variants) == 1 && variants[0].Name != "" {
		return variants[0].Name
	}
	return ""
}

func groupDayLessons(day model.GroupDay) []google.DayLesson {
	var out []google.DayLesson
	for index, raw := range day.Lessons {
		lessons := groupLessonsAt(raw)
		if len(lessons) == 0 {
			continue
		}
		title, description, location := google.GroupLessonInfo(lessons)
		out = append(out, google.DayLesson{Index: index, Title: title, Description: description, Location: location})
	}
	return out
}

func groupLessonsAt(raw model.GroupLesson) []google.GroupLesson {
	switch typed := raw.(type) {
	case *model.GroupLessonExplain:
		if lesson := groupLessonFromExplain(typed); lesson.Name != "" {
			return []google.GroupLesson{lesson}
		}
	case []*model.GroupLessonExplain:
		var out []google.GroupLesson
		for _, lesson := range typed {
			if converted := groupLessonFromExplain(lesson); converted.Name != "" {
				out = append(out, converted)
			}
		}
		return out
	case map[string]any:
		if lesson := groupLessonFromMap(typed); lesson.Name != "" {
			return []google.GroupLesson{lesson}
		}
	case []any:
		var out []google.GroupLesson
		for _, item := range typed {
			lesson, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if converted := groupLessonFromMap(lesson); converted.Name != "" {
				out = append(out, converted)
			}
		}
		return out
	}
	return nil
}

func groupLessonFromExplain(lesson *model.GroupLessonExplain) google.GroupLesson {
	if lesson == nil {
		return google.GroupLesson{}
	}
	return google.GroupLesson{
		Subgroup: lesson.Subgroup,
		Name:     lesson.Lesson,
		Type:     stringValue(lesson.Type),
		Teacher:  stringValue(lesson.Teacher),
		Cabinet:  stringValue(lesson.Cabinet),
		Comment:  stringValue(lesson.Comment),
	}
}

func groupLessonFromMap(lesson map[string]any) google.GroupLesson {
	return google.GroupLesson{
		Subgroup: intValue(lesson["subgroup"]),
		Name:     mapString(lesson, "lesson"),
		Type:     mapString(lesson, "type"),
		Teacher:  mapString(lesson, "teacher"),
		Cabinet:  mapString(lesson, "cabinet"),
		Comment:  mapString(lesson, "comment"),
	}
}

func mapString(values map[string]any, key string) string {
	value, ok := values[key].(string)
	if !ok {
		return ""
	}
	return value
}

func intValue(value any) *int {
	switch typed := value.(type) {
	case float64:
		converted := int(typed)
		return &converted
	case int:
		converted := typed
		return &converted
	}
	return nil
}

func teacherDayLessons(day model.TeacherDay) []google.DayLesson {
	var out []google.DayLesson
	for index, lesson := range day.Lessons {
		if lesson == nil || lesson.Lesson == "" {
			continue
		}
		title, description, location := google.TeacherLessonInfo(google.TeacherLesson{
			Subgroup: lesson.Subgroup,
			Group:    lesson.Group,
			Name:     lesson.Lesson,
			Type:     stringValue(lesson.Type),
			Cabinet:  stringValue(lesson.Cabinet),
			Comment:  stringValue(lesson.Comment),
		})
		out = append(out, google.DayLesson{Index: index, Title: title, Description: description, Location: location})
	}
	return out
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (b *Bot) SendGoogleLinked(peerID int64, email string) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: peerID},
		Text:        fmt.Sprintf("Гугл аккаунт '%s' успешно привязан!", email),
		ParseMode:   "HTML",
		ReplyMarkup: googleControlCalendarKeyboard(),
	})
	return err
}

type googleCalCb struct{ bot *Bot }

func (cb *googleCalCb) Prefix() string { return "gcal:" }

func (cb *googleCalCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	action := strings.TrimPrefix(u.Data, "gcal:")
	parts := strings.SplitN(action, ":", 2)
	head := parts[0]

	switch head {
	case googleActionMenu:
		return cb.bot.showGoogleCalendarMenu(u, chat)
	case googleActionList:
		return cb.bot.showGoogleCalendarList(u, chat)
	case googleActionAdd:
		return cb.bot.showGoogleCalendarAdd(u, chat)
	case googleActionDelete:
		return cb.bot.showGoogleCalendarDelete(u, chat)
	case googleActionDrop:
		if len(parts) < 2 {
			return cb.bot.showGoogleCalendarList(u, chat)
		}
		localID, ok := parseGoogleLocalID(parts[1])
		if !ok {
			return cb.bot.showGoogleCalendarList(u, chat)
		}
		return cb.bot.dropGoogleCalendar(u, chat, localID)
	case googleActionPermMenu:
		return cb.bot.showGooglePermissionsMenu(u, chat)
	case googleActionPermCal:
		if len(parts) < 2 {
			return cb.bot.showGooglePermissionsMenu(u, chat)
		}
		localID, ok := parseGoogleLocalID(parts[1])
		if !ok {
			return cb.bot.showGooglePermissionsMenu(u, chat)
		}
		return cb.bot.showGooglePermissionsCalendar(u, chat, localID)
	case googleActionPermApply:
		if len(parts) < 2 {
			return cb.bot.showGooglePermissionsMenu(u, chat)
		}
		role, localID, ok := splitGoogleApply(parts[1])
		if !ok {
			return cb.bot.showGooglePermissionsMenu(u, chat)
		}
		return cb.bot.applyGooglePermissions(u, chat, role, localID)
	}

	return cb.bot.showGoogleCalendarMenu(u, chat)
}

func splitGoogleApply(payload string) (string, int64, bool) {
	index := strings.Index(payload, ":")
	if index < 0 {
		return "", 0, false
	}
	localID, ok := parseGoogleLocalID(payload[index+1:])
	if !ok {
		return "", 0, false
	}
	return payload[:index], localID, true
}

func googleLocalID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func parseGoogleLocalID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
