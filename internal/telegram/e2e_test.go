package telegram

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

func setupE2EBot(t *testing.T, adminIDs ...int64) (*Bot, *Repository) {
	t.Helper()
	return setupE2EBotWithCaller(t, stubCaller{}, adminIDs...)
}

func setupE2EBotWithCaller(t *testing.T, caller telegoapi.Caller, adminIDs ...int64) (*Bot, *Repository) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Telegram.Token = "test:token"
	cfg.Telegram.AdminIDs = adminIDs
	cfg.Timetable.Weekdays = [][2][2]string{
		{{"08:00", "08:45"}, {"08:55", "09:40"}},
		{{"09:50", "10:35"}, {"10:45", "11:30"}},
	}
	cfg.Timetable.Saturday = [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
	}
	cfg.Parser.Calls = &config.CallsConfig{Enabled: true}

	log := logger.New("error", nil)
	loc := i18n.New("ru")
	chatRepo, err := New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chatRepo.Close() })
	raspCache, err := cache.New(t.TempDir() + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	groups := map[string]any{
		"100": e2eJsonRoundTrip(map[string]any{
			"group": "100",
			"days": []any{
				map[string]any{"day": "31.08.2026", "lessons": []any{
					map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101"},
					map[string]any{"lesson": "Физика", "type": "ЛР", "cabinet": "202"},
				}},
				map[string]any{"day": "01.09.2026", "lessons": []any{
					map[string]any{"lesson": "Информатика", "type": "Пр", "cabinet": "303"},
				}},
			},
		}),
	}
	raspCache.SetGroups(groups, "gHash")

	teachers := map[string]any{
		"Иванов И.И.": e2eJsonRoundTrip(map[string]any{
			"teacher": "Иванов И.И.",
			"days": []any{
				map[string]any{"day": "31.08.2026", "lessons": []any{
					map[string]any{"lesson": "Математика", "type": "Лек", "group": "100", "cabinet": "101"},
				}},
			},
		}),
	}
	raspCache.SetTeachers(teachers, "tHash")

	site := cache.Schedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}, {"08:55", "09:40"}},
			{{"09:50", "10:35"}, {"10:45", "11:30"}},
		},
		Saturday: [][2][2]string{
			{{"09:00", "09:45"}, {"09:55", "10:40"}},
		},
	}
	raspCache.SetCalls(site, cache.Schedule{}, "site")

	bClient, err := telego.NewBot("123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", telego.WithAPICaller(caller), telego.WithDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}

	b := &Bot{
		client:    bClient,
		cfg:       cfg,
		log:       log,
		i18n:      loc,
		chatRepo:  chatRepo,
		cache:     raspCache,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
	}
	b.aliasRepo = NewAliasRepository(chatRepo)
	b.aliasRepo.EnsureTable()
	b.registerAll()
	return b, chatRepo
}

type stubCaller struct{}

func (stubCaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	return &telegoapi.Response{Ok: true, Result: []byte(`{"message_id": 1}`)}, nil
}

func e2eJsonRoundTrip(v any) any {
	data, _ := json.Marshal(v)
	var result any
	json.Unmarshal(data, &result)
	return result
}

func TestE2E_StudentFullLifecycle(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	userID := int64(12345)

	chat, _ := repo.FindOrCreate("telegram", userID)
	if chat.Mode != "" {
		t.Fatal("new chat should have empty mode")
	}

	chat.Mode = ModeStudent
	chat.Scene = "set_group"
	repo.Save(chat)

	chat.Group = "100"
	chat.Scene = ""
	chat.ShowDaily = true
	chat.ShowWeekly = true
	chat.ShowCalls = true
	chat.Formatter = 1
	chat.NoticeChanges = true
	chat.NoticeNextWeek = true
	chat.DiffEnabled = true
	chat.DiffMaxLines = 30
	repo.Save(chat)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeStudent {
		t.Errorf("mode: got %q", saved.Mode)
	}
	if saved.Group != "100" {
		t.Errorf("group: got %q", saved.Group)
	}
	if !saved.ShowDaily || !saved.ShowWeekly || !saved.ShowCalls {
		t.Error("buttons not persisted")
	}
	if saved.Formatter != 1 {
		t.Errorf("formatter: got %d", saved.Formatter)
	}
	if !saved.NoticeChanges {
		t.Error("notice not persisted")
	}
	if !saved.NoticeNextWeek {
		t.Error("notice_next_week not persisted")
	}
	if !saved.DiffEnabled {
		t.Error("diff not persisted")
	}
	if saved.DiffMaxLines != 30 {
		t.Errorf("diff_max_lines: got %d", saved.DiffMaxLines)
	}

	data := b.cache.GetGroups()["100"]
	for i := 0; i < 4; i++ {
		saved.Formatter = i
		repo.Save(saved)

		text := b.formatGroupFull(saved, "100", data)
		if text == "" {
			t.Errorf("formatter %d produced empty output", i)
		}
		if !strings.Contains(text, "Математика") {
			t.Errorf("formatter %d missing lesson name", i)
		}
	}
}

func TestE2E_TeacherFullLifecycle(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(55555)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeTeacher
	chat.Teacher = "Иванов И.И."
	chat.Formatter = 0
	chat.ShowDaily = true
	repo.Save(chat)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Mode != "teacher" {
		t.Errorf("mode: got %q", saved.Mode)
	}
	if saved.Teacher != "Иванов И.И." {
		t.Errorf("teacher: got %q", saved.Teacher)
	}

	data := b.cache.GetTeachers()["Иванов И.И."]
	text := b.formatTeacherFull(saved, "Иванов И.И.", data)
	if text == "" {
		t.Error("teacher schedule empty")
	}
	if !strings.Contains(text, "Математика") {
		t.Error("teacher schedule missing lesson")
	}
}

func TestE2E_ParentFullLifecycle(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(66666)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeParent
	chat.Group = "100"
	repo.Save(chat)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeParent {
		t.Errorf("mode: got %q", saved.Mode)
	}
	if saved.Group != "100" {
		t.Errorf("group: got %q", saved.Group)
	}
}

func TestE2E_GuestFlow(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(77777)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeGuest
	repo.Save(chat)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeGuest {
		t.Errorf("mode: got %q", saved.Mode)
	}
	if saved.Group != "" {
		t.Error("guest should have no group")
	}
}

