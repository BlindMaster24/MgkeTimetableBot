package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/mymmrac/telego"
)

func (b *Bot) sendSettingsMenu(u *Update, chat *Chat) error {
	chat.Scene = sceneSettings
	b.chatRepo.Save(chat)
	return b.SendTextWithReplyKeyboard(u.ChatID, b.loc("settings_menu"), b.replySettingsMain())
}

func (b *Bot) toggleAndReply(u *Update, chat *Chat, flip func(*Chat), text string, kb *telego.ReplyKeyboardMarkup) error {
	flip(chat)
	b.chatRepo.Save(chat)
	return b.SendTextWithReplyKeyboard(u.ChatID, text, kb)
}

type btnToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *btnToggleTextCmd) Name() string        { return "/btn_toggle_text_" + c.kind }
func (c *btnToggleTextCmd) Description() string { return "" }
func (c *btnToggleTextCmd) Scene() string       { return sceneSettings }

func (c *btnToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "daily":
		return text == `✅ Кнопка "📄 На день"` || text == `🚫 Кнопка "📄 На день"`
	case "weekly":
		return text == `✅ Кнопка "📑 На неделю"` || text == `🚫 Кнопка "📑 На неделю"`
	case "calls":
		return text == `✅ Кнопка "🕐 Звонки"` || text == `🚫 Кнопка "🕐 Звонки"`
	case "about":
		return text == `✅ Кнопка "💡 О боте"` || text == `🚫 Кнопка "💡 О боте"`
	case "fast_group":
		return text == `✅ Кнопка "👩‍🎓 Группа"` || text == `🚫 Кнопка "👩‍🎓 Группа"`
	case "fast_teacher":
		return text == `✅ Кнопка "👩‍🏫 Преподаватель"` || text == `🚫 Кнопка "👩‍🏫 Преподаватель"`
	}
	return false
}

func (c *btnToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "daily":
		chat.ShowDaily = !chat.ShowDaily
		reply = fmt.Sprintf("Показывать кнопку \"📄 На день\"? Установлено: '%s'", yesNoRu(chat.ShowDaily))
	case "weekly":
		chat.ShowWeekly = !chat.ShowWeekly
		reply = fmt.Sprintf("Показывать кнопку \"📑 На неделю\"? Установлено: '%s'", yesNoRu(chat.ShowWeekly))
	case "calls":
		chat.ShowCalls = !chat.ShowCalls
		reply = fmt.Sprintf("Показывать кнопку \"🕐 Звонки\"? Установлено: '%s'", yesNoRu(chat.ShowCalls))
	case "about":
		chat.ShowAbout = !chat.ShowAbout
		reply = fmt.Sprintf("Показывать кнопку \"💡 О боте\"? Установлено: '%s'", yesNoRu(chat.ShowAbout))
	case "fast_group":
		chat.ShowFastGroup = !chat.ShowFastGroup
		reply = fmt.Sprintf("Показывать кнопку \"👩‍🎓 Группа\"? Установлено: '%s'", yesNoRu(chat.ShowFastGroup))
	case "fast_teacher":
		chat.ShowFastTeacher = !chat.ShowFastTeacher
		reply = fmt.Sprintf("Показывать кнопку \"👩‍🏫 Преподаватель\"? Установлено: '%s'", yesNoRu(chat.ShowFastTeacher))
	}
	c.bot.chatRepo.Save(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsButtons(chat))
}

func yesNoRu(v bool) string {
	if v {
		return "да"
	}
	return "нет"
}

type noticeToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *noticeToggleTextCmd) Name() string        { return "/notice_toggle_text_" + c.kind }
func (c *noticeToggleTextCmd) Description() string { return "" }
func (c *noticeToggleTextCmd) Scene() string       { return sceneSettings }

func (c *noticeToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "changes":
		return text == "🔈 Оповещение о новых днях: Да" || text == "🔇 Оповещение о новых днях: Нет"
	case "next_week":
		return text == "🔈 Оповещение о новой неделе: Да" || text == "🔇 Оповещение о новой неделе: Нет"
	case "calls":
		return text == "🔈 Оповещение о звонках: Да" || text == "🔇 Оповещение о звонках: Нет"
	}
	return false
}

