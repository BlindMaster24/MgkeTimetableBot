package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/mymmrac/telego"
)

func onOffStr(v bool) string {
	if v {
		return "включено"
	}
	return "выключено"
}

func yesNoStr(v bool) string {
	if v {
		return "да"
	}
	return "нет"
}

type dayCb struct{ bot *Bot }

func (cb *dayCb) Prefix() string { return "day" }
func (cb *dayCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showDaySchedule(u, chat)
	})
}

type weekCb struct{ bot *Bot }

func (cb *weekCb) Prefix() string { return "week" }
func (cb *weekCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showWeekSchedule(u, chat)
	})
}

type callsCb struct{ bot *Bot }

func (cb *callsCb) Prefix() string { return "calls" }
func (cb *callsCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		cb.bot.showCallsFull(u, chat)
		return nil
	})
}

type callsFullCb struct{ bot *Bot }

func (cb *callsFullCb) Prefix() string { return "calls_full" }
func (cb *callsFullCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		cb.bot.showCallsFullFull(u, chat)
		return nil
	})
}

type imageCb struct{ bot *Bot }

func (cb *imageCb) Prefix() string { return "image" }
func (cb *imageCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	if chat.Mode == ModeStudent || chat.Mode == ModeParent {
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
		}
		data, ok := cb.bot.cache.GetGroups()[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("group_not_exists"))
		}
		path, err := imagepkg.RenderGroupFromCache(chat.Group, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}
	if chat.Mode == "teacher" {
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("need_teacher"))
		}
		data, ok := cb.bot.cache.GetTeachers()[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("teacher_not_exists"))
		}
		path, err := imagepkg.RenderTeacherFromCache(chat.Teacher, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type imageGroupCb struct{ bot *Bot }

func (cb *imageGroupCb) Prefix() string { return "image_group:" }
func (cb *imageGroupCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	group := strings.TrimPrefix(u.Data, "image_group:")
	data, ok := cb.bot.cache.GetGroups()[group]
	if !ok {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("group_not_exists"))
	}
	path, err := imagepkg.RenderGroupFromCache(group, data, "./cache/images")
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
	}
	return u.Bot.SendPhoto(u.ChatID, path, "")
}

type imageTeacherCb struct{ bot *Bot }

func (cb *imageTeacherCb) Prefix() string { return "image_teacher:" }
func (cb *imageTeacherCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	teacher := strings.TrimPrefix(u.Data, "image_teacher:")
	data, ok := cb.bot.cache.GetTeachers()[teacher]
	if !ok {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("teacher_not_exists"))
	}
	path, err := imagepkg.RenderTeacherFromCache(teacher, data, "./cache/images")
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
	}
	return u.Bot.SendPhoto(u.ChatID, path, "")
}

type cancelCb struct{ bot *Bot }

func (cb *cancelCb) Prefix() string { return "cancel" }
func (cb *cancelCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		wasSetup := chat.Scene == "setup"
		chat.Scene = ""
		cb.bot.chatRepo.Save(chat)
		if wasSetup {
			return u.Bot.SendTextWithKeyboard(u.ChatID, cb.bot.loc("about_bot"), cb.bot.mainMenuKeyboard(chat))
		}
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("input_cancelled"))
}

type setupCb struct{ bot *Bot }

func (cb *setupCb) Prefix() string { return "setup" }
func (cb *setupCb) Handler(ctx context.Context, u *Update) error {
	mode := strings.TrimPrefix(u.Data, "setup:")
	mode = strings.TrimPrefix(mode, ":")

	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	switch mode {
	case "student":
		chat.Mode = ModeStudent
		chat.Scene = "set_group"
	case "teacher":
		chat.Mode = "teacher"
		chat.Scene = "set_teacher"
	case "parent":
		chat.Mode = ModeParent
		chat.Scene = "set_group"
	case "guest":
		chat.Mode = "guest"
		chat.Scene = ""
	default:
		cb.bot.AnswerCallback(u.Callback.ID, "")
		return u.Bot.SendTextWithKeyboard(u.ChatID, cb.bot.loc("setup_select_mode"), selectModeKeyboard(cb.bot.i18n.T))
	}

	cb.bot.chatRepo.Save(chat)
	cb.bot.AnswerCallback(u.Callback.ID, "")

	switch mode {
	case "student", "parent":
		return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_group_number"))
	case "teacher":
		return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_teacher_name"))
	default:
		return u.Bot.SendText(u.ChatID, cb.bot.loc("mode_guest"))
	}
}