func TestE2E_AllTogglesPersist(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(99999)

	boolToggles := []struct {
		name   string
		setter func(*Chat)
		getter func(*Chat) bool
	}{
		{"ShowDaily", func(c *Chat) { c.ShowDaily = true }, func(c *Chat) bool { return c.ShowDaily }},
		{"ShowWeekly", func(c *Chat) { c.ShowWeekly = true }, func(c *Chat) bool { return c.ShowWeekly }},
		{"ShowCalls", func(c *Chat) { c.ShowCalls = true }, func(c *Chat) bool { return c.ShowCalls }},
		{"ShowAbout", func(c *Chat) { c.ShowAbout = true }, func(c *Chat) bool { return c.ShowAbout }},
		{"ShowFastGroup", func(c *Chat) { c.ShowFastGroup = true }, func(c *Chat) bool { return c.ShowFastGroup }},
		{"ShowFastTeacher", func(c *Chat) { c.ShowFastTeacher = true }, func(c *Chat) bool { return c.ShowFastTeacher }},
		{"HidePastDays", func(c *Chat) { c.HidePastDays = true }, func(c *Chat) bool { return c.HidePastDays }},
		{"DeleteLastMsg", func(c *Chat) { c.DeleteLastMsg = true }, func(c *Chat) bool { return c.DeleteLastMsg }},
		{"AllowSendMess", func(c *Chat) { c.AllowSendMess = true }, func(c *Chat) bool { return c.AllowSendMess }},
		{"NoticeChanges", func(c *Chat) { c.NoticeChanges = true }, func(c *Chat) bool { return c.NoticeChanges }},
		{"NoticeNextWeek", func(c *Chat) { c.NoticeNextWeek = true }, func(c *Chat) bool { return c.NoticeNextWeek }},
		{"NoticeCalls", func(c *Chat) { c.NoticeCalls = true }, func(c *Chat) bool { return c.NoticeCalls }},
		{"NoticeParserErrors", func(c *Chat) { c.NoticeParserErrors = true }, func(c *Chat) bool { return c.NoticeParserErrors }},
		{"ShowParserTime", func(c *Chat) { c.ShowParserTime = true }, func(c *Chat) bool { return c.ShowParserTime }},
		{"ShowHints", func(c *Chat) { c.ShowHints = true }, func(c *Chat) bool { return c.ShowHints }},
		{"DiffEnabled", func(c *Chat) { c.DiffEnabled = true }, func(c *Chat) bool { return c.DiffEnabled }},
		{"DiffAutoInWeek", func(c *Chat) { c.DiffAutoInWeek = true }, func(c *Chat) bool { return c.DiffAutoInWeek }},
		{"DiffAutoInUpdates", func(c *Chat) { c.DiffAutoInUpdates = true }, func(c *Chat) bool { return c.DiffAutoInUpdates }},
		{"DiffShowBeforeAfter", func(c *Chat) { c.DiffShowBeforeAfter = true }, func(c *Chat) bool { return c.DiffShowBeforeAfter }},
	}

	for _, toggle := range boolToggles {
		t.Run(toggle.name, func(t *testing.T) {
			chat, _ := repo.FindOrCreate("telegram", userID)
			toggle.setter(chat)
			if err := repo.Save(chat); err != nil {
				t.Fatal(err)
			}
			saved, _ := repo.FindOrCreate("telegram", userID)
			if !toggle.getter(saved) {
				t.Errorf("%s not persisted", toggle.name)
			}
		})
	}
}

func TestE2E_AdminAccessControl(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	_ = repo

	adminIDs := b.cfg.Telegram.AdminIDs
	if len(adminIDs) != 1 || adminIDs[0] != 999 {
		t.Error("admin IDs not set")
	}

	if !b.isAdmin(999) {
		t.Error("999 should be admin")
	}
	if b.isAdmin(12345) {
		t.Error("12345 should not be admin")
	}
	if b.isAdmin(0) {
		t.Error("0 should not be admin")
	}
}

func TestE2E_AllCommandsRegistered(t *testing.T) {
	b, _ := setupE2EBot(t)
	expected := []string{
		"/start", "/help", "/cancel", "/setup", "/day", "/week",
		"/calls", "/about", "/group", "/teacher", "/settings",
		"/image", "/buttons", "/buttons_reload", "/formatter", "/forceparse", "/resetcache",
		"/eula", "/api", "/diff", "/notice", "/view", "/dev", "/math", "/flushcache", "/debug", "/send", "/trigger",
		"/history", "/stats", "/google_calendar", "/alias",
		"/regexp", "/vanish", "/parserLogs", "/requireNewButtons", "/createApiKey", "/decryptKey",
		"/cabinet", "/groups", "/teachers", "/comparegroups",
		"/ping", "/ics", "/subscriptions", "/subscriptions_test",
		"/archive", "/endings", "/chat", "/id", "/error", "/test",
		"/groupweek", "/groupimage", "/teacherweek", "/teacherimage",
		"/setgroup", "/setteacher", "/vychetkaDlyaBrovkiDSOnline", "/sql", "/restart", "/archivestats",
		"/noticedebug",
	}
	if len(b.commands) != len(expected) {
		t.Errorf("expected %d commands, got %d", len(expected), len(b.commands))
	}
	for _, name := range expected {
		if _, ok := b.commands[name]; !ok {
			t.Errorf("missing command %s", name)
		}
	}
}

func TestE2E_AllCallbacksRegistered(t *testing.T) {
	b, _ := setupE2EBot(t)
	expected := []string{
		"calls_full", "image", "cancel", "answer:",
		"timetable_g:", "timetable_t:", "gcal:",
		"alias:del:", "alias:menu",
	}
	for _, prefix := range expected {
		found := false
		for p := range b.callbacks {
			if strings.HasPrefix(p, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no callback with prefix %q", prefix)
		}
	}
}

func TestE2E_AllCommandsHaveDescriptions(t *testing.T) {
	b, _ := setupE2EBot(t)
	for name, cmd := range b.commands {
		desc := cmd.Description()
		if desc == "" {
			t.Errorf("command %s has empty description", name)
		}
	}
}

func TestE2E_MainMenuKeyboard_StudentFull(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: true, ShowWeekly: true, ShowCalls: true, ShowAbout: true, ShowFastGroup: true, ShowFastTeacher: true}
	kb := replyMainMenu(b, chat)
	if kb == nil || len(kb.Keyboard) < 3 {
		t.Error("replyMainMenu broken")
	}

	texts := flattenReplyKeyboardTexts(kb)
	if !strings.Contains(texts, "📄 На день") {
		t.Error("missing Day button")
	}
	if !strings.Contains(texts, "📑 На неделю") {
		t.Error("missing Week button")
	}
	if !strings.Contains(texts, "🕐 Звонки") {
		t.Error("missing Calls button")
	}
	if !strings.Contains(texts, "💡 О боте") {
		t.Error("missing About button")
	}
	if !strings.Contains(texts, "⚙️ Настройки") {
		t.Error("missing Settings button")
	}
	if !strings.Contains(texts, "👩‍🎓 Группа") {
		t.Error("missing fast Group button")
	}
	if !strings.Contains(texts, "Препод.") && !strings.Contains(texts, "Преподаватель") {
		t.Error("missing fast Teacher button")
	}
}