func (c *noticeToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "changes":
		chat.NoticeChanges = !chat.NoticeChanges
		reply = fmt.Sprintf("Оповещение о добавлении нового дня: %s", onOffRu(chat.NoticeChanges))
	case "next_week":
		chat.NoticeNextWeek = !chat.NoticeNextWeek
		reply = fmt.Sprintf("Оповещение о добавлении новой недели: %s", onOffRu(chat.NoticeNextWeek))
	case "calls":
		chat.NoticeCalls = !chat.NoticeCalls
		reply = fmt.Sprintf("Оповещение об изменениях расписания звонков: %s", onOffRu(chat.NoticeCalls))
	}
	c.bot.chatRepo.Save(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsNotice(chat))
}

func onOffRu(v bool) string {
	if v {
		return "включено"
	}
	return "выключено"
}

type viewToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *viewToggleTextCmd) Name() string        { return "/view_toggle_text_" + c.kind }
func (c *viewToggleTextCmd) Description() string { return "" }
func (c *viewToggleTextCmd) Scene() string       { return sceneSettings }

func (c *viewToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "hide_past_days":
		return text == "✅ Скрывать прошедшие дни" || text == "🚫 Скрывать прошедшие дни"
	case "show_parser_time":
		return text == "✅ Время последней загрузки расписания" || text == "🚫 Время последней загрузки расписания"
	case "show_hints":
		return text == "✅ Показывать подсказки: Да" || text == "🚫 Показывать подсказки: Нет"
	}
	return false
}

func (c *viewToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "hide_past_days":
		chat.HidePastDays = !chat.HidePastDays
		reply = fmt.Sprintf("Скрывать прошедшие дни? Установлено: '%s'", yesNoRu(chat.HidePastDays))
	case "show_parser_time":
		chat.ShowParserTime = !chat.ShowParserTime
		reply = fmt.Sprintf("Отображать в сообщении время последней загрузки расписания? Установлено: '%s'", yesNoRu(chat.ShowParserTime))
	case "show_hints":
		chat.ShowHints = !chat.ShowHints
		reply = fmt.Sprintf("Показывать ли подсказки под расписанием? Установлено: '%s'", yesNoRu(chat.ShowHints))
	}
	c.bot.chatRepo.Save(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsView(chat))
}

type diffToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *diffToggleTextCmd) Name() string        { return "/diff_toggle_text_" + c.kind }
func (c *diffToggleTextCmd) Description() string { return "" }
func (c *diffToggleTextCmd) Scene() string       { return sceneSettings }

func (c *diffToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "enabled":
		return text == `✅ Включить раздел "Что изменилось"` || text == `🚫 Включить раздел "Что изменилось"`
	case "max_lines":
		return strings.HasPrefix(text, "🧾 Лимит строк: ")
	case "advanced":
		return text == "⚙️ Расширенные"
	case "auto_week":
		return text == "✅ Показывать diff после /week" || text == "🚫 Показывать diff после /week"
	case "auto_updates":
		return text == "✅ Показывать diff в уведомлениях" || text == "🚫 Показывать diff в уведомлениях"
	case "before_after":
		return text == `✅ Показывать "старое -> новое"` || text == `🚫 Показывать "старое -> новое"`
	case "back_basic":
		return text == "⬅️ Базовые настройки"
	}
	return false
}

func (c *diffToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "enabled":
		chat.DiffEnabled = !chat.DiffEnabled
		c.bot.chatRepo.Save(chat)
		reply := fmt.Sprintf("Включить раздел \"Что изменилось\"? Установлено: '%s'\nЕсли отключено, кнопки/блоки diff пользователю не показываются.", yesNoRu(chat.DiffEnabled))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiff(chat))
	case "max_lines":
		presets := []int{10, 20, 30, 50}
		current := chat.DiffMaxLines
		idx := 0
		for i, p := range presets {
			if p == current {
				idx = i
				break
			}
		}
		chat.DiffMaxLines = presets[(idx+1)%len(presets)]
		c.bot.chatRepo.Save(chat)
		reply := fmt.Sprintf("Лимит строк diff: %d\nКогда изменений больше лимита, бот покажет только первые строки и общий остаток.", chat.DiffMaxLines)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiff(chat))
	case "advanced":
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, "Расширенные настройки раздела \"Что изменилось\".", c.bot.replySettingsDiffAdvanced(chat))
	case "auto_week":
		chat.DiffAutoInWeek = !chat.DiffAutoInWeek
		c.bot.chatRepo.Save(chat)
		reply := fmt.Sprintf("Показывать diff после /week? Установлено: '%s'\nЕсли включено, после недельного расписания бот сразу добавляет блок изменений.", yesNoRu(chat.DiffAutoInWeek))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "auto_updates":
		chat.DiffAutoInUpdates = !chat.DiffAutoInUpdates
		c.bot.chatRepo.Save(chat)
		reply := fmt.Sprintf("Показывать diff в уведомлениях? Установлено: '%s'\nЕсли включено, в автоуведомлениях о сменах будет краткий список изменений.", yesNoRu(chat.DiffAutoInUpdates))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "before_after":
		chat.DiffShowBeforeAfter = !chat.DiffShowBeforeAfter
		c.bot.chatRepo.Save(chat)
		reply := fmt.Sprintf("Показывать старое -> новое для изменённых пар? Установлено: '%s'\nЕсли включено, бот покажет обе версии пары в строках с типом \"~\".", yesNoRu(chat.DiffShowBeforeAfter))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "back_basic":
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, "Базовые настройки раздела \"Что изменилось\".", c.bot.replySettingsDiff(chat))
	}
	return nil
}

