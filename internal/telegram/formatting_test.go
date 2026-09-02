package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func setupTestBotWithData(t *testing.T) (*Bot, *Repository, *cache.RaspCache) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Telegram.Token = "test:token"
	cfg.Telegram.AdminIDs = []int64{999}
	cfg.Timetable.Weekdays = [][2][2]string{
		{{"08:00", "08:45"}, {"08:55", "09:40"}},
		{{"09:50", "10:35"}, {"10:45", "11:30"}},
		{{"11:50", "12:35"}, {"12:45", "13:30"}},
	}
	cfg.Timetable.Saturday = [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
		{{"10:50", "11:35"}, {"11:55", "12:40"}},
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

	b := &Bot{
		cfg:       cfg,
		log:       log,
		i18n:      loc,
		chatRepo:  chatRepo,
		cache:     raspCache,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
		startTime: time.Now(),
	}
	b.registerAll()
	return b, chatRepo, raspCache
}

func seedTestData(t *testing.T, c *cache.RaspCache) {
	t.Helper()
	now := time.Now()
	groups := map[string]any{
		"100": map[string]any{
			"days": []any{
				map[string]any{
					"day": now.Format("02.01.2006"),
					"lessons": []any{
						map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101"},
						map[string]any{"lesson": "Физика", "type": "ЛР", "cabinet": "202"},
					},
				},
				map[string]any{
					"day":     now.AddDate(0, 0, 1).Format("02.01.2006"),
					"lessons": []any{},
				},
			},
		},
		"200": map[string]any{
			"days": []any{
				map[string]any{
					"day": now.Format("02.01.2006"),
					"lessons": []any{
						map[string]any{"lesson": "Информатика", "type": "Пр", "cabinet": "303"},
					},
				},
			},
		},
	}
	c.SetGroups(groups, "gHash")

	teachers := map[string]any{
		"Иванов И.И.": map[string]any{
			"days": []any{
				map[string]any{
					"day": now.Format("02.01.2006"),
					"lessons": []any{
						map[string]any{"lesson": "Математика", "type": "Лек", "group": "100", "cabinet": "101"},
					},
				},
			},
		},
	}
	c.SetTeachers(teachers, "tHash")

	c.SetCalls(cache.Schedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}, {"08:55", "09:40"}},
			{{"09:50", "10:35"}, {"10:45", "11:30"}},
		},
		Saturday: [][2][2]string{
			{{"09:00", "09:45"}, {"09:55", "10:40"}},
		},
	}, cache.Schedule{}, "site")
}

func makeUpdate(userID int64, text string) *Update {
	return &Update{
		ChatID: userID,
		UserID: userID,
		Text:   text,
	}
}
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{5368709120, "5.00 GB"},
	}
	for _, tc := range tests {
		got := formatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		start    time.Time
		contains string
	}{
		{time.Now().Add(-30 * time.Second), "3"},
		{time.Now().Add(-5 * time.Minute), "5m"},
		{time.Now().Add(-2 * time.Hour), "2h"},
		{time.Now().Add(-2*time.Hour - 30*time.Minute), "2h 30m"},
	}
	for _, tc := range tests {
		got := formatUptime(tc.start)
		if !strings.Contains(got, tc.contains) {
			t.Errorf("formatUptime(%v) = %q, want to contain %q", tc.start, got, tc.contains)
		}
	}
}

func TestIsNowInSlot(t *testing.T) {
	monday := time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC)
	saturday := time.Date(2025, 1, 11, 12, 0, 0, 0, time.UTC)
	sunday := time.Date(2025, 1, 12, 12, 0, 0, 0, time.UTC)
	slot := [2][2]string{{"00:00", "23:59"}, {"00:00", "23:59"}}
	if !isNowInSlot(monday, slot, []int{1, 2, 3, 4, 5}) {
		t.Error("expected weekday slot to match on Monday")
	}
	if isNowInSlot(saturday, slot, []int{1, 2, 3, 4, 5}) {
		t.Error("expected weekday slot to not match on Saturday")
	}
	if isNowInSlot(sunday, slot, []int{1, 2, 3, 4, 5}) {
		t.Error("expected weekday slot to not match on Sunday")
	}
	if !isNowInSlot(saturday, slot, []int{6}) {
		t.Error("expected Saturday slot to match on Saturday")
	}
}

