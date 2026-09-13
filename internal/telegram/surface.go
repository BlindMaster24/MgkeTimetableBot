package telegram

import (
	"os"
	"sort"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parity"
	"github.com/mymmrac/telego"
)

type Layout struct {
	Builder string
	Rows    [][]string
}

type surfaceScenario struct {
	name string
	chat *Chat
}

type builtKeyboard struct {
	name string
	rows func() [][]string
	data func() [][]string
}

func SurfaceOf() parity.Surface {
	return parity.Surface{
		Commands:  SurfaceOfCommands(),
		Callbacks: SurfaceOfCallbacks(),
		Buttons:   SurfaceOfButtons(),
	}
}

func SurfaceOfCommands() []string {
	b := newSurfaceBot()
	out := make([]string, 0, len(b.commandOrder))
	for _, cmd := range b.commandOrder {
		out = append(out, cmd.Name())
	}
	sort.Strings(out)
	return out
}

func SurfaceOfCallbacks() []string {
	b := newSurfaceBot()
	out := make([]string, 0, len(b.callbacks))
	for prefix := range b.callbacks {
		out = append(out, prefix)
	}
	sort.Strings(out)
	return out
}

func SurfaceOfButtons() []string {
	seen := map[string]bool{}
	var out []string
	for _, layout := range SurfaceLayouts() {
		for _, row := range layout.Rows {
			for _, label := range row {
				if seen[label] {
					continue
				}
				seen[label] = true
				out = append(out, label)
			}
		}
	}
	sort.Strings(out)
	return out
}

func SurfaceLayouts() []Layout {
	b := newSurfaceBot()

	seen := map[string]bool{}
	var out []Layout
	for _, scenario := range surfaceScenarios() {
		for _, keyboard := range b.surfaceKeyboards(scenario.chat) {
			rows := keyboard.rows()
			if len(rows) == 0 {
				continue
			}
			key := keyboard.name + "|" + serializeRows(rows)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Layout{Builder: keyboard.name, Rows: rows})
		}
	}
	return out
}

type CallbackLayout struct {
	Builder string
	Rows    [][]string
}

func SurfaceCallbackLayouts() []CallbackLayout {
	b := newSurfaceBot()

	seen := map[string]bool{}
	var out []CallbackLayout
	for _, scenario := range surfaceScenarios() {
		for _, keyboard := range b.surfaceKeyboards(scenario.chat) {
			if keyboard.data == nil {
				continue
			}
			rows := keyboard.data()
			if len(rows) == 0 {
				continue
			}
			key := keyboard.name + "|" + serializeRows(rows)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, CallbackLayout{Builder: keyboard.name, Rows: rows})
		}
	}
	return out
}