type aboutCb struct{ bot *Bot }

func (cb *aboutCb) Prefix() string { return "about" }
func (cb *aboutCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return u.Bot.SendText(u.ChatID, cb.bot.loc("about_bot"))
}

type groupCb struct{ bot *Bot }

func (cb *groupCb) Prefix() string { return "group" }
func (cb *groupCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "set_group"
		cb.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_group_number"))
}

type teacherCb struct{ bot *Bot }

func (cb *teacherCb) Prefix() string { return "teacher" }
func (cb *teacherCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "set_teacher"
		cb.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_teacher_name"))
}

type settingsCb struct{ bot *Bot }

func (cb *settingsCb) Prefix() string { return "settings" }
func (cb *settingsCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return u.Bot.SendTextWithKeyboard(u.ChatID, cb.bot.loc("settings_menu"), cb.bot.settingsKeyboardFull(chat))
	})
}

type icsCb struct{ bot *Bot }

func (cb *icsCb) Prefix() string { return "ics" }
func (cb *icsCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type btnToggleCb struct{ bot *Bot }

func (cb *btnToggleCb) Prefix() string { return "btn_toggle:" }
func (cb *btnToggleCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {

	field := strings.TrimPrefix(u.Data, "btn_toggle:")
	var msg string
	switch field {
	case "show_daily":
		chat.ShowDaily = !chat.ShowDaily
		msg = fmt.Sprintf("Показывать кнопку \"📄 На день\"? Установлено: '%s'", yesNoStr(chat.ShowDaily))
	case "show_weekly":
		chat.ShowWeekly = !chat.ShowWeekly
		msg = fmt.Sprintf("Показывать кнопку \"📑 На неделю\"? Установлено: '%s'", yesNoStr(chat.ShowWeekly))
	case "show_calls":
		chat.ShowCalls = !chat.ShowCalls
		msg = fmt.Sprintf("Показывать кнопку \"🕐 Звонки\"? Установлено: '%s'", yesNoStr(chat.ShowCalls))
	case "show_about":
		chat.ShowAbout = !chat.ShowAbout
		msg = fmt.Sprintf("Показывать кнопку \"💡 О боте\"? Установлено: '%s'", yesNoStr(chat.ShowAbout))
	case "show_fast_group":
		chat.ShowFastGroup = !chat.ShowFastGroup
		msg = fmt.Sprintf("Показывать кнопку \"👩‍🎓 Группа\"? Установлено: '%s'", yesNoStr(chat.ShowFastGroup))
	case "show_fast_teacher":
		chat.ShowFastTeacher = !chat.ShowFastTeacher
		msg = fmt.Sprintf("Показывать кнопку \"👩‍🏫 Преподаватель\"? Установлено: '%s'", yesNoStr(chat.ShowFastTeacher))
	}

	cb.bot.chatRepo.Save(chat)
	if msg != "" {
		return cb.bot.SendTextWithKeyboard(u.ChatID, msg, cb.bot.buttonsKeyboard(chat))
	}
	return cb.bot.SendTextWithKeyboard(u.ChatID, "Меню настройки кнопок.", cb.bot.buttonsKeyboard(chat))
	})
}

type btnMenuCb struct{ bot *Bot }

func (cb *btnMenuCb) Prefix() string { return "btn_menu" }
func (cb *btnMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return u.Bot.SendTextWithKeyboard(u.ChatID, "Настройка кнопок", cb.bot.buttonsKeyboard(chat))
	})
}

type fmtMenuCb struct{ bot *Bot }

func (cb *fmtMenuCb) Prefix() string { return "fmt_menu" }
func (cb *fmtMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настройки форматировщика.", cb.bot.formatterKeyboard(chat))
	})
}

type fmtSelectCb struct{ bot *Bot }