func TestExtractDays(t *testing.T) {
	days := extractDays(nil)
	if days != nil {
		t.Error("expected nil for nil input")
	}

	days = extractDays("not a map")
	if days != nil {
		t.Error("expected nil for string input")
	}

	days = extractDays(map[string]any{"days": "not array"})
	if days != nil {
		t.Error("expected nil for non-array days")
	}

	days = extractDays(map[string]any{
		"days": []any{
			map[string]any{"day": "01.01.2026"},
			map[string]any{"day": "02.01.2026"},
		},
	})
	if len(days) != 2 {
		t.Errorf("expected 2 days, got %d", len(days))
	}
}

func TestGetDayRasp(t *testing.T) {
	days := getDayRasp(nil)
	if days != nil {
		t.Error("expected nil for nil input")
	}

	today := time.Now().Format("02.01.2006")
	daysData := []map[string]any{
		{"day": "01.01.2006"},
		{"day": today},
		{"day": "03.01.2006"},
	}
	days = getDayRasp(daysData)
	if len(days) != 1 || days[0]["day"] != today {
		t.Errorf("expected today %s, got %v", today, days)
	}

	daysData = []map[string]any{
		{"day": "01.01.2006"},
	}
	days = getDayRasp(daysData)
	if len(days) != 1 || days[0]["day"] != "01.01.2006" {
		t.Error("expected fallback to first day")
	}
}

func TestBuildWeekLabel(t *testing.T) {
	label := buildWeekLabel(nil)
	if label != "" {
		t.Error("expected empty for nil days")
	}

	days := []map[string]any{
		{"day": "01.09.2026"},
		{"day": "05.09.2026"},
	}
	label = buildWeekLabel(days)
	if !strings.Contains(label, "Учебная неделя") {
		t.Errorf("expected week label, got %q", label)
	}
	t.Logf("week label: %s", label)
}

func TestRemovePastDays(t *testing.T) {
	today := time.Now().Format("02.01.2006")
	yesterday := time.Now().AddDate(0, 0, -1).Format("02.01.2006")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")

	days := []map[string]any{
		{"day": yesterday, "lessons": []any{map[string]any{"lesson": "X"}}},
		{"day": today, "lessons": []any{map[string]any{"lesson": "Y"}}},
		{"day": tomorrow, "lessons": []any{map[string]any{"lesson": "Z"}}},
	}

	result := removePastDays(days)
	if len(result) != 2 {
		t.Errorf("expected 2 days (today+tomorrow), got %d", len(result))
	}

	daysEmpty := []map[string]any{
		{"day": today, "lessons": []any{}},
		{"day": tomorrow, "lessons": []any{map[string]any{"lesson": "Z"}}},
	}
	result = removePastDays(daysEmpty)
	if len(result) != 1 {
		t.Errorf("expected 1 day (tomorrow, today empty), got %d", len(result))
	}
}

func TestExtractWeekTypeAndValue(t *testing.T) {
	typ, val := extractWeekTypeAndValue("group:100")
	if typ != "group" || val != "100" {
		t.Errorf("expected group:100, got %s:%s", typ, val)
	}
	typ, val = extractWeekTypeAndValue("nocolon")
	if typ != "" || val != "" {
		t.Errorf("expected empty, got %s:%s", typ, val)
	}
}

func TestRandomKey(t *testing.T) {
	key := randomKey(nil)
	if key != "" {
		t.Error("expected empty for nil map")
	}
	key = randomKey(map[string]any{})
	if key != "" {
		t.Error("expected empty for empty map")
	}
	key = randomKey(map[string]any{"only": 1})
	if key != "only" {
		t.Errorf("expected only, got %s", key)
	}
	key = randomKey(map[string]any{"a": 1, "b": 2})
	if key != "a" && key != "b" {
		t.Errorf("expected a or b, got %s", key)
	}
}

func TestSourceCheck(t *testing.T) {
	if got := sourceCheck("S", true); got != "✅ S" {
		t.Errorf("expected ✅ S, got %s", got)
	}
	if got := sourceCheck("S", false); got != "S" {
		t.Errorf("expected S, got %s", got)
	}
}