func (b *Bot) surfaceKeyboards(chat *Chat) []builtKeyboard {
	reply := func(name string, build func() *telego.ReplyKeyboardMarkup) builtKeyboard {
		return builtKeyboard{name: name, rows: func() [][]string { return replyRows(build()) }}
	}
	inline := func(name string, build func() *telego.InlineKeyboardMarkup) builtKeyboard {
		return builtKeyboard{name: name, rows: func() [][]string { return inlineRows(build()) }, data: func() [][]string { return inlineData(build()) }}
	}

	keyboards := []builtKeyboard{
		reply("replyMainMenu", func() *telego.ReplyKeyboardMarkup { return replyMainMenu(b, chat) }),
		reply("replySettingsMain", b.replySettingsMain),
		reply("replySettingsButtons", func() *telego.ReplyKeyboardMarkup { return b.replySettingsButtons(chat) }),
		reply("replySettingsNotice", func() *telego.ReplyKeyboardMarkup { return b.replySettingsNotice(chat) }),
		reply("replySettingsView", func() *telego.ReplyKeyboardMarkup { return b.replySettingsView(chat) }),
		reply("replySettingsDiff", func() *telego.ReplyKeyboardMarkup { return b.replySettingsDiff(chat) }),
		reply("replySettingsDiffAdvanced", func() *telego.ReplyKeyboardMarkup { return b.replySettingsDiffAdvanced(chat) }),
		reply("replySettingsFormatters", func() *telego.ReplyKeyboardMarkup { return b.replySettingsFormatters(chat) }),
		reply("replySettingsSchedules", b.replySettingsSchedules),
		reply("replySettingsAliases", b.replySettingsAliases),
		reply("replySubscriptionsMenu", b.replySubscriptionsMenu),
		reply("replySelectMode", b.replySelectMode),
		reply("replyCancel", b.replyCancel),
		reply("replyStartButton", b.replyStartButton),
		inline("withCancelButton", func() *telego.InlineKeyboardMarkup { return withCancelButton(nil) }),
		inline("groupHistoryKeyboard", func() *telego.InlineKeyboardMarkup { return groupHistoryKeyboard(chat) }),
		inline("teacherHistoryKeyboard", func() *telego.InlineKeyboardMarkup { return teacherHistoryKeyboard(chat) }),
		inline("verticalValuesKeyboard", func() *telego.InlineKeyboardMarkup {
			return verticalValuesKeyboard([]string{"100", "101"})
		}),
		inline("getWeekTimetableKeyboard", func() *telego.InlineKeyboardMarkup {
			return getWeekTimetableKeyboard("group", "100")
		}),
		inline("weekTimetableButton", func() *telego.InlineKeyboardMarkup {
			return weekTimetableButton("На неделю", "group", "100", 0, true)
		}),
		inline("weekTimetableButton(teacher)", func() *telego.InlineKeyboardMarkup {
			return weekTimetableButton("На неделю", "teacher", "Иванов И.И.", 0, true)
		}),
		inline("callsFullKeyboard", callsFullKeyboard),
		inline("weekControlKeyboard", func() *telego.InlineKeyboardMarkup {
			return b.weekControlKeyboard("group", "100", 0, false)
		}),
		inline("googleMenuKeyboard", googleMenuKeyboard),
		inline("googleListKeyboard(empty)", func() *telego.InlineKeyboardMarkup { return googleListKeyboard(false) }),
		inline("googleListKeyboard(full)", func() *telego.InlineKeyboardMarkup { return googleListKeyboard(true) }),
		inline("googleAuthKeyboard", func() *telego.InlineKeyboardMarkup { return googleAuthKeyboard("https://example.com") }),
		inline("googleBackKeyboard", googleBackKeyboard),
		inline("googleControlCalendarKeyboard", googleControlCalendarKeyboard),
		inline("googlePermissionsControl", func() *telego.InlineKeyboardMarkup { return googlePermissionsControl(1) }),
		inline("weekControlKeyboard(hidePast)", func() *telego.InlineKeyboardMarkup {
			return b.weekControlKeyboardHeader("group", "100", 0, true, true)
		}),
		inline("parserAlertKeyboard", func() *telego.InlineKeyboardMarkup {
			return buttonsKeyboard(notification.HealthAlertButtons(health.AlertParserLayout))
		}),
		inline("calendarAlertKeyboard", func() *telego.InlineKeyboardMarkup {
			return buttonsKeyboard(notification.HealthAlertButtons(health.AlertCalendarFailures))
		}),
		inline("apiAlertKeyboard", func() *telego.InlineKeyboardMarkup {
			return buttonsKeyboard(notification.HealthAlertButtons(health.AlertAPIErrors))
		}),
	}

	if b.cache != nil {
		keyboards = append(keyboards,
			reply("replyCallsSettings(admin)", func() *telego.ReplyKeyboardMarkup { return b.replyCallsSettings(chat, true) }),
			reply("replyCallsSettings(user)", func() *telego.ReplyKeyboardMarkup { return b.replyCallsSettings(chat, false) }),
		)
	}
	return keyboards
}