func (cb *fmtSelectCb) Prefix() string { return "fmt_select:" }
func (cb *fmtSelectCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {

	idxStr := strings.TrimPrefix(u.Data, "fmt_select:")
	idx := 0
	for _, c := range idxStr {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		}
	}

	if idx >= 0 && idx < len(formatter.AllFormatters) {
		chat.Formatter = idx
		cb.bot.chatRepo.Save(chat)
	}

	return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настройки форматировщика.", cb.bot.formatterKeyboard(chat))
	})
}

type noticeMenuCb struct{ bot *Bot }

func (cb *noticeMenuCb) Prefix() string { return "notice_menu" }
func (cb *noticeMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showNoticeSettings(u, chat)
	})
}

type viewMenuCb struct{ bot *Bot }

func (cb *viewMenuCb) Prefix() string { return "view_menu" }
func (cb *viewMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showViewSettings(u, chat)
	})
}

type mainMenuCb struct{ bot *Bot }

func (cb *mainMenuCb) Prefix() string { return "main_menu" }
func (cb *mainMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return u.Bot.SendTextWithKeyboard(u.ChatID, cb.bot.loc("main_menu"), cb.bot.mainMenuKeyboard(chat))
	})
}

type noticeToggleCb struct{ bot *Bot }

func (cb *noticeToggleCb) Prefix() string { return "notice_toggle:" }
func (cb *noticeToggleCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {

	field := strings.TrimPrefix(u.Data, "notice_toggle:")
	var msg string
	switch field {
	case "notice_changes":
		chat.NoticeChanges = !chat.NoticeChanges
		msg = fmt.Sprintf("Оповещение о добавлении нового дня: %s", onOffStr(chat.NoticeChanges))
	case "notice_next_week":
		chat.NoticeNextWeek = !chat.NoticeNextWeek
		msg = fmt.Sprintf("Оповещение о добавлении новой недели: %s", onOffStr(chat.NoticeNextWeek))
	case "notice_calls":
		chat.NoticeCalls = !chat.NoticeCalls
		msg = fmt.Sprintf("Оповещение об изменениях расписания звонков: %s", onOffStr(chat.NoticeCalls))
	case "notice_parser_errors":
		chat.NoticeParserErrors = !chat.NoticeParserErrors
		msg = fmt.Sprintf("Оповещение об ошибке парсера: %s", onOffStr(chat.NoticeParserErrors))
	}

	cb.bot.chatRepo.Save(chat)
	if msg != "" {
		return cb.bot.SendTextWithKeyboard(u.ChatID, msg, cb.bot.noticeKeyboard(chat))
	}
	return cb.bot.showNoticeSettings(u, chat)
	})
}

type viewToggleCb struct{ bot *Bot }

func (cb *viewToggleCb) Prefix() string { return "view_toggle:" }
func (cb *viewToggleCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {

	field := strings.TrimPrefix(u.Data, "view_toggle:")
	var msg string
	switch field {
	case "hide_past_days":
		chat.HidePastDays = !chat.HidePastDays
		msg = fmt.Sprintf("Скрывать прошедшие дни? Установлено: '%s'", yesNoStr(chat.HidePastDays))
	case "show_parser_time":
		chat.ShowParserTime = !chat.ShowParserTime
		msg = fmt.Sprintf("Отображать в сообщении время последней загрузки расписания? Установлено: '%s'", yesNoStr(chat.ShowParserTime))
	case "show_hints":
		chat.ShowHints = !chat.ShowHints
		msg = fmt.Sprintf("Показывать ли подсказки под расписанием? Установлено: '%s'", yesNoStr(chat.ShowHints))
	}

	cb.bot.chatRepo.Save(chat)
	if msg != "" {
		return cb.bot.SendTextWithKeyboard(u.ChatID, msg, cb.bot.viewKeyboard(chat))
	}
	return cb.bot.showViewSettings(u, chat)
	})
}

func (b *Bot) showCallsFull(u *Update, chat *Chat) {
	b.displayCalls(u, chat, false)
}

func (b *Bot) showCallsFullFull(u *Update, chat *Chat) {
	b.displayCalls(u, chat, true)
}