func TestBotIsAdmin(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	if !b.isAdmin(999) {
		t.Error("expected 999 to be admin")
	}
	if b.isAdmin(123) {
		t.Error("expected 123 to not be admin")
	}
}

func TestBotLocData(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	result := b.locData("group_not_selected", map[string]interface{}{"Group": "100"})
	if result == "" || result == "[group_not_selected]" {
		t.Errorf("expected translated data, got %q", result)
	}
}

func TestGetRandHint(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	hint := b.getRandHint()
	if hint == "" {
		t.Error("expected non-empty hint")
	}
}

func TestGetFormatterOpts(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowHints: true, ShowParserTime: true}
	opts := b.getFormatterOpts(chat)
	if !opts.IsTelegram {
		t.Error("expected IsTelegram true")
	}
	if !opts.ShowParserTime {
		t.Error("expected ShowParserTime true")
	}
	if !opts.ShowHints {
		t.Error("expected ShowHints true")
	}

	chat.ShowHints = false
	opts = b.getFormatterOpts(chat)
	if opts.RandHint != "" {
		t.Error("expected empty hint when ShowHints is false")
	}
}

func TestGetFormatterOptsGroupNotSet(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "", ShowHints: true}
	opts := b.getFormatterOpts(chat)
	if !strings.Contains(opts.RandHint, " групп") {
		t.Errorf("expected group hint, got %q", opts.RandHint)
	}
}

func TestGetFormatterOptsTeacherNotSet(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: "teacher", Teacher: "", ShowHints: true}
	opts := b.getFormatterOpts(chat)
	if !strings.Contains(opts.RandHint, "преподавател") {
		t.Errorf("expected teacher hint, got %q", opts.RandHint)
	}
}

func TestGetFormatterOptsNoButtonsHint(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowHints: true, ShowDaily: false, ShowWeekly: false}
	opts := b.getFormatterOpts(chat)
	if !strings.Contains(opts.RandHint, "кнопк") {
		t.Errorf("expected buttons hint, got %q", opts.RandHint)
	}
}

func TestFmtOpts(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "100"}
	opts := b.fmtOpts(chat, true)
	if !opts.ShowHeader {
		t.Error("expected ShowHeader true")
	}
	opts = b.fmtOpts(chat, false)
	if opts.ShowHeader {
		t.Error("expected ShowHeader false")
	}
}

func TestFormatGroupDay(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	chat := &Chat{Mode: ModeStudent, Group: "100", Formatter: 0}
	data := b.cache.GetGroups()["100"]
	text := b.formatGroupDay(chat, data)
	if text == "" {
		t.Error("expected non-empty formatted day")
	}
	t.Logf("group day:\n%s", text)
}

func TestFormatTeacherDay(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	chat := &Chat{Mode: "teacher", Teacher: "Иванов И.И.", Formatter: 0}
	data := b.cache.GetTeachers()["Иванов И.И."]
	text := b.formatTeacherDay(chat, data)
	if text == "" {
		t.Error("expected non-empty formatted teacher day")
	}
	t.Logf("teacher day:\n%s", text)
}

func TestFormatGroupFull(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	chat := &Chat{Mode: ModeStudent, Group: "100", Formatter: 0}
	data := b.cache.GetGroups()["100"]
	text := b.formatGroupFull(chat, "100", data)
	if text == "" {
		t.Error("expected non-empty formatted full week")
	}
	t.Logf("group full:\n%s", text)
}

func TestFormatTeacherFull(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	chat := &Chat{Mode: "teacher", Teacher: "Иванов И.И.", Formatter: 0}
	data := b.cache.GetTeachers()["Иванов И.И."]
	text := b.formatTeacherFull(chat, "Иванов И.И.", data)
	if text == "" {
		t.Error("expected non-empty formatted teacher full")
	}
	t.Logf("teacher full:\n%s", text)
}

func TestCallsLines(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	slots := [][2][2]string{
		{{"08:00", "08:45"}, {"08:55", "09:40"}},
		{{"09:50", "10:35"}, {"10:45", "11:30"}},
	}
	text := b.callsLines(slots, 2, false, []int{1,2,3,4,5})
	if text == "" {
		t.Error("expected non-empty calls lines")
	}
	if !strings.Contains(text, "1. 08:00") {
		t.Errorf("expected lesson numbering, got %q", text)
	}
	if !strings.Contains(text, "2. 09:50") {
		t.Errorf("expected second lesson, got %q", text)
	}
}