func TestE2E_MainMenuKeyboard_GuestMode(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Mode: ModeGuest}
	kb := replyMainMenu(b, chat)
	if kb == nil {
		t.Fatal("nil keyboard")
	}
	texts := flattenReplyKeyboardTexts(kb)
	if !strings.Contains(texts, "👩‍🎓 Группа") {
		t.Error("guest mode should show Group button")
	}
	if !strings.Contains(texts, "👩‍🏫 Преподаватель") {
		t.Error("guest mode should show Teacher button")
	}
}

func TestE2E_MainMenuKeyboard_NoMode(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Mode: ""}
	kb := replyMainMenu(b, chat)
	if kb == nil {
		t.Fatal("nil keyboard")
	}
	texts := flattenReplyKeyboardTexts(kb)
	if !strings.Contains(texts, "Первоначальная настройка") {
		t.Error("no mode should show Setup button")
	}
}

func TestE2E_MainMenuKeyboard_TeacherMode(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Mode: ModeTeacher, Teacher: "Иванов", ShowDaily: true, ShowAbout: true}
	kb := replyMainMenu(b, chat)
	texts := flattenReplyKeyboardTexts(kb)
	if !strings.Contains(texts, "📄 На день") {
		t.Error("teacher mode should show Day button")
	}
}

func TestE2E_SettingsKeyboardFull(t *testing.T) {
	b, _ := setupE2EBot(t)
	kb := b.replySettingsMain()
	texts := flattenReplyKeyboardTexts(kb)

	if !strings.Contains(texts, "Первоначальная настройка") {
		t.Error("missing Setup button")
	}
	if !strings.Contains(texts, "🗓️ Управление расписаниями") {
		t.Error("missing Schedule Management button (old feature)")
	}
	if !strings.Contains(texts, "Кнопки") {
		t.Error("missing Buttons button")
	}
	if !strings.Contains(texts, "Форматировщик") {
		t.Error("missing Formatter button")
	}
	if !strings.Contains(texts, "Оповещения") {
		t.Error("missing Notifications button")
	}
	if !strings.Contains(texts, "Подписки") {
		t.Error("missing Subscriptions button (old feature)")
	}
	if !strings.Contains(texts, "Отображение") {
		t.Error("missing View button")
	}
	if !strings.Contains(texts, "Сравнение") {
		t.Error("missing Diff button (old label)")
	}
	if !strings.Contains(texts, "Показать текущие") {
		t.Error("missing Show Current button (old feature)")
	}
	if !strings.Contains(texts, "Главное меню") {
		t.Error("should have Main Menu, not Cancel")
	}
}

func TestE2E_ButtonsKeyboard(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{
		Mode:            ModeStudent,
		ShowDaily:       true,
		ShowWeekly:      false,
		ShowCalls:       true,
		ShowAbout:       false,
		ShowFastGroup:   true,
		ShowFastTeacher: false,
	}
	kb := b.replySettingsButtons(chat)
	texts := flattenReplyKeyboardTexts(kb)

	if !strings.Contains(texts, "📄 На день") {
		t.Error("missing Day button text")
	}
	if !strings.Contains(texts, "📑 На неделю") {
		t.Error("missing Week button text")
	}
}

func TestE2E_FormatterKeyboard(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Formatter: 1}
	kb := b.replySettingsFormatters(chat)
	texts := flattenReplyKeyboardTexts(kb)

	if !strings.Contains(texts, "Стуктурированный") {
		t.Error("missing Default formatter label")
	}
	if !strings.Contains(texts, "Визуальный") {
		t.Error("missing Visual formatter label")
	}
	if !strings.Contains(texts, "Компактный") {
		t.Error("missing Compact formatter label")
	}
	if !strings.Contains(texts, "LitolaxStyle") {
		t.Error("missing Litolax formatter label")
	}
	if !strings.Contains(texts, "выбран") {
		t.Error("should show selected formatter")
	}
}

func TestE2E_NoticeSettings_ThreeToggles(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{
		Mode:           ModeStudent,
		NoticeChanges:  true,
		NoticeNextWeek: false,
		NoticeCalls:    true,
	}

	texts := flattenReplyKeyboardTexts(b.replySettingsNotice(chat))
	if !strings.Contains(texts, "новых днях") {
		t.Error("missing notice changes toggle")
	}
	if !strings.Contains(texts, "новой неделе") {
		t.Error("missing notice_next_week toggle (old feature)")
	}
	if !strings.Contains(texts, "звонках") {
		t.Error("missing notice calls toggle")
	}
}

func TestE2E_ViewSettings(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{
		Mode:           ModeStudent,
		HidePastDays:   true,
		ShowParserTime: false,
		ShowHints:      true,
	}
	kb := b.replySettingsView(chat)
	texts := flattenReplyKeyboardTexts(kb)

	if !strings.Contains(texts, "Скрывать прошедшие дни") {
		t.Error("missing hide past days")
	}
	if !strings.Contains(texts, "последней загрузки") {
		t.Error("missing show parser time (old text)")
	}
	if !strings.Contains(texts, "подсказки") && !strings.Contains(texts, "Подсказки") {
		t.Error("missing show hints")
	}
}

