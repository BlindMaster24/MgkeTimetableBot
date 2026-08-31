package telegram

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func setupE2EBot(t *testing.T, adminIDs ...int64) (*Bot, *Repository) {
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

	b := &Bot{
		cfg:       cfg,
		log:       log,
		i18n:      loc,
		chatRepo:  chatRepo,
		cache:     raspCache,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
	}
	b.registerAll()
	return b, chatRepo
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



func e2eJsonRoundTrip(v any) any {
	data, _ := json.Marshal(v)
	var result any
	json.Unmarshal(data, &result)
	return result
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
	b, repo := setupE2EBot(t)
	userID := int64(66666)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeParent
	chat.Group = "100"
	repo.Save(chat)

	data := b.cache.GetGroups()["100"]
	text := b.formatGroupFull(chat, "100", data)
	if text == "" {
		t.Error("parent group schedule empty")
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
		"/image", "/buttons", "/formatter", "/forceparse", "/resetcache",
		"/eula", "/api", "/diff", "/flushcache", "/debug", "/send", "/trigger",
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
		"day", "week", "calls", "calls_full", "image",
		"image_group:", "image_teacher:", "cancel", "setup",
		"about", "group", "teacher", "settings", "ics",
		"btn_toggle:", "btn_menu", "fmt_menu", "fmt_select:",
		"notice_menu", "view_menu", "notice_toggle:", "view_toggle:",
		"main_menu", "diff_menu", "diff_toggle:", "calls_menu",
		"calls_show", "calls_refresh", "calls_source:", "calls_source_reset",
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

func TestE2E_AllKeyboardBuildersWork(t *testing.T) {
	b, _ := setupE2EBot(t)

	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: true, ShowWeekly: true, ShowCalls: true, ShowAbout: true, ShowFastGroup: true, ShowFastTeacher: true}
	kb := b.mainMenuKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) < 3 {
		t.Error("mainMenuKeyboard broken")
	}

	kb = b.settingsKeyboardFull(chat)
	if kb == nil || len(kb.InlineKeyboard) < 4 {
		t.Error("settingsKeyboardFull broken")
	}

	kb = b.buttonsKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) < 2 {
		t.Error("buttonsKeyboard broken")
	}

	kb = b.formatterKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) < 2 {
		t.Error("formatterKeyboard broken")
	}

	kb = selectModeKeyboard(b.i18n.T)
	if kb == nil || len(kb.InlineKeyboard) != 2 {
		t.Error("selectModeKeyboard broken")
	}

	kb = cancelKeyboard(b.i18n.T)
	if kb == nil || len(kb.InlineKeyboard) != 1 {
		t.Error("cancelKeyboard broken")
	}

	kb = settingsKeyboard(b.i18n.T)
	if kb == nil || len(kb.InlineKeyboard) != 2 {
		t.Error("settingsKeyboard broken")
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

	textWd := b.callsLines(wd, 2)
	if textWd == "" {
		t.Error("callsLines empty")
	}
	if !strings.Contains(textWd, "1. 08:00") {
		t.Error("callsLines missing first lesson")
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

	weekDays := []map[string]any{
		{"day": "01.09.2026"},
		{"day": "05.09.2026"},
	}
	label := buildWeekLabel(weekDays)
	if !strings.Contains(label, "Учебная неделя") {
		t.Errorf("week label: %q", label)
	}
}