func TestCallsLinesEmpty(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	text := b.callsLines(nil, 0, false, []int{1,2,3,4,5})
	if text != "" {
		t.Errorf("expected empty for nil slots, got %q", text)
	}
}

func TestAllFormattersWithGroupData(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	data := b.cache.GetGroups()["100"]

	for i, f := range formatter.AllFormatters {
		chat := &Chat{Mode: ModeStudent, Group: "100", Formatter: i, ShowHints: false}
		text := f.FormatGroupFull("100", extractDays(data), b.fmtOpts(chat, true))
		if text == "" {
			t.Errorf("formatter %d produced empty output", i)
		}
		t.Logf("formatter %d (%s):\n%s", i, f.Label(), text)
	}
}

func TestAllFormattersWithTeacherData(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	data := b.cache.GetTeachers()["Иванов И.И."]

	for i, f := range formatter.AllFormatters {
		chat := &Chat{Mode: "teacher", Teacher: "Иванов И.И.", Formatter: i, ShowHints: false}
		text := f.FormatTeacherFull("Иванов И.И.", extractDays(data), b.fmtOpts(chat, true))
		if text == "" {
			t.Errorf("formatter %d produced empty output for teacher", i)
		}
	}
}

func TestBuildWeekLabelVariousDates(t *testing.T) {
	days := []map[string]any{
		{"day": "01.09.2026"},
		{"day": "07.09.2026"},
	}
	label := buildWeekLabel(days)
	if !strings.Contains(label, "Учебная неделя") {
		t.Errorf("expected week label, got %q", label)
	}

	days = []map[string]any{
		{"day": "01.12.2026"},
		{"day": "05.12.2026"},
	}
	label = buildWeekLabel(days)
	if !strings.Contains(label, "Учебная неделя") {
		t.Errorf("expected week label, got %q", label)
	}
}

func TestGetFormatterOptsHasParserError(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	b.cache.SetSuccessUpdate(false)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowHints: true}
	opts := b.getFormatterOpts(chat)
	if !opts.HasParserError {
		t.Error("expected HasParserError true")
	}
}



func TestFormatCallsScheduleOnlyWeekdays(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	b.cfg.Timetable.Saturday = nil
	text := b.formatCallsSchedule()
	if !strings.Contains(text, "08:00") {
		t.Error("expected weekday times")
	}
}

func TestFormatCallsScheduleOnlySaturday(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	b.cfg.Timetable.Weekdays = nil
	text := b.formatCallsSchedule()
	if !strings.Contains(text, "09:00") {
		t.Error("expected saturday times")
	}
}

func TestFormatCallsScheduleBoth(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	text := b.formatCallsSchedule()
	if !strings.Contains(text, "08:00") || !strings.Contains(text, "09:00") {
		t.Error("expected both weekday and saturday times")
	}
}

func TestGetRaspCache(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	if b.GetRaspCache() == nil {
		t.Error("expected non-nil cache")
	}
}

func TestSetParseFunc(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	called := false
	b.SetParseFunc(func() error {
		called = true
		return nil
	})
	b.parseFunc()
	if !called {
		t.Error("expected parseFunc to be called")
	}
}

func TestCallbackPrefixesUnique(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	prefixes := make(map[string]string)
	for prefix, cb := range b.callbacks {
		if existing, ok := prefixes[prefix]; ok {
			t.Errorf("duplicate prefix %q: %T and %s", prefix, cb, existing)
		}
		prefixes[prefix] = prefix
	}
}

func TestCommandNamesUnique(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	names := make(map[string]bool)
	for name := range b.commands {
		if names[name] {
			t.Errorf("duplicate command name %s", name)
		}
		names[name] = true
	}
}

func TestAllCommandsHaveDescriptions(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	for name, cmd := range b.commands {
		desc := cmd.Description()
		if desc == "" {
			t.Errorf("command %s has empty description", name)
		}
	}
}

func TestKeyboardMainNotEmpty(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: true, ShowWeekly: true}
	kb := b.mainMenuKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Error("expected non-empty keyboard")
	}
}