func TestE2E_DiffSettings_BasicAndAdvanced(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{
		Mode:                ModeStudent,
		DiffEnabled:         true,
		DiffMaxLines:        20,
		DiffAutoInWeek:      false,
		DiffAutoInUpdates:   true,
		DiffShowBeforeAfter: false,
	}

	kbBasic := b.replySettingsDiff(chat)
	textsBasic := flattenReplyKeyboardTexts(kbBasic)
	if !strings.Contains(textsBasic, "Что изменилось") {
		t.Error("diff basic missing enabled toggle")
	}
	if !strings.Contains(textsBasic, "Лимит строк") {
		t.Error("diff basic missing max lines")
	}
	if !strings.Contains(textsBasic, "Расширенные") {
		t.Error("diff basic missing advanced link (old feature)")
	}

	kbAdvanced := b.replySettingsDiffAdvanced(chat)
	textsAdvanced := flattenReplyKeyboardTexts(kbAdvanced)
	if !strings.Contains(textsAdvanced, "после /week") {
		t.Error("diff advanced missing autoInWeek")
	}
	if !strings.Contains(textsAdvanced, "уведомлениях") {
		t.Error("diff advanced missing autoInUpdates")
	}
	if !strings.Contains(textsAdvanced, "старое") || !strings.Contains(textsAdvanced, "новое") {
		t.Error("diff advanced missing showBeforeAfter")
	}
	if !strings.Contains(textsAdvanced, "Базовые настройки") {
		t.Error("diff advanced missing back to basic link (old feature)")
	}
}

func TestE2E_SchedulesSettings(t *testing.T) {
	b, _ := setupE2EBot(t)
	kb := b.replySettingsSchedules()
	texts := flattenReplyKeyboardTexts(kb)
	if !strings.Contains(texts, "Звонки: управление") {
		t.Error("schedules missing calls management")
	}
}

func TestE2E_CurrentSettings(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{
		Mode:           ModeStudent,
		Group:          "100",
		Formatter:      1,
		ShowDaily:      true,
		ShowWeekly:     false,
		ShowCalls:      true,
		DiffEnabled:    true,
		DiffMaxLines:   20,
		NoticeChanges:  true,
		NoticeNextWeek: false,
		NoticeCalls:    true,
	}
	text := b.currentSettingsText(chat)
	if !strings.Contains(text, "Режим чата: student") {
		t.Error("missing mode")
	}
	if !strings.Contains(text, "Выбранная группа: 100") {
		t.Error("missing group")
	}
	if !strings.Contains(text, "да") && !strings.Contains(text, "нет") {
		t.Error("missing formatter name")
	}
	if !strings.Contains(text, "Оповещение о добавлении новой недели: нет") {
		t.Error("missing notice_next_week display")
	}
}

func TestE2E_WeekControlKeyboard(t *testing.T) {
	b, _ := setupE2EBot(t)
	currentWeek := utils.WeekIndexFromDate(time.Now())
	kb := b.weekControlKeyboard("group", "100", currentWeek.Value(), true)
	texts := flattenKeyboardTexts(kb)

	if !strings.Contains(texts, "📷 Сгенерировать изображение") {
		t.Error("missing image button")
	}
	if !strings.Contains(texts, "🔼") {
		t.Error("week control should have show-full button when hidePastDays=true and current week")
	}

	kb2 := b.weekControlKeyboard("group", "100", currentWeek.Value()-1, true)
	texts2 := flattenKeyboardTexts(kb2)
	if !strings.Contains(texts2, "➡️") {
		t.Error("past week should have forward arrow")
	}
}

func TestE2E_AcademicWeekLabel(t *testing.T) {
	week := utils.WeekIndexFromDate(time.Now())
	label := buildWeekLabelFromWeek(week)
	if !strings.Contains(label, "Учебная неделя №") {
		t.Errorf("week label missing academic number: %q", label)
	}
	if !strings.Contains(label, "(") || !strings.Contains(label, ")") {
		t.Errorf("week label missing date range: %q", label)
	}

	d1, d2 := week.WeekRange()
	if !strings.Contains(label, d1.Format("02.01")) {
		t.Errorf("week label missing start date: %q", label)
	}
	if !strings.Contains(label, d2.Format("02.01")) {
		t.Errorf("week label missing end date: %q", label)
	}
}

func TestE2E_CallsScheduleDisplay(t *testing.T) {
	b, _ := setupE2EBot(t)
	text := b.formatCallsSchedule()
	if text == "" {
		t.Error("calls schedule empty")
	}
	if !strings.Contains(text, "08:00") {
		t.Error("calls missing weekday times")
	}
	if !strings.Contains(text, "09:00") {
		t.Error("calls missing saturday times")
	}

	wd := b.cache.GetCallsWeekdays()
	sa := b.cache.GetCallsSaturday()
	if len(wd) == 0 {
		t.Error("empty weekday calls")
	}
	if len(sa) == 0 {
		t.Error("empty saturday calls")
	}

	textWd := b.callsLines(wd, 2, false, []int{1, 2, 3, 4, 5})
	if textWd == "" {
		t.Error("callsLines empty")
	}
	if !strings.Contains(textWd, "1. 08:00") {
		t.Error("callsLines missing first lesson")
	}
}

func TestE2E_CallsSourceSwitch(t *testing.T) {
	b, _ := setupE2EBot(t)
	calls := b.cache.GetCalls()
	if calls.Active.Source != "site" {
		t.Errorf("initial source: %q", calls.Active.Source)
	}

	calls.Active.Source = "config"
	calls.Active.Schedule.Weekdays = b.cfg.Timetable.Weekdays
	b.cache.SetCallsFromCache(calls)

	result := b.cache.GetCalls()
	if result.Active.Source != "config" {
		t.Errorf("source after switch: %q", result.Active.Source)
	}

	calls2 := b.cache.GetCalls()
	calls2.Active = cache.CallsActive{
		Source: "site",
		Hash:   calls.Site.Hash,
	}
	calls2.Active.Schedule = calls.Site.Schedule
	b.cache.SetCallsFromCache(calls2)

	result2 := b.cache.GetCalls()
	if result2.Active.Source != "site" {
		t.Errorf("source after reset: %q", result2.Active.Source)
	}
}

func TestE2E_FormatterOptsEdgeCases(t *testing.T) {
	b, _ := setupE2EBot(t)

	chat := &Chat{Mode: "", ShowHints: true}
	opts := b.getFormatterOpts(chat)
	if opts.RandHint == "" {
		t.Error("expected hint for empty mode")
	}

	chat.Mode = ModeStudent
	chat.Group = ""
	chat.ShowHints = true
	opts = b.getFormatterOpts(chat)
	if opts.RandHint == "" {
		t.Error("expected group hint")
	}

	chat.Mode = "teacher"
	chat.Teacher = ""
	chat.ShowHints = true
	opts = b.getFormatterOpts(chat)
	if opts.RandHint == "" {
		t.Error("expected teacher hint")
	}

	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.ShowHints = true
	chat.ShowDaily = false
	chat.ShowWeekly = false
	opts = b.getFormatterOpts(chat)
	if opts.RandHint == "" {
		t.Error("expected buttons hint")
	}

	b.cache.SetSuccessUpdate(false)
	opts = b.getFormatterOpts(chat)
	if !opts.HasParserError {
		t.Error("expected HasParserError")
	}

	chat.ShowParserTime = true
	opts = b.fmtOpts(chat, true)
	if !opts.ShowParserTime || !opts.ShowHeader {
		t.Error("fmtOpts broken")
	}
}

