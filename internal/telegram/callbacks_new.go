package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/mymmrac/telego"
)

type diffMenuCb struct{ bot *Bot }

func (cb *diffMenuCb) Prefix() string { return "diff_menu" }
func (cb *diffMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showDiffSettings(u, chat)
}

type diffToggleCb struct{ bot *Bot }

func (cb *diffToggleCb) Prefix() string { return "diff_toggle:" }
func (cb *diffToggleCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	field := strings.TrimPrefix(u.Data, "diff_toggle:")
	switch field {
	case "diff_enabled":
		chat.DiffEnabled = !chat.DiffEnabled
	case "diff_auto_in_week":
		chat.DiffAutoInWeek = !chat.DiffAutoInWeek
	case "diff_auto_in_updates":
		chat.DiffAutoInUpdates = !chat.DiffAutoInUpdates
	case "diff_show_before_after":
		chat.DiffShowBeforeAfter = !chat.DiffShowBeforeAfter
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
	}

	cb.bot.chatRepo.Save(chat)
	return cb.bot.showDiffSettings(u, chat)
}

type callsMenuCb struct{ bot *Bot }

func (cb *callsMenuCb) Prefix() string { return "calls_menu" }
func (cb *callsMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showCallsSettings(u, chat)
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

	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showCallsSettings(u, chat)
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

	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showCallsSettings(u, chat)
}

func (b *Bot) showDiffSettings(u *Update, chat *Chat) error {
	onOff := func(v bool) string {
		if v {
			return "✅"
		}
		return "❌"
	}

	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.DiffEnabled) + " Включить \"Что изменилось\"", CallbackData: "diff_toggle:diff_enabled"}},
			{{Text: onOff(chat.DiffAutoInUpdates) + " Показывать diff в уведомлениях", CallbackData: "diff_toggle:diff_auto_in_updates"}},
			{{Text: onOff(chat.DiffAutoInWeek) + " Показывать diff после /week", CallbackData: "diff_toggle:diff_auto_in_week"}},
			{{Text: onOff(chat.DiffShowBeforeAfter) + " Старое → новое", CallbackData: "diff_toggle:diff_show_before_after"}},
			{{Text: fmt.Sprintf("🧾 Лимит строк: %d", chat.DiffMaxLines), CallbackData: "diff_toggle:diff_max_lines"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Настройки \"Что изменилось\":", kb)
}

func (b *Bot) showCallsSettings(u *Update, chat *Chat) error {
	sourceLabel := func(s string) string {
		switch s {
		case "site":
			return "сайт"
		case "manual":
			return "вручную"
		case "config":
			return "конфиг"
		default:
			return s
		}
	}

	calls := b.cache.GetCalls()
	activeSource := calls.Active.Source

	var lines []string
	lines = append(lines, "🔔 <b>Управление расписанием звонков.</b>")
	lines = append(lines, "")
	if calls.Active.Source != "site" || calls.Active.Hash != calls.Site.Hash {
		lines = append(lines, fmt.Sprintf("Текущий источник: <b>%s</b>", sourceLabel(activeSource)))
	} else {
		lines = append(lines, fmt.Sprintf("Источник: <b>%s</b>", sourceLabel(activeSource)))
	}

	var rows [][]telego.InlineKeyboardButton
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "📊 Показать", CallbackData: "calls_show"},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "✅ Обновить с сайта", CallbackData: "calls_refresh"},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: sourceCheck("Сайт", activeSource == "site"), CallbackData: "calls_source:site"},
		{Text: sourceCheck("Вручную", activeSource == "manual"), CallbackData: "calls_source:manual"},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: sourceCheck("Конфиг", activeSource == "config"), CallbackData: "calls_source:config"},
		{Text: "🔄 Авто", CallbackData: "calls_source_reset"},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "Меню настроек", CallbackData: "settings"},
		{Text: "Главное меню", CallbackData: "main_menu"},
	})

	return u.Bot.SendTextWithKeyboard(u.ChatID, strings.Join(lines, "\n"), &telego.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	})
}

func sourceCheck(label string, active bool) string {
	if active {
		return "✅ " + label
	}
	return label
}

func (b *Bot) sendCallsShow(u *Update) error {
	weekdays := b.cache.GetCallsWeekdays()
	saturday := b.cache.GetCallsSaturday()
	if weekdays == nil {
		weekdays = b.cfg.Timetable.Weekdays
		saturday = b.cfg.Timetable.Saturday
	}

	var msg []string
	msg = append(msg, "__ <b>Звонки (будни)</b> __")
	msg = append(msg, b.callsLines(weekdays, len(weekdays)))

	msg = append(msg, "\n__ <b>Звонки (суббота)</b> __")
	msg = append(msg, b.callsLines(saturday, len(saturday)))

	return b.SendText(u.ChatID, strings.Join(msg, "\n"))
}