func TestKeyboardMainGuestMode(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeGuest}
	kb := b.mainMenuKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Error("expected non-empty keyboard for guest")
	}
}

func TestKeyboardMainNoMode(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ""}
	kb := b.mainMenuKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Error("expected non-empty keyboard for no mode")
	}
}

func TestKeyboardMainAllButtonsDisabled(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: false, ShowWeekly: false, ShowCalls: false, ShowAbout: false, ShowFastGroup: false, ShowFastTeacher: false}
	kb := b.mainMenuKeyboard(chat)
	if kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Error("expected at least settings button")
	}
}

func TestKeyboardSettingsFull(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Mode: ModeStudent}
	kb := b.settingsKeyboardFull(chat)
	if kb == nil || len(kb.InlineKeyboard) < 4 {
		t.Errorf("expected at least 4 rows, got %d", len(kb.InlineKeyboard))
	}
}

func TestKeyboardButtons(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{ShowDaily: true, ShowWeekly: false, ShowCalls: true, ShowAbout: false, ShowFastGroup: true, ShowFastTeacher: false}
	kb := b.buttonsKeyboard(chat)
	if kb == nil {
		t.Fatal("expected non-nil keyboard")
	}
	text := ""
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			text += btn.Text + " "
		}
	}
	if !strings.Contains(text, "✅") || !strings.Contains(text, "🚫") {
		t.Errorf("expected toggle indicators, got %q", text)
	}
}

func TestKeyboardFormatter(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	chat := &Chat{Formatter: 0}
	kb := b.formatterKeyboard(chat)
	if kb == nil {
		t.Fatal("expected non-nil keyboard")
	}
	count := len(kb.InlineKeyboard)
	expectedRows := (len(formatter.AllFormatters)+1)/2 + 1
	if count != expectedRows {
		t.Errorf("expected %d rows, got %d", expectedRows, count)
	}
	text := ""
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			text += btn.Text + " "
		}
	}
	if !strings.Contains(text, "(выбран)") {
		t.Error("expected selected indicator")
	}
}

func TestChatRepoCRUD(t *testing.T) {
	b, repo, _ := setupTestBotWithData(t)
	_ = b

	chat, err := repo.FindOrCreate("telegram", 12345)
	if err != nil {
		t.Fatal(err)
	}
	if chat.PeerID != 12345 {
		t.Errorf("expected peer_id 12345, got %d", chat.PeerID)
	}
	if chat.Mode != "" {
		t.Error("expected empty mode for new chat")
	}

	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.ShowDaily = true
	chat.Formatter = 1
	chat.DiffEnabled = true
	chat.DiffMaxLines = 30
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	chat2, err := repo.FindOrCreate("telegram", 12345)
	if err != nil {
		t.Fatal(err)
	}
	if chat2.Mode != ModeStudent {
		t.Errorf("expected mode student, got %s", chat2.Mode)
	}
	if chat2.Group != "100" {
		t.Errorf("expected group 100, got %s", chat2.Group)
	}
	if !chat2.ShowDaily {
		t.Error("expected ShowDaily true")
	}
	if chat2.Formatter != 1 {
		t.Errorf("expected formatter 1, got %d", chat2.Formatter)
	}
	if !chat2.DiffEnabled {
		t.Error("expected DiffEnabled true")
	}
	if chat2.DiffMaxLines != 30 {
		t.Errorf("expected DiffMaxLines 30, got %d", chat2.DiffMaxLines)
	}
}

func TestChatRepoDifferentServices(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	_, _ = repo.FindOrCreate("telegram", 100)
	chat2, _ := repo.FindOrCreate("viber", 100)
	chat2.Mode = ModeStudent
	chat2.Group = "200"
	repo.Save(chat2)

	chat1check, _ := repo.FindOrCreate("telegram", 100)
	if chat1check.Group != "" {
		t.Error("telegram chat should not be affected by viber")
	}
}