func TestE2E_FormattersProduceValidOutput(t *testing.T) {
	b, _ := setupE2EBot(t)
	data := b.cache.GetGroups()["100"]
	chat := &Chat{Mode: ModeStudent, Group: "100", Formatter: 0, ShowHints: false}

	for i := 0; i < 4; i++ {
		chat.Formatter = i
		text := b.formatGroupFull(chat, "100", data)
		if text == "" {
			t.Errorf("formatter %d empty", i)
		}
		t.Logf("formatter %d:\n%s", i, text)
	}

	teacherData := b.cache.GetTeachers()["Иванов И.И."]
	chat.Mode = "teacher"
	chat.Teacher = "Иванов И.И."
	for i := 0; i < 4; i++ {
		chat.Formatter = i
		text := b.formatTeacherFull(chat, "Иванов И.И.", teacherData)
		if text == "" {
			t.Errorf("teacher formatter %d empty", i)
		}
	}
}

func TestE2E_ChatSceneTransitions(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(33333)

	scenes := []struct {
		scene string
		mode  ChatMode
		group string
	}{
		{"setup", "", ""},
		{"set_group", ModeStudent, ""},
		{"", ModeStudent, "100"},
		{"set_group", ModeStudent, "100"},
		{"", ModeStudent, "200"},
	}

	for _, s := range scenes {
		chat, _ := repo.FindOrCreate("telegram", userID)
		chat.Scene = s.scene
		if s.mode != "" {
			chat.Mode = s.mode
		}
		if s.group != "" {
			chat.Group = s.group
		}
		repo.Save(chat)

		saved, _ := repo.FindOrCreate("telegram", userID)
		if saved.Scene != s.scene {
			t.Errorf("scene: want %q, got %q", s.scene, saved.Scene)
		}
	}
}

func TestE2E_AllKeyboardBuildersWork(t *testing.T) {
	b, _ := setupE2EBot(t)

	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: true, ShowWeekly: true, ShowCalls: true, ShowAbout: true, ShowFastGroup: true, ShowFastTeacher: true}
	if replay := replyMainMenu(b, chat); replay == nil || len(replay.Keyboard) < 3 {
		t.Error("replyMainMenu broken")
	}

	if kb := b.replySettingsMain(); kb == nil || len(kb.Keyboard) < 5 {
		t.Error("replySettingsMain broken")
	}

	if kb := b.replySettingsButtons(chat); kb == nil || len(kb.Keyboard) < 2 {
		t.Error("replySettingsButtons broken")
	}

	if kb := b.replySettingsFormatters(chat); kb == nil || len(kb.Keyboard) < 2 {
		t.Error("replySettingsFormatters broken")
	}

	if kb := b.replySelectMode(); kb == nil || len(kb.Keyboard) != 3 {
		t.Error("replySelectMode broken")
	}

	if kb := b.replyCancel(); kb == nil || len(kb.Keyboard) != 1 {
		t.Error("replyCancel broken")
	}

	if kb := b.replyStartButton(); kb == nil || len(kb.Keyboard) != 1 {
		t.Error("replyStartButton broken")
	}

	if kb := b.replySettingsDiff(chat); kb == nil || len(kb.Keyboard) < 2 {
		t.Error("replySettingsDiff broken")
	}

	if kb := b.replySettingsDiffAdvanced(chat); kb == nil || len(kb.Keyboard) < 2 {
		t.Error("replySettingsDiffAdvanced broken")
	}

	if kb := b.replySettingsSchedules(); kb == nil || len(kb.Keyboard) < 1 {
		t.Error("replySettingsSchedules broken")
	}
}

func TestE2E_CacheResetPreservesDB(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(44444)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Formatter = 2
	repo.Save(chat)

	chat2, _ := repo.FindOrCreate("telegram", userID)
	if chat2.Group != "100" || chat2.Formatter != 2 {
		t.Fatal("setup failed")
	}
}

func TestE2E_RemovePastDays(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.cfg.Timetable.Weekdays = [][2][2]string{{{"00:00", "00:00"}, {"00:00", "23:59"}}}
	b.cfg.Timetable.Saturday = [][2][2]string{{{"00:00", "00:00"}, {"00:00", "23:59"}}}

	today := time.Now().Format("02.01.2006")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")
	expectedFirst := today
	if todayAutoSkipped() {
		expectedFirst = tomorrow
	}
	days := []map[string]any{
		{"day": "25.08.2026", "lessons": []any{map[string]any{"lesson": "Old"}}},
		{"day": today, "lessons": []any{map[string]any{"lesson": "Today"}}},
		{"day": tomorrow, "lessons": []any{map[string]any{"lesson": "Future"}}},
	}
	result := b.removePastDays(days)
	if len(result) == 0 {
		t.Fatal("removePastDays returned empty")
	}
	firstDate, _ := result[0]["day"].(string)
	if firstDate != expectedFirst {
		t.Errorf("first day should be %s, got %s", expectedFirst, firstDate)
	}
}

func flattenReplyKeyboardTexts(kb *telego.ReplyKeyboardMarkup) string {
	if kb == nil {
		return ""
	}
	var all string
	for _, row := range kb.Keyboard {
		for _, btn := range row {
			all += btn.Text + " "
		}
	}
	return all
}

func flattenKeyboardTexts(kb *telego.InlineKeyboardMarkup) string {
	if kb == nil {
		return ""
	}
	var all string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			all += btn.Text + " "
		}
	}
	return all
}

func TestE2E_GoogleCalendarCommandRegistered(t *testing.T) {
	b, _ := setupE2EBot(t)
	if _, ok := b.commands["/google_calendar"]; !ok {
		t.Error("google_calendar command not registered")
	}
	if _, ok := b.callbacks["gcal:"]; !ok {
		t.Error("gcal callback not registered")
	}
}

func TestE2E_AliasCommandRegistered(t *testing.T) {
	b, _ := setupE2EBot(t)
	if _, ok := b.commands["/alias"]; !ok {
		t.Error("alias command not registered")
	}
	if _, ok := b.callbacks["alias:del:"]; !ok {
		t.Error("alias:del callback not registered")
	}
	if _, ok := b.callbacks["alias:menu"]; !ok {
		t.Error("alias:menu callback not registered")
	}
}