type formatterSelectTextCmd struct {
	bot *Bot
}

func (c *formatterSelectTextCmd) Name() string        { return "/formatter_select_text" }
func (c *formatterSelectTextCmd) Description() string { return "" }
func (c *formatterSelectTextCmd) Scene() string       { return sceneSettings }

func (c *formatterSelectTextCmd) MatchText(text string) bool {
	for _, f := range formatter.AllFormatters {
		if text == f.Label() || text == f.Label()+" (выбран)" {
			return true
		}
	}
	return false
}

func (c *formatterSelectTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	for i, f := range formatter.AllFormatters {
		if strings.HasPrefix(u.Text, f.Label()) {
			chat.Formatter = i
			c.bot.chatRepo.Save(chat)
			return c.bot.SendTextWithReplyKeyboard(
				u.ChatID,
				fmt.Sprintf("Был успешно выбран \"%s\" форматировщик.", f.Label()),
				c.bot.replySettingsFormatters(chat),
			)
		}
	}
	return nil
}

type settingsNavTextCmd struct {
	bot  *Bot
	kind string
}

func (c *settingsNavTextCmd) Name() string        { return "/settings_nav_" + c.kind }
func (c *settingsNavTextCmd) Description() string { return "" }

func (c *settingsNavTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "to_settings":
		return text == "Меню настроек"
	case "to_main":
		return text == "Главное меню"
	}
	return false
}

func (c *settingsNavTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if c.kind == "to_settings" {
		return c.bot.sendSettingsMenu(u, chat)
	}

	chat.Scene = ""
	c.bot.chatRepo.Save(chat)
	return c.bot.showSchedule(u, chat)
}

type callsManageTextCmd struct {
	bot  *Bot
	menu string
}

func (c *callsManageTextCmd) Name() string        { return "/calls_manage_text" }
func (c *callsManageTextCmd) Description() string { return "" }
func (c *callsManageTextCmd) Scene() string       { return sceneSettingsSchedules }

func (c *callsManageTextCmd) MatchText(text string) bool {
	return text == "🕐 Звонки: управление"
}

func (c *callsManageTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	spec, ok := c.bot.menuByID(c.menu)
	if !ok {
		return nil
	}
	return c.bot.openMenu(u, chat, spec)
}

type showCurrentSettingsTextCmd struct {
	bot *Bot
}

func (c *showCurrentSettingsTextCmd) Name() string        { return "/show_current_settings_text" }
func (c *showCurrentSettingsTextCmd) Description() string { return "" }
func (c *showCurrentSettingsTextCmd) Scene() string       { return sceneSettings }

func (c *showCurrentSettingsTextCmd) MatchText(text string) bool {
	return text == "Показать текущие"
}

func (c *showCurrentSettingsTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, b_currentSettingsText(c.bot, chat), c.bot.replySettingsMain())
}

func b_currentSettingsText(b *Bot, chat *Chat) string {
	return b.currentSettingsText(chat)
}

type aliasActionTextCmd struct {
	bot  *Bot
	kind string
}

func (c *aliasActionTextCmd) Name() string        { return "/alias_action_text_" + c.kind }
func (c *aliasActionTextCmd) Description() string { return "" }
func (c *aliasActionTextCmd) Scene() string       { return sceneSettingsAlias }

func (c *aliasActionTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "list":
		return text == "Список"
	case "add":
		return text == "Добавить"
	case "remove":
		return text == "Удалить"
	case "clear":
		return text == "Отчистить все"
	}
	return false
}

