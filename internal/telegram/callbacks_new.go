package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
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