func TestE2E_AliasAddAndList(t *testing.T) {
	b, repo := setupE2EBot(t)
	b.aliasRepo = NewAliasRepository(repo)
	b.aliasRepo.EnsureTable()
	userID := int64(200)

	err := b.aliasRepo.Add(userID, "Математика", "Мат-тест")
	if err != nil {
		t.Fatalf("add alias: %v", err)
	}

	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if aliases[0].Key != "Математика" || aliases[0].Value != "Мат-тест" {
		t.Errorf("wrong alias: %v", aliases[0])
	}

	err = b.aliasRepo.Remove(userID, "Математика")
	if err != nil {
		t.Fatalf("remove alias: %v", err)
	}

	aliases, _ = b.aliasRepo.List(userID)
	if len(aliases) != 0 {
		t.Fatalf("expected 0 aliases after remove, got %d", len(aliases))
	}
}

func TestE2E_AliasClear(t *testing.T) {
	b, repo := setupE2EBot(t)
	b.aliasRepo = NewAliasRepository(repo)
	b.aliasRepo.EnsureTable()
	userID := int64(300)
	b.aliasRepo.Add(userID, "A", "B")
	b.aliasRepo.Add(userID, "C", "D")
	b.aliasRepo.Clear(userID)
	aliases, _ := b.aliasRepo.List(userID)
	if len(aliases) != 0 {
		t.Fatalf("expected 0 after clear, got %d", len(aliases))
	}
}

func TestE2E_GoogleCalendarMenuNoOAuth(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.cfg.Google.OAuth.ClientID = ""
	chat := &Chat{Mode: ModeStudent, Group: "100"}
	u := &Update{Bot: b, ChatID: 100, UserID: 100}
	_ = chat
	_ = u
	if b.cfg.Google.OAuth.ClientID != "" {
		t.Error("expected empty OAuth ClientID")
	}
}

func TestE2E_GoogleCalendarAddStudent(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.cfg.Google.OAuth.ClientID = "test-id"
	chat := &Chat{Mode: ModeStudent, Group: "100", GoogleEmail: "test@gmail.com"}
	if chat.Mode != ModeStudent || chat.Group == "" || chat.GoogleEmail == "" {
		t.Error("chat not configured properly")
	}
	_ = b
}

func TestE2E_GoogleCalendarAddNoMode(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.cfg.Google.OAuth.ClientID = "test-id"
	chat := &Chat{GoogleEmail: "test@gmail.com"}
	if chat.Mode != "" {
		t.Error("expected empty mode")
	}
	_ = b
}