func (b *Bot) displayCalls(u *Update, chat *Chat, full bool) {
	activeWeekdays := b.cache.GetCallsWeekdays()
	activeSaturday := b.cache.GetCallsSaturday()
	if activeWeekdays == nil {
		activeWeekdays = b.cfg.Timetable.Weekdays
		activeSaturday = b.cfg.Timetable.Saturday
	}

	maxLessons := len(activeWeekdays)
	if len(activeSaturday) > maxLessons {
		maxLessons = len(activeSaturday)
	}

	userMax := maxLessons
	if !full {
		current := countCurrentLessons(chat, b.cache)
		if current > 0 && current < maxLessons {
			userMax = current
		}
	}

	var msg []string
	msg = append(msg, "__ <b>Звонки (будни)</b> __")
	msg = append(msg, b.callsLines(activeWeekdays, userMax))
	msg = append(msg, "\n__ <b>Звонки (суббота)</b> __")
	msg = append(msg, b.callsLines(activeSaturday, userMax))

	if !full && userMax < maxLessons {
		kb := &telego.InlineKeyboardMarkup{
			InlineKeyboard: [][]telego.InlineKeyboardButton{
				{{Text: "Показать полностью", CallbackData: "calls_full"}},
			},
		}
		b.SendTextWithKeyboard(u.ChatID, strings.Join(msg, "\n"), kb)
		return
	}

	b.SendText(u.ChatID, strings.Join(msg, "\n"))
}

func countCurrentLessons(chat *Chat, c *cache.RaspCache) int {
	var data any
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return 0
		}
		data, _ = c.GetGroups()[chat.Group]
	case ModeTeacher:
		if chat.Teacher == "" {
			return 0
		}
		data, _ = c.GetTeachers()[chat.Teacher]
	}
	if data == nil {
		return 0
		}
	if daysRaw, ok := data.(map[string]any); ok {
		if daysArr, ok := daysRaw["days"].([]any); ok && len(daysArr) > 0 {
			if dayMap, ok := daysArr[0].(map[string]any); ok {
				if lessons, ok := dayMap["lessons"].([]any); ok {
					return len(lessons)
				}
			}
		}
	}
	return 0
}

func (b *Bot) callsLines(slots [][2][2]string, maxLessons int) string {
	now := time.Now()
	weekday := now.Weekday()

	var lines []string
	for i := 0; i < maxLessons; i++ {
		if i >= len(slots) {
			break
		}
		slot := slots[i]
		line := fmt.Sprintf("%d. %s - %s | %s - %s", i+1, slot[0][0], slot[0][1], slot[1][0], slot[1][1])

		if isNowInSlot(weekday, slot) {
			line = "👉 " + line + " 👈"
		}

		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func isNowInSlot(weekday time.Weekday, slot [2][2]string) bool {
	isWorkday := weekday >= time.Monday && weekday <= time.Friday
	if !isWorkday {
		return false
	}

	now := time.Now()
	startParts := strings.Split(slot[0][0], ":")
	endParts := strings.Split(slot[1][1], ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		return false
	}

	startH, startM := 0, 0
	fmt.Sscanf(startParts[0], "%d", &startH)
	fmt.Sscanf(startParts[1], "%d", &startM)
	endH, endM := 0, 0
	fmt.Sscanf(endParts[0], "%d", &endH)
	fmt.Sscanf(endParts[1], "%d", &endM)

	startMin := startH*60 + startM
	endMin := endH*60 + endM
	nowMin := now.Hour()*60 + now.Minute()

	return nowMin >= startMin && nowMin <= endMin
}

type diffMenuCb struct{ bot *Bot }

func (cb *diffMenuCb) Prefix() string { return "diff_menu" }
func (cb *diffMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showDiffSettings(u, chat)
	})
}

type diffToggleCb struct{ bot *Bot }

