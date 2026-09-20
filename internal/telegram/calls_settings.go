package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

type callsSettingsTextCmd struct {
	bot  *Bot
	kind string
}

func (c *callsSettingsTextCmd) Name() string        { return "/calls_settings_text_" + c.kind }
func (c *callsSettingsTextCmd) Description() string { return "" }
func (c *callsSettingsTextCmd) Scene() string       { return sceneSettingsCalls }

func (c *callsSettingsTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "show":
		return text == "📊 Показать"
	case "refresh":
		return text == "✅ Обновить с сайта"
	case "edit":
		return text == "✏️ Изменить вручную"
	case "source_site":
		return text == "Источник: сайт" || text == "✅ Источник: сайт"
	case "source_manual":
		return text == "Источник: вручную" || text == "✅ Источник: вручную"
	case "source_config":
		return text == "Источник: конфиг" || text == "✅ Источник: конфиг"
	case "source_auto":
		return text == "Источник: авто" || text == "✅ Источник: авто"
	}
	return false
}

func (c *callsSettingsTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if c.kind == "show" {
		return c.bot.sendCallsShow(u)
	}

	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	switch c.kind {
	case "refresh":
		return c.bot.refreshCallsNow(u, chat)
	case "edit":
		chat.Scene = sceneCallsEditInput
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Введите расписание звонков. Пример\nБудни\n1 08:30 09:15 09:25 10:10\n2 10:20 11:05 11:15 12:00\nСуббота\n1 09:00 09:45 09:55 10:40", c.bot.replyCancel())
	case "source_site", "source_manual", "source_config":
		source := strings.TrimPrefix(c.kind, "source_")
		c.bot.applyCallsSource(source)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Источник звонков переключен на %s.\n\n%s", callsSourceLabel(source), c.bot.callsMenuText(chat, true)), c.bot.replyCallsSettings(chat, true))
	case "source_auto":
		c.bot.cache.ResetCallsOverride()
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Источник звонков переключен на авто.\n\n%s", c.bot.callsMenuText(chat, true)), c.bot.replyCallsSettings(chat, true))
	}
	return nil
}

func (b *Bot) applyCallsSource(source string) {
	b.cache.SetCallsOverride(source)
	if source != "config" {
		return
	}
	calls := b.cache.GetCalls()
	calls.Active.Schedule = cache.CallsSchedule{
		Weekdays: b.cfg.Timetable.Weekdays,
		Saturday: b.cfg.Timetable.Saturday,
	}
	b.cache.SetCallsFromCache(calls)
}

func (b *Bot) refreshCallsNow(u *Update, chat *Chat) error {
	lines := []string{"🔄 Обновление звонков"}

	var err error
	if b.parseFunc != nil {
		err = b.parseFunc()
	}

	calls := b.cache.GetCalls()
	siteParsed := len(calls.Site.Schedule.Weekdays) > 0

	switch {
	case err != nil:
		lines = append(lines, "⚠️ Ошибка парсера")
	case !siteParsed:
		lines = append(lines, "⚠️ Сайт отдал пусто")
	default:
		lines = append(lines, "✅ Сайт успешно спарсен")
	}

	if siteParsed {
		lines = append(lines, "🧪 Результат парсинга: OK")
	} else {
		lines = append(lines, "🧪 Результат парсинга: EMPTY")
	}

	if calls.Site.UpdatedAt > 0 {
		lines = append(lines, fmt.Sprintf("📅 Дата на сайте: %s", time.UnixMilli(calls.Site.UpdatedAt).Format("02.01.2006 15:04")))
	}

	active := calls.Active.Source
	if calls.OverrideSource != "" {
		active = calls.OverrideSource
	}
	lines = append(lines, fmt.Sprintf("📌 Активный источник: %s", callsSourceLabel(active)))

	if calls.ManualReason != "" && calls.Active.Source == "manual" {
		lines = append(lines, fmt.Sprintf("✍️ Причина: %s", calls.ManualReason))
	}

	if err != nil {
		lines = append(lines, "🪠 "+err.Error())
	}

	schedule := b.callsScheduleFor(chat)
	if len(schedule.Weekdays) > 0 {
		lines = append(lines, "\n__ Звонки (будни) __")
		lines = append(lines, formatCallsPlain(schedule.Weekdays))
		lines = append(lines, "\n__ Звонки (суббота) __")
		lines = append(lines, formatCallsPlain(schedule.Saturday))
	}

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, strings.Join(lines, "\n"), b.replyCallsSettings(chat, true))
}

func formatCallsPlain(slots [][2][2]string) string {
	var lines []string
	for i, slot := range slots {
		lines = append(lines, fmt.Sprintf("%d. %s - %s | %s - %s", i+1, slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
	}
	return strings.Join(lines, "\n")
}

func callsSourceLabel(source string) string {
	switch source {
	case "site":
		return "сайт"
	case "manual":
		return "вручную"
	default:
		return "конфиг"
	}
}

func (b *Bot) callsMenuText(chat *Chat, admin bool) string {
	calls := b.cache.GetCalls()

	lines := []string{"Управление расписанием звонков."}
	lines = append(lines, b.callsCampusMenuLines(chat)...)
	if admin {
		if calls.OverrideSource != "" {
			lines = append(lines, fmt.Sprintf("Переопределение источника: %s", callsSourceLabel(calls.OverrideSource)))
		} else {
			lines = append(lines, fmt.Sprintf("Источник: авто (сейчас: %s)", callsSourceLabel(calls.Active.Source)))
		}
	}

	if calls.OverrideSource == "" && calls.Active.Source != "site" && calls.SiteEmptyNotifiedAt > 0 {
		lines = append(lines, "Сайт отдал пусто.")
	}

	switch {
	case calls.Active.Source == "site" && calls.Site.UpdatedAtRaw != "":
		lines = append(lines, fmt.Sprintf("Обновлено на сайте: %s", calls.Site.UpdatedAtRaw))
	case calls.Active.Source == "site" && calls.Site.UpdatedAt > 0:
		lines = append(lines, fmt.Sprintf("Обновлено на сайте: %s", time.UnixMilli(calls.Site.UpdatedAt).Format("02.01.2006 15:04")))
	case calls.Active.Source == "manual" && calls.ManualReason != "":
		lines = append(lines, fmt.Sprintf("Причина: %s", calls.ManualReason))
	case calls.Active.Source == "config" && len(b.cfg.Timetable.Weekdays) > 0:
		lines = append(lines, "Используются данные из конфига")
	}

	return strings.Join(lines, "\n")
}

func (b *Bot) showCallsSettingsReply(u *Update, chat *Chat) error {
	admin := b.isAdmin(u.UserID)
	return b.SendTextWithReplyKeyboard(u.ChatID, b.callsMenuText(chat, admin), b.replyCallsSettings(chat, admin))
}