func TestChatRepoToggleBooleans(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	chat, _ := repo.FindOrCreate("telegram", 500)

	boolFields := []struct {
		setter func(*Chat)
		getter func(*Chat) bool
		name   string
	}{
		{func(c *Chat) { c.ShowAbout = true }, func(c *Chat) bool { return c.ShowAbout }, "ShowAbout"},
		{func(c *Chat) { c.ShowDaily = true }, func(c *Chat) bool { return c.ShowDaily }, "ShowDaily"},
		{func(c *Chat) { c.ShowWeekly = true }, func(c *Chat) bool { return c.ShowWeekly }, "ShowWeekly"},
		{func(c *Chat) { c.ShowCalls = true }, func(c *Chat) bool { return c.ShowCalls }, "ShowCalls"},
		{func(c *Chat) { c.ShowFastGroup = true }, func(c *Chat) bool { return c.ShowFastGroup }, "ShowFastGroup"},
		{func(c *Chat) { c.ShowFastTeacher = true }, func(c *Chat) bool { return c.ShowFastTeacher }, "ShowFastTeacher"},
		{func(c *Chat) { c.HidePastDays = true }, func(c *Chat) bool { return c.HidePastDays }, "HidePastDays"},
		{func(c *Chat) { c.DeleteLastMsg = true }, func(c *Chat) bool { return c.DeleteLastMsg }, "DeleteLastMsg"},
		{func(c *Chat) { c.AllowSendMess = true }, func(c *Chat) bool { return c.AllowSendMess }, "AllowSendMess"},
		{func(c *Chat) { c.NoticeChanges = true }, func(c *Chat) bool { return c.NoticeChanges }, "NoticeChanges"},
		{func(c *Chat) { c.NoticeNextWeek = true }, func(c *Chat) bool { return c.NoticeNextWeek }, "NoticeNextWeek"},
		{func(c *Chat) { c.NoticeCalls = true }, func(c *Chat) bool { return c.NoticeCalls }, "NoticeCalls"},
		{func(c *Chat) { c.NoticeParserErrors = true }, func(c *Chat) bool { return c.NoticeParserErrors }, "NoticeParserErrors"},
		{func(c *Chat) { c.ShowParserTime = true }, func(c *Chat) bool { return c.ShowParserTime }, "ShowParserTime"},
		{func(c *Chat) { c.ShowHints = true }, func(c *Chat) bool { return c.ShowHints }, "ShowHints"},
		{func(c *Chat) { c.DiffEnabled = true }, func(c *Chat) bool { return c.DiffEnabled }, "DiffEnabled"},
		{func(c *Chat) { c.DiffAutoInWeek = true }, func(c *Chat) bool { return c.DiffAutoInWeek }, "DiffAutoInWeek"},
		{func(c *Chat) { c.DiffAutoInUpdates = true }, func(c *Chat) bool { return c.DiffAutoInUpdates }, "DiffAutoInUpdates"},
		{func(c *Chat) { c.DiffShowBeforeAfter = true }, func(c *Chat) bool { return c.DiffShowBeforeAfter }, "DiffShowBeforeAfter"},
	}

	for _, bf := range boolFields {
		bf.setter(chat)
		if err := repo.Save(chat); err != nil {
			t.Fatalf("save %s: %v", bf.name, err)
		}
		chat2, _ := repo.FindOrCreate("telegram", 500)
		if !bf.getter(chat2) {
			t.Errorf("%s should be true after set", bf.name)
		}
	}
}

func TestChatRepoFindAllWithNotifications(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat1, _ := repo.FindOrCreate("telegram", 101)
	chat1.Mode = ModeStudent
	chat1.Group = "100"
	chat1.AllowSendMess = true
	chat1.NoticeChanges = true
	repo.Save(chat1)

	chat2, _ := repo.FindOrCreate("telegram", 102)
	chat2.Mode = ModeStudent
	chat2.Group = "200"
	chat2.AllowSendMess = false
	chat2.NoticeChanges = true
	repo.Save(chat2)

	chats, err := repo.FindAllWithNotifications("telegram")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Errorf("expected 1 chat with notifications, got %d", len(chats))
	}
}

func TestChatRepoCountAll(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	c1, _ := repo.FindOrCreate("telegram", 201)
	c1.Mode = ModeStudent
	c1.Group = "100"
	repo.Save(c1)

	c2, _ := repo.FindOrCreate("telegram", 202)
	c2.Mode = "teacher"
	c2.Teacher = "Ivanov"
	repo.Save(c2)

	count, err := repo.CountAll()
	if err != nil {
		t.Fatal(err)
	}
	if count < 2 {
		t.Errorf("expected at least 2 chats, got %d", count)
	}
}