func surfaceScenarios() []surfaceScenario {
	profiles := []struct {
		name  string
		apply func(*Chat)
	}{
		{"defaults", func(*Chat) {}},
		{"all-on", func(c *Chat) {
			c.ShowDaily = true
			c.ShowWeekly = true
			c.ShowCalls = true
			c.ShowAbout = true
			c.ShowFastGroup = true
			c.ShowFastTeacher = true
		}},
		{"daily", func(c *Chat) { c.ShowDaily = true }},
		{"weekly", func(c *Chat) { c.ShowWeekly = true }},
		{"calls", func(c *Chat) { c.ShowCalls = true }},
		{"about", func(c *Chat) { c.ShowAbout = true }},
		{"fast-group", func(c *Chat) { c.ShowFastGroup = true }},
		{"fast-teacher", func(c *Chat) { c.ShowFastTeacher = true }},
		{"view", func(c *Chat) {
			c.HidePastDays = true
			c.ShowParserTime = true
			c.ShowHints = true
		}},
		{"notice", func(c *Chat) {
			c.NoticeChanges = true
			c.NoticeNextWeek = true
			c.NoticeCalls = true
		}},
		{"diff", func(c *Chat) {
			c.DiffEnabled = true
			c.DiffAutoInWeek = true
			c.DiffAutoInUpdates = true
			c.DiffShowBeforeAfter = true
			c.DiffMaxLines = 25
		}},
		{"history", func(c *Chat) {
			c.HistoryGroup = []string{"100", "101"}
			c.HistoryTeacher = []string{"Иванов И.И."}
		}},
	}

	modes := []struct {
		name  string
		apply func(*Chat)
	}{
		{"unset", func(*Chat) {}},
		{"guest", func(c *Chat) { c.Mode = ModeGuest }},
		{"student", func(c *Chat) { c.Mode = ModeStudent; c.Group = "100" }},
		{"student-no-group", func(c *Chat) { c.Mode = ModeStudent }},
		{"parent", func(c *Chat) { c.Mode = ModeParent; c.Group = "100" }},
		{"teacher", func(c *Chat) { c.Mode = ModeTeacher; c.Teacher = "Иванов И.И." }},
	}

	var scenarios []surfaceScenario
	for _, mode := range modes {
		for _, profile := range profiles {
			chat := &Chat{}
			mode.apply(chat)
			profile.apply(chat)
			scenarios = append(scenarios, surfaceScenario{name: mode.name + "/" + profile.name, chat: chat})
		}
	}
	return scenarios
}

func replyRows(kb *telego.ReplyKeyboardMarkup) [][]string {
	if kb == nil {
		return nil
	}
	rows := make([][]string, 0, len(kb.Keyboard))
	for _, row := range kb.Keyboard {
		labels := make([]string, 0, len(row))
		for _, button := range row {
			labels = append(labels, button.Text)
		}
		rows = append(rows, labels)
	}
	return rows
}

func inlineData(kb *telego.InlineKeyboardMarkup) [][]string {
	if kb == nil {
		return nil
	}
	rows := make([][]string, 0, len(kb.InlineKeyboard))
	for _, row := range kb.InlineKeyboard {
		data := make([]string, 0, len(row))
		for _, button := range row {
			if button.CallbackData != "" {
				data = append(data, button.CallbackData)
			}
		}
		if len(data) == 0 {
			continue
		}
		rows = append(rows, data)
	}
	return rows
}

func inlineRows(kb *telego.InlineKeyboardMarkup) [][]string {
	if kb == nil {
		return nil
	}
	rows := make([][]string, 0, len(kb.InlineKeyboard))
	for _, row := range kb.InlineKeyboard {
		labels := make([]string, 0, len(row))
		for _, button := range row {
			labels = append(labels, button.Text)
		}
		rows = append(rows, labels)
	}
	return rows
}

func serializeRows(rows [][]string) string {
	out := ""
	for _, row := range rows {
		for _, label := range row {
			out += label + "\x1f"
		}
		out += "\x1e"
	}
	return out
}

func newSurfaceBot() *Bot {
	cfg := &config.Config{}
	cfg.Telegram.Token = "0:surface"
	cfg.Telegram.AdminIDs = []int64{1}
	cfg.Calendar.ICS.Enabled = true
	cfg.Google.OAuth.ClientID = "surface"

	var raspCache *cache.RaspCache
	if dir, err := os.MkdirTemp("", "mgke-surface"); err == nil {
		raspCache, _ = cache.New(dir)
	}

	b := &Bot{
		cfg:       cfg,
		log:       logger.New("error", nil),
		i18n:      i18n.New("ru"),
		cache:     raspCache,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
	}
	b.registerAll()
	return b
}