func (cb *diffToggleCb) Prefix() string { return "diff_toggle:" }
func (cb *diffToggleCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		field := strings.TrimPrefix(u.Data, "diff_toggle:")
		var msg string
		switch field {
		case "diff_enabled":
			chat.DiffEnabled = !chat.DiffEnabled
			msg = fmt.Sprintf("Включить раздел \"Что изменилось\"? Установлено: '%s'\nЕсли отключено, кнопки/блоки diff пользователю не показываются.", yesNoStr(chat.DiffEnabled))
		case "diff_auto_in_week":
			chat.DiffAutoInWeek = !chat.DiffAutoInWeek
			msg = fmt.Sprintf("Показывать diff после /week? Установлено: '%s'\nЕсли включено, после недельного расписания бот сразу добавляет блок изменений.", yesNoStr(chat.DiffAutoInWeek))
		case "diff_auto_in_updates":
			chat.DiffAutoInUpdates = !chat.DiffAutoInUpdates
			msg = fmt.Sprintf("Показывать diff в уведомлениях? Установлено: '%s'\nЕсли включено, в автоуведомлениях о сменах будет краткий список изменений.", yesNoStr(chat.DiffAutoInUpdates))
		case "diff_show_before_after":
			chat.DiffShowBeforeAfter = !chat.DiffShowBeforeAfter
			msg = fmt.Sprintf("Показывать старое -> новое для изменённых пар? Установлено: '%s'\nЕсли включено, бот покажет обе версии пары в строках с типом \"~\".", yesNoStr(chat.DiffShowBeforeAfter))
		case "diff_max_lines":
			presets := [4]int{10, 20, 30, 50}
			current := 20
			for i, p := range presets {
				if p == chat.DiffMaxLines {
					current = i
					break
				}
			}
			chat.DiffMaxLines = presets[(current+1)%len(presets)]
			msg = fmt.Sprintf("Лимит строк diff: %d\nКогда изменений больше лимита, бот покажет только первые строки и общий остаток.", chat.DiffMaxLines)
		}

		cb.bot.chatRepo.Save(chat)
		if msg != "" {
			return cb.bot.SendTextWithKeyboard(u.ChatID, msg, cb.bot.diffKeyboard(chat))
		}
		return cb.bot.showDiffSettings(u, chat)
	})
}

type callsMenuCb struct{ bot *Bot }

func (cb *callsMenuCb) Prefix() string { return "calls_menu" }
func (cb *callsMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showCallsSettings(u, chat)
	})
}

type callsShowCb struct{ bot *Bot }

func (cb *callsShowCb) Prefix() string { return "calls_show" }
func (cb *callsShowCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	return cb.bot.sendCallsShow(u)
}

type callsRefreshCb struct{ bot *Bot }

func (cb *callsRefreshCb) Prefix() string { return "calls_refresh" }
func (cb *callsRefreshCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if cb.bot.parseFunc == nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("parse_not_available"))
	}
	go func() {
		if err := cb.bot.parseFunc(); err != nil {
			cb.bot.log.Error().Err(err).Msg("calls refresh parse error")
			cb.bot.SendText(u.ChatID, "⚠️ Ошибка парсера")
			return
		}
		cb.bot.SendText(u.ChatID, "✅ Звонки обновлены с сайта")
	}()
	return u.Bot.SendText(u.ChatID, "🔄 Обновление звонков...")
}

type callsSourceCb struct{ bot *Bot }

func (cb *callsSourceCb) Prefix() string { return "calls_source:" }
func (cb *callsSourceCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	sourceStr := strings.TrimPrefix(u.Data, "calls_source:")
	calls := cb.bot.cache.GetCalls()
	calls.Active.Source = sourceStr

	switch sourceStr {
	case "site":
		calls.Active.Schedule = calls.Site.Schedule
	case "manual":
		calls.Active.Schedule = calls.Manual.Schedule
	case "config":
		calls.Active.Schedule = cache.CallsSchedule{
			Weekdays: cb.bot.cfg.Timetable.Weekdays,
			Saturday: cb.bot.cfg.Timetable.Saturday,
		}
	}
	calls.Active.Hash = sourceStr

	cb.bot.cacheMu.Lock()
	cb.bot.cache.SetCallsFromCache(calls)
	cb.bot.cacheMu.Unlock()

	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showCallsSettings(u, chat)
	})
}

type callsSourceResetCb struct{ bot *Bot }

func (cb *callsSourceResetCb) Prefix() string { return "calls_source_reset" }
func (cb *callsSourceResetCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	calls := cb.bot.cache.GetCalls()
	calls.Active = cache.CallsActive{
		Schedule: calls.Site.Schedule,
		Source:   "site",
		Hash:     calls.Site.Hash,
	}
	cb.bot.cacheMu.Lock()
	cb.bot.cache.SetCallsFromCache(calls)
	cb.bot.cacheMu.Unlock()

	return withChat(cb.bot, u, func(chat *Chat) error {
		return cb.bot.showCallsSettings(u, chat)
	})
}