func TestChatRepoCountByMode(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat1, _ := repo.FindOrCreate("telegram", 301)
	chat1.Mode = ModeStudent
	repo.Save(chat1)

	chat2, _ := repo.FindOrCreate("telegram", 302)
	chat2.Mode = "teacher"
	repo.Save(chat2)

	modes, err := repo.CountByMode()
	if err != nil {
		t.Fatal(err)
	}
	if modes["student"] < 1 {
		t.Error("expected at least 1 student")
	}
	if modes["teacher"] < 1 {
		t.Error("expected at least 1 teacher")
	}
}

func TestChatRepoFindGroupsForNotification(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat, _ := repo.FindOrCreate("telegram", 401)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.AllowSendMess = true
	chat.NoticeChanges = true
	repo.Save(chat)

	groups := repo.FindGroupsForNotification("telegram")
	found := false
	for _, g := range groups {
		if g == "100" {
			found = true
		}
	}
	if !found {
		t.Error("expected group 100 in notification groups")
	}
}

func TestChatRepoFindTeachersForNotification(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat, _ := repo.FindOrCreate("telegram", 501)
	chat.Mode = "teacher"
	chat.Teacher = "Иванов И.И."
	chat.AllowSendMess = true
	chat.NoticeChanges = true
	repo.Save(chat)

	teachers := repo.FindTeachersForNotification("telegram")
	found := false
	for _, te := range teachers {
		if te == "Иванов И.И." {
			found = true
		}
	}
	if !found {
		t.Error("expected teacher in notification teachers")
	}
}

func TestChatRepoFindByGroup(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat, _ := repo.FindOrCreate("telegram", 601)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.AllowSendMess = true
	repo.Save(chat)

	chats := repo.FindByGroup("telegram", "100")
	if len(chats) < 1 {
		t.Error("expected at least 1 chat for group 100")
	}
}

func TestChatRepoFindByTeacher(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat, _ := repo.FindOrCreate("telegram", 701)
	chat.Mode = "teacher"
	chat.Teacher = "Петров П.П."
	chat.AllowSendMess = true
	repo.Save(chat)

	chats := repo.FindByTeacher("telegram", "Петров П.П.")
	if len(chats) < 1 {
		t.Error("expected at least 1 chat for teacher")
	}
}

func TestChatRepoFindAllTGChats(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)

	chat, _ := repo.FindOrCreate("telegram", 801)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.AllowSendMess = true
	repo.Save(chat)

	chats, err := repo.FindAllTGChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) < 1 {
		t.Error("expected at least 1 TG chat")
	}
}

func TestCacheCallsRoundTrip(t *testing.T) {
	dir := t.TempDir() + "/cache"
	c, err := cache.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	site := cache.Schedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}, {"08:55", "09:40"}},
			{{"09:50", "10:35"}, {"10:45", "11:30"}},
		},
		Saturday: [][2][2]string{
			{{"09:00", "09:45"}, {"09:55", "10:40"}},
		},
	}
	c.SetCalls(site, cache.Schedule{}, "site")

	calls := c.GetCalls()
	if len(calls.Active.Schedule.Weekdays) != 2 {
		t.Errorf("expected 2 weekday slots, got %d", len(calls.Active.Schedule.Weekdays))
	}
	if len(calls.Active.Schedule.Saturday) != 1 {
		t.Errorf("expected 1 saturday slot, got %d", len(calls.Active.Schedule.Saturday))
	}

	wd := c.GetCallsWeekdays()
	if len(wd) != 2 {
		t.Errorf("expected 2 GetCallsWeekdays, got %d", len(wd))
	}

	sa := c.GetCallsSaturday()
	if len(sa) != 1 {
		t.Errorf("expected 1 GetCallsSaturday, got %d", len(sa))
	}

	c.Save()
	c2, _ := cache.New(dir)
	calls2 := c2.GetCalls()
	if len(calls2.Active.Schedule.Weekdays) != 2 {
		t.Errorf("expected 2 weekday slots after reload, got %d", len(calls2.Active.Schedule.Weekdays))
	}
}