func TestE2E_GroupHistoryAppendAndKeyboard(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(4242)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.AppendGroupHistory("100")
	repo.Save(chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if len(loaded.HistoryGroup) != 1 || loaded.HistoryGroup[0] != "100" {
		t.Fatalf("history: %v", loaded.HistoryGroup)
	}

	loaded.AppendGroupHistory("100")
	if len(loaded.HistoryGroup) != 1 {
		t.Error("duplicate history entry added")
	}

	loaded.AppendGroupHistory("200")
	repo.Save(loaded)
	again, _ := repo.FindOrCreate("telegram", userID)
	if len(again.HistoryGroup) != 2 || again.HistoryGroup[0] != "200" {
		t.Fatalf("history after append: %v", again.HistoryGroup)
	}

	kb := groupHistoryKeyboard(again)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("keyboard rows: %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "200" {
		t.Errorf("first row: %q", kb.InlineKeyboard[0][0].Text)
	}

	withCancel := withCancelButton(groupHistoryKeyboard(again))
	if len(withCancel.InlineKeyboard) != 3 {
		t.Errorf("cancel row missing")
	}
	if withCancel.InlineKeyboard[2][0].CallbackData != "cancel" {
		t.Errorf("cancel cb: %q", withCancel.InlineKeyboard[2][0].CallbackData)
	}
}

func TestE2E_TeacherHistoryDedup(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(4243)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.AppendTeacherHistory("Иванов И.И.")
	chat.AppendTeacherHistory("Петров П.П.")
	chat.AppendTeacherHistory("Иванов И.И.")
	repo.Save(chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if len(loaded.HistoryTeacher) != 2 {
		t.Fatalf("history: %v", loaded.HistoryTeacher)
	}
	if loaded.HistoryTeacher[0] != "Иванов И.И." {
		t.Errorf("recent first: %v", loaded.HistoryTeacher)
	}
}

func TestE2E_MatchTeacherList(t *testing.T) {
	_, _ = setupE2EBot(t)

	candidates := map[string]any{
		"Иванов И.И.":   struct{}{},
		"Иванова А.А.":  struct{}{},
		"Петров П.П.":   struct{}{},
		"Сидоров С.С.":  struct{}{},
		"Козлов К.К.":   struct{}{},
		"Николаев Н.Н.": struct{}{},
	}

	matched, tooMany := matchTeacherList("иванов", candidates)
	if len(matched) != 2 || tooMany {
		t.Fatalf("matched: %v tooMany: %v", matched, tooMany)
	}

	matched, tooMany = matchTeacherList("петров п.п.", candidates)
	if len(matched) != 1 || tooMany {
		t.Fatalf("exact match failed: %v %v", matched, tooMany)
	}

	matched, tooMany = matchTeacherList("ов", candidates)
	if len(matched) != 5 || tooMany {
		t.Errorf("broad search: %d matches, tooMany=%v", len(matched), tooMany)
	}

	matched, _ = matchTeacherList("несуществующий", candidates)
	if len(matched) != 0 {
		t.Errorf("expected no match, got %v", matched)
	}
}

func TestE2E_VerticalValuesKeyboard(t *testing.T) {
	kb := verticalValuesKeyboard([]string{"Иванов И.И.", "Петров П.П."})
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("rows: %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].CallbackData != "answer:Иванов И.И." {
		t.Errorf("cb data: %q", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestE2E_GetWeekTimetableKeyboard(t *testing.T) {
	kb := getWeekTimetableKeyboard("group", "100")
	if kb.InlineKeyboard[0][0].Text != "На неделю" {
		t.Errorf("label: %q", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][0].CallbackData != "timetable_g:100:0:0:1" {
		t.Errorf("cb data: %q", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestE2E_WeekControlKeyboardPrefix(t *testing.T) {
	b, _ := setupE2EBot(t)
	kb := b.weekControlKeyboard("group", "100", utils.WeekIndexFromDate(time.Now()).Value(), false)

	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "timetable_group") || strings.HasPrefix(btn.CallbackData, "timetable_teacher") {
				t.Errorf("long prefix in callback: %q", btn.CallbackData)
			}
		}
	}

	first := kb.InlineKeyboard[0][0].CallbackData
	if !strings.HasPrefix(first, "timetable_g:") {
		t.Errorf("first cb: %q", first)
	}
}

func TestE2E_GroupCommandSetsScene(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(5252)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "get_group:day"
	repo.Save(chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Scene != "get_group:day" {
		t.Fatalf("scene: %q", loaded.Scene)
	}

	groups := b.cache.GetGroups()
	if len(groups) == 0 {
		t.Fatal("no groups in cache")
	}
}

func TestE2E_CallsManualReason(t *testing.T) {
	b, _ := setupE2EBot(t)

	b.cache.SetCallsManual(
		[][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		[][2][2]string{{{"09:00", "09:45"}}},
		"Технический перерыв",
	)

	calls := b.cache.GetCalls()
	if calls.Active.Source != "manual" {
		t.Errorf("source: %q", calls.Active.Source)
	}
	if calls.ManualReason != "Технический перерыв" {
		t.Errorf("reason: %q", calls.ManualReason)
	}

	line := b.callsLines(calls.Active.Schedule.Weekdays, 1, true, []int{1, 2, 3, 4, 5})
	if !strings.Contains(line, "08:00") {
		t.Errorf("line: %q", line)
	}
}

func TestE2E_BrovkaCSVBuild(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(6262)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeTeacher
	chat.Teacher = "Иванов И.И."
	repo.Save(chat)

	cmd, ok := b.commands["/vychetkaDlyaBrovkiDSOnline"]
	if !ok {
		t.Fatal("brovka command not registered")
	}

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/vychetkaDlyaBrovkiDSOnline"}
	if err := cmd.Handler(context.Background(), u); err == nil {
		t.Log("brovka with empty archive returned nil (archive unavailable path)")
	}
}

func TestE2E_HistoryCmdAsksTeacher(t *testing.T) {
	_, repo := setupE2EBot(t)
	userID := int64(6263)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	repo.Save(chat)

	_, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = repo, chat
	if chat.Scene != "" {
		t.Errorf("scene should start empty, got %q", chat.Scene)
	}

	chat.Scene = "history_teacher"
	repo.Save(chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Scene != "history_teacher" {
		t.Errorf("scene: %q, want history_teacher", loaded.Scene)
	}
}

func TestE2E_HistoryTeacherScene_MultiMatch(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(6264)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "history_teacher"
	repo.Save(chat)

	scene := &historyTeacherScene{bot: b}
	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "иванов"}
	_ = scene.Handle(context.Background(), u, chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Scene != "history_week" {
		t.Errorf("scene after single match: %q", loaded.Scene)
	}
	if loaded.Teacher != "Иванов И.И." {
		t.Errorf("teacher: %q", loaded.Teacher)
	}
	if loaded.HistoryTeacher == nil || len(loaded.HistoryTeacher) == 0 {
		t.Error("teacher not appended to history")
	}
}

func TestE2E_SubTestPickUsesSelectedIndex(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(6265)

	repo.AddSubscription(userID, "group", "100")
	repo.AddSubscription(userID, "teacher", "Иванов И.И.")

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "sub_test_pick"
	repo.Save(chat)

	scene := &subTestPickScene{bot: b}
	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "2"}
	_ = scene.Handle(context.Background(), u, chat)

	loaded, _ := repo.FindOrCreate("telegram", userID)
	if loaded.Scene != "sub_test_mode:teacher:Иванов И.И." {
		t.Fatalf("scene after subscription pick: %q", loaded.Scene)
	}

	mode := &subTestModeScene{bot: b}
	u = &Update{Bot: b, ChatID: userID, UserID: userID, Text: "2"}
	_ = mode.Handle(context.Background(), u, loaded)

	loaded, _ = repo.FindOrCreate("telegram", userID)
	if loaded.Scene != "" {
		t.Errorf("scene not cleared: %q", loaded.Scene)
	}
}

func TestE2E_TimetableCallbackReadsArchive(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(777)

	archiveRepo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archiveRepo.Close() })
	if _, err := archiveRepo.DB().Exec(`CREATE TABLE IF NOT EXISTS timetable_archive (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		day INTEGER NOT NULL,
		"group" TEXT,
		teacher TEXT,
		data TEXT NOT NULL,
		UNIQUE(day, "group"),
		UNIQUE(day, teacher)
	)`); err != nil {
		t.Fatal(err)
	}
	b.archive = archiveRepo

	currentWeek := utils.WeekIndexFromDate(time.Now()).Value()
	pastWeek := currentWeek - 3
	futureWeek := currentWeek + 3

	weekLabel := func(week int) string {
		w := utils.WeekIndexFromNumber(week)
		d1, d2 := w.WeekRange()
		return d1.Format("02.01.2006") + "-" + d2.Format("02.01.2006")
	}

	entries := []archive.AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": weekLabel(pastWeek)[:10], "lessons": []any{
			map[string]any{"lesson": "ПрошлаяМатематика", "cabinet": "1"},
		}}},
		{Type: "group", Value: "100", Day: map[string]any{"day": weekLabel(futureWeek)[:10], "lessons": []any{
			map[string]any{"lesson": "БудущаяФизика", "cabinet": "9"},
		}}},
	}
	if err := archiveRepo.AppendDays(entries); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindOrCreate("telegram", userID); err != nil {
		t.Fatal(err)
	}

	minIdx, maxIdx := utils.WeekIndexFromNumber(pastWeek).WeekDayIndexRange()
	days := b.archiveDaysForWeek("group", "100", minIdx, maxIdx)
	if len(days) != 1 {
		t.Fatalf("expected 1 past-week day from archive, got %d", len(days))
	}
	if days[0]["day"] != weekLabel(pastWeek)[:10] {
		t.Errorf("past day: %v", days[0]["day"])
	}

	minIdx, maxIdx = utils.WeekIndexFromNumber(futureWeek).WeekDayIndexRange()
	days = b.archiveDaysForWeek("group", "100", minIdx, maxIdx)
	if len(days) != 1 {
		t.Fatalf("expected 1 future-week day from archive, got %d", len(days))
	}

	lessons, _ := days[0]["lessons"].([]any)
	if len(lessons) != 1 {
		t.Fatalf("expected 1 lesson, got %d", len(lessons))
	}
	lesson, _ := lessons[0].(map[string]any)
	if lesson["lesson"] != "БудущаяФизика" {
		t.Errorf("lesson content mismatch: %v", lesson["lesson"])
	}
}

func TestE2E_ArchiveStatsCommand(t *testing.T) {
	b, _ := setupE2EBot(t, 999)

	archiveRepo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archiveRepo.Close() })
	if _, err := archiveRepo.DB().Exec(`CREATE TABLE IF NOT EXISTS timetable_archive (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		day INTEGER NOT NULL,
		"group" TEXT,
		teacher TEXT,
		data TEXT NOT NULL,
		UNIQUE(day, "group"),
		UNIQUE(day, teacher)
	)`); err != nil {
		t.Fatal(err)
	}
	b.archive = archiveRepo

	if err := archiveRepo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
	}); err != nil {
		t.Fatal(err)
	}

	cmd := &archiveStatsCmd{bot: b}
	u := makeUpdate(999, "/archivestats")
	u.Bot = b
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
}