func (c *aliasActionTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "list":
		return c.bot.showAliasList(u, u.UserID)
	case "add":
		chat.Scene = sceneAliasAdd
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendText(u.ChatID, "Введите алиас в формате: оригинальное_название = замена")
	case "remove":
		return c.bot.showAliasRemoveList(u, u.UserID)
	case "clear":
		c.bot.aliasRepo.Clear(u.UserID)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, "Меню настройки алиасов.", c.bot.replySettingsAliases())
	}
	return nil
}

type subsActionTextCmd struct {
	bot  *Bot
	kind string
}

func (c *subsActionTextCmd) Name() string        { return "/subs_action_text_" + c.kind }
func (c *subsActionTextCmd) Description() string { return "" }

func (c *subsActionTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "add_group":
		return text == "➕ Группа"
	case "add_teacher":
		return text == "➕ Преподаватель"
	case "list":
		return text == "📋 Мои подписки"
	case "remove":
		return text == "❌ Удалить подписку"
	case "test":
		return text == "🧪 Проверить"
	}
	return false
}

func (c *subsActionTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "add_group":
		groups := c.bot.cache.GetGroups()
		if len(groups) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		count, _ := c.bot.chatRepo.CountSubscriptions(u.UserID)
		if count >= MAX_SUBSCRIPTIONS {
			return u.Bot.SendText(u.ChatID, fmt.Sprintf("Достигнут лимит подписок (%d).", MAX_SUBSCRIPTIONS))
		}
		chat.Scene = sceneSubAddGroup
		c.bot.chatRepo.Save(chat)
		prompt := fmt.Sprintf("Введите номер группы, на которую хотите подписаться (например, %s)", randomKey(groups))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "add_teacher":
		teachers := c.bot.cache.GetTeachers()
		if len(teachers) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		count, _ := c.bot.chatRepo.CountSubscriptions(u.UserID)
		if count >= MAX_SUBSCRIPTIONS {
			return u.Bot.SendText(u.ChatID, fmt.Sprintf("Достигнут лимит подписок (%d).", MAX_SUBSCRIPTIONS))
		}
		chat.Scene = sceneSubAddTeacher
		c.bot.chatRepo.Save(chat)
		prompt := fmt.Sprintf("Введите фамилию преподавателя (например, %s)", randomKey(teachers))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "list":
		list, _ := c.bot.chatRepo.GetSubscriptions(u.UserID)
		if len(list) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Подписок нет.", c.bot.replySubscriptionsMenu())
		}
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.formatSubscriptionsList(list), c.bot.replySubscriptionsMenu())
	case "remove":
		list, _ := c.bot.chatRepo.GetSubscriptions(u.UserID)
		if len(list) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Подписок нет.", c.bot.replySubscriptionsMenu())
		}
		chat.Scene = sceneSubRemove
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Введите номер подписки для удаления:\n"+c.bot.formatSubscriptionsList(list), c.bot.replySubscriptionsMenu())
	case "test":
		return c.bot.subTestPrompt(u)
	}
	return nil
}

type setupModeTextCmd struct {
	bot  *Bot
	kind string
}

func (c *setupModeTextCmd) Name() string        { return "/setup_mode_text_" + c.kind }
func (c *setupModeTextCmd) Description() string { return "" }
func (c *setupModeTextCmd) Scene() string       { return sceneSetup }

func (c *setupModeTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "guest":
		return text == "👀 Гость" || text == "Гость"
	case "student":
		return text == "👩‍🎓 Учащийся" || text == "👩‍🎓 Ученик" || text == "Учащийся" || text == "Ученик"
	case "parent":
		return text == "👨‍👩‍👦 Родитель" || text == "Родитель"
	case "skip":
		return text == "🔙 Пропустить" || text == "Пропустить"
	}
	return false
}

func (c *setupModeTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "guest":
		chat.Mode = ModeGuest
		chat.Scene = ""
		c.bot.chatRepo.Save(chat)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("about_bot"), replyMainMenu(c.bot, chat))
	case "student", "parent":
		groups := c.bot.cache.GetGroups()
		if len(groups) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		if c.kind == "student" {
			chat.Mode = ModeStudent
		} else {
			chat.Mode = ModeParent
		}
		chat.Scene = sceneSetGroup
		c.bot.chatRepo.Save(chat)
		prompt := fmt.Sprintf("%s (например, %s)", c.bot.loc("setup_enter_group"), randomKey(groups))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "skip":
		chat.Scene = ""
		c.bot.chatRepo.Save(chat)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("about_bot"), replyMainMenu(c.bot, chat))
	}
	return nil
}

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