func TestCacheSetCallsFromCache(t *testing.T) {
	dir := t.TempDir() + "/cache"
	c, _ := cache.New(dir)

	calls := c.GetCalls()
	calls.Active.Source = "manual"
	calls.Active.Schedule.Weekdays = [][2][2]string{
		{{"10:00", "10:45"}, {"10:55", "11:40"}},
	}
	c.SetCallsFromCache(calls)

	result := c.GetCalls()
	if result.Active.Source != "manual" {
		t.Errorf("expected source manual, got %s", result.Active.Source)
	}
	if len(result.Active.Schedule.Weekdays) != 1 {
		t.Errorf("expected 1 weekday slot, got %d", len(result.Active.Schedule.Weekdays))
	}
}

func TestCacheReset(t *testing.T) {
	dir := t.TempDir() + "/cache"
	c, _ := cache.New(dir)
	c.SetGroups(map[string]any{"100": "data"}, "h")
	c.SetTeachers(map[string]any{"T1": "data"}, "h2")

	c.Reset()

	if len(c.GetGroups()) != 0 {
		t.Error("expected empty groups after reset")
	}
	if len(c.GetTeachers()) != 0 {
		t.Error("expected empty teachers after reset")
	}
}

func TestRepoDB(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	if repo.DB() == nil {
		t.Error("expected non-nil DB")
	}
}

func TestRepoClose(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	if err := repo.Close(); err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = b
	_ = ctx
}

func TestChatRepoScene(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	chat, _ := repo.FindOrCreate("telegram", 901)
	chat.Scene = "set_group"
	repo.Save(chat)

	chat2, _ := repo.FindOrCreate("telegram", 901)
	if chat2.Scene != "set_group" {
		t.Errorf("expected scene set_group, got %q", chat2.Scene)
	}

	chat2.Scene = ""
	repo.Save(chat2)
	chat3, _ := repo.FindOrCreate("telegram", 901)
	if chat3.Scene != "" {
		t.Errorf("expected empty scene, got %q", chat3.Scene)
	}
}

func TestChatRepoGoogleEmail(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	chat, _ := repo.FindOrCreate("telegram", 911)
	chat.GoogleEmail = "test@gmail.com"
	repo.Save(chat)

	chat2, _ := repo.FindOrCreate("telegram", 911)
	if chat2.GoogleEmail != "test@gmail.com" {
		t.Errorf("expected email test@gmail.com, got %q", chat2.GoogleEmail)
	}
}

func TestChatRepoRef(t *testing.T) {
	_, repo, _ := setupTestBotWithData(t)
	chat, _ := repo.FindOrCreate("telegram", 921)
	chat.Ref = "referral_123"
	chat.DiffMaxLines = 50
	repo.Save(chat)

	chat2, _ := repo.FindOrCreate("telegram", 921)
	if chat2.Ref != "referral_123" {
		t.Errorf("expected ref referral_123, got %q", chat2.Ref)
	}
	if chat2.DiffMaxLines != 50 {
		t.Errorf("expected DiffMaxLines 50, got %d", chat2.DiffMaxLines)
	}
}

func TestShowWeekLabelInFullWeek(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	seedTestData(t, b.cache)
	chat := &Chat{Mode: ModeStudent, Group: "100", Formatter: 0, ShowHints: false}
	data := b.cache.GetGroups()["100"]
	days := extractDays(data)
	text := formatter.GetByIndex(chat.Formatter).FormatGroupFull("", days, formatter.FormatOptions{
		IsTelegram: true,
		ShowHeader: false,
		WeekLabel:  buildWeekLabel(days),
	})
	if !strings.Contains(text, "Учебная неделя") {
		t.Errorf("expected week label in output, got %q", text)
	}
}

func TestFormattersIndexBounds(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	_ = b
	for i := 0; i < len(formatter.AllFormatters); i++ {
		f := formatter.GetByIndex(i)
		if f == nil {
			t.Errorf("GetByIndex(%d) returned nil", i)
		}
	}
	if formatter.GetByIndex(-1) == nil {
		t.Log("GetByIndex(-1) correctly returns nil")
	}
	if formatter.GetByIndex(len(formatter.AllFormatters)) == nil {
		t.Log("GetByIndex(out of bounds) correctly returns nil")
	}
}