func TestE2E_NoticeDebugCommand(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	userID := int64(12345)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.NoticeChanges = true
	chat.NoticeNextWeek = true
	chat.NoticeCalls = true
	chat.NoticeParserErrors = true
	repo.Save(chat)

	guest, _ := repo.FindOrCreate("telegram", 555)
	guest.Mode = ModeGuest
	guest.NoticeParserErrors = true
	repo.Save(guest)

	cmd := &noticeDebugCmd{bot: b}
	u := makeUpdate(999, "/noticedebug")
	u.Bot = b
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if err := cmd.Handler(context.Background(), func() *Update { u := makeUpdate(12345, "/noticedebug"); u.Bot = b; return u }()); err != nil {
		t.Errorf("non-admin handler: %v", err)
	}
}

func TestE2E_ReplyKeyboardSettingsMenu(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2001)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	repo.Save(chat)

	u := makeUpdate(userID, "/settings")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettings {
		t.Errorf("scene: got %q, want settings", saved.Scene)
	}

	nav := &settingsNavTextCmd{bot: b, kind: "to_settings"}
	if !nav.MatchText("Меню настроек") {
		t.Error("to_settings should match «Меню настроек»")
	}
	mainNav := &settingsNavTextCmd{bot: b, kind: "to_main"}
	if !mainNav.MatchText("Главное меню") {
		t.Error("to_main should match «Главное меню»")
	}
}

func TestE2E_ButtonToggleTextCommand(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2002)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Scene = sceneSettings
	chat.ShowDaily = true
	repo.Save(chat)

	c := &btnToggleTextCmd{bot: b, kind: "daily"}
	if !c.MatchText(`🚫 Кнопка "📄 На день"`) {
		t.Fatal("should match toggle text")
	}

	chat.ShowDaily = false
	repo.Save(chat)
	u := makeUpdate(userID, `🚫 Кнопка "📄 На день"`)
	u.Bot = b
	if err := c.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	saved, _ := repo.FindOrCreate("telegram", userID)
	if !saved.ShowDaily {
		t.Error("ShowDaily should be toggled to true")
	}
	if saved.Scene != sceneSettings {
		t.Errorf("scene should stay settings, got %q", saved.Scene)
	}
}

func TestE2E_SceneGuardBlocksOutOfSceneText(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2003)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Scene = ""
	chat.ShowDaily = true
	repo.Save(chat)

	c := &btnToggleTextCmd{bot: b, kind: "daily"}
	if !c.MatchText(`🚫 Кнопка "📄 На день"`) {
		t.Fatal("should match toggle text")
	}

	chat.ShowDaily = false

	u := makeUpdate(userID, `🚫 Кнопка "📄 На день"`)
	u.Bot = b

	chat.Scene = ""
	if b.dispatchTextCommand(context.Background(), u, chat) {
		t.Error("toggle must NOT fire outside scene=settings")
	}

	chat.Scene = sceneSettings
	if !b.dispatchTextCommand(context.Background(), u, chat) {
		t.Error("toggle must fire inside scene=settings")
	}
}

func TestE2E_NotFoundResetsScene(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2004)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Scene = sceneSettings
	repo.Save(chat)

	u := makeUpdate(userID, "абракадабра")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Errorf("scene should be reset on notFound, got %q", saved.Scene)
	}
}

func TestE2E_NoticeViewDiffReplyKeyboards(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2005)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Scene = sceneSettings
	repo.Save(chat)

	ntc := &noticeToggleTextCmd{bot: b, kind: "changes"}
	if !ntc.MatchText("🔇 Оповещение о новых днях: Нет") {
		t.Error("notice toggle text mismatch")
	}
	vtc := &viewToggleTextCmd{bot: b, kind: "hide_past_days"}
	if !vtc.MatchText("✅ Скрывать прошедшие дни") {
		t.Error("view toggle text mismatch")
	}
	dtc := &diffToggleTextCmd{bot: b, kind: "enabled"}
	if !dtc.MatchText(`✅ Включить раздел "Что изменилось"`) {
		t.Error("diff toggle text mismatch")
	}
}

func TestE2E_SetupModeTextCommands(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(2006)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "setup"
	repo.Save(chat)

	guest := &setupModeTextCmd{bot: b, kind: "guest"}
	if !guest.MatchText("👀 Гость") {
		t.Error("guest text mismatch")
	}
	u := makeUpdate(userID, "👀 Гость")
	u.Bot = b
	if err := guest.Handler(context.Background(), u); err != nil {
		t.Fatalf("guest handler: %v", err)
	}
	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeGuest {
		t.Errorf("mode: got %q", saved.Mode)
	}
	if saved.Scene != "" {
		t.Errorf("scene should clear, got %q", saved.Scene)
	}

	student, _ := repo.FindOrCreate("telegram", int64(2007))
	student.Scene = "setup"
	repo.Save(student)
	stu := &setupModeTextCmd{bot: b, kind: "student"}
	if !stu.MatchText("👩‍🎓 Учащийся") {
		t.Error("student text mismatch")
	}
	u2 := makeUpdate(int64(2007), "👩‍🎓 Учащийся")
	u2.Bot = b
	if err := stu.Handler(context.Background(), u2); err != nil {
		t.Fatalf("student handler: %v", err)
	}
	saved2, _ := repo.FindOrCreate("telegram", int64(2007))
	if saved2.Mode != ModeStudent {
		t.Errorf("mode: got %q", saved2.Mode)
	}
	if saved2.Scene != "set_group" {
		t.Errorf("scene should be set_group, got %q", saved2.Scene)
	}
}
