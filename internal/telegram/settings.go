package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/mymmrac/telego"
)

func (b *Bot) settingsKeyboardFull(chat *Chat) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: b.loc("button_setup"), CallbackData: "setup"}},
			{{Text: "🗓️ Управление расписаниями", CallbackData: "schedules_menu"}},
			{{Text: "⌨️ Кнопки", CallbackData: "btn_menu"}, {Text: "📃 Форматировщик", CallbackData: "fmt_menu"}},
			{{Text: "🔊 Оповещения", CallbackData: "notice_menu"}, {Text: "🔔 Подписки", CallbackData: "subs_menu"}, {Text: "🖼️ Отображение", CallbackData: "view_menu"}, {Text: "📊 Сравнение", CallbackData: "diff_menu"}},
			{{Text: "Показать текущие", CallbackData: "current_settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) buttonsKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: noYesSmile(chat.ShowDaily, "Кнопка \"📄 На день\""), CallbackData: "btn_toggle:show_daily"},
				{Text: noYesSmile(chat.ShowWeekly, "Кнопка \"📑 На неделю\""), CallbackData: "btn_toggle:show_weekly"},
			},
			{
				{Text: noYesSmile(chat.ShowCalls, "Кнопка \"🕐 Звонки\""), CallbackData: "btn_toggle:show_calls"},
				{Text: noYesSmile(chat.ShowAbout, "Кнопка \"💡 О боте\""), CallbackData: "btn_toggle:show_about"},
			},
			{
				{Text: noYesSmile(chat.ShowFastGroup, "Кнопка \"👩‍🎓 Группа\""), CallbackData: "btn_toggle:show_fast_group"},
				{Text: noYesSmile(chat.ShowFastTeacher, "Кнопка \"👩‍🏫 Преподаватель\""), CallbackData: "btn_toggle:show_fast_teacher"},
			},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func noYesSmile(v bool, label string) string {
	if v {
		return "✅ " + label
	}
	return "🚫 " + label
}

func (b *Bot) formatterKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton
	var currentRow []telego.InlineKeyboardButton
	for i, f := range formatter.AllFormatters {
		label := f.Label()
		if chat.Formatter == i {
			label += " (выбран)"
		}
		cbData := fmt.Sprintf("fmt_select:%d", i)
		currentRow = append(currentRow, telego.InlineKeyboardButton{Text: label, CallbackData: cbData})
		if (i+1)%2 == 0 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "Меню настроек", CallbackData: "settings"},
		{Text: "Главное меню", CallbackData: "main_menu"},
	})
	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) showNoticeSettings(u *Update, chat *Chat) error {
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настройки оповещений.", b.noticeKeyboard(chat))
}

func (b *Bot) showViewSettings(u *Update, chat *Chat) error {
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настройки отображения.", b.viewKeyboard(chat))
}

func (b *Bot) showDiffSettings(u *Update, chat *Chat) error {
	onOff := func(v bool) string {
		if v {
			return "✅"
		}
		return "🚫"
	}
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.DiffEnabled) + " Включить раздел \"Что изменилось\"", CallbackData: "diff_toggle:diff_enabled"}},
			{{Text: fmt.Sprintf("🧾 Лимит строк: %d", chat.DiffMaxLines), CallbackData: "diff_toggle:diff_max_lines"}},
			{{Text: "⚙️ Расширенные", CallbackData: "diff_advanced"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настроек раздела \"Что изменилось\".", kb)
}

func (b *Bot) showDiffAdvancedSettings(u *Update, chat *Chat) error {
	onOff := func(v bool) string {
		if v {
			return "✅"
		}
		return "🚫"
	}
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.DiffAutoInWeek) + " Показывать diff после /week", CallbackData: "diff_toggle:diff_auto_in_week"}},
			{{Text: onOff(chat.DiffAutoInUpdates) + " Показывать diff в уведомлениях", CallbackData: "diff_toggle:diff_auto_in_updates"}},
			{{Text: onOff(chat.DiffShowBeforeAfter) + " Показывать \"старое -> новое\"", CallbackData: "diff_toggle:diff_show_before_after"}},
			{{Text: "⬅️ Базовые настройки", CallbackData: "diff_menu"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Расширенные настройки раздела \"Что изменилось\".", kb)
}

func (b *Bot) showSchedulesSettings(u *Update, chat *Chat) error {
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "🕐 Звонки: управление", CallbackData: "calls_menu"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Управление расписаниями:", kb)
}

func (b *Bot) showCurrentSettings(u *Update, chat *Chat) error {
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, b.currentSettingsText(chat), kb)
}


type schedulesMenuCb struct{ bot *Bot }

func (cb *schedulesMenuCb) Prefix() string { return "schedules_menu" }
func (cb *schedulesMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showSchedulesSettings(u, chat)
}

type currentSettingsCb struct{ bot *Bot }

func (cb *currentSettingsCb) Prefix() string { return "current_settings" }
func (cb *currentSettingsCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showCurrentSettings(u, chat)
}

type diffAdvancedCb struct{ bot *Bot }

func (cb *diffAdvancedCb) Prefix() string { return "diff_advanced" }
func (cb *diffAdvancedCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.showDiffAdvancedSettings(u, chat)
}

type subsMenuCb struct{ bot *Bot }

func (cb *subsMenuCb) Prefix() string { return "subs_menu" }
func (cb *subsMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Подписки позволяют получать уведомления об изменениях расписания другой группы или преподавателя.", cb.bot.subscriptionsKeyboard())
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
		{Text: "✎ Изменить вручную", CallbackData: "calls_edit"},
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
	chat, _ := b.chatRepo.FindOrCreate("telegram", u.UserID)
	b.displayCalls(u, chat, true)
	return nil
}

func (b *Bot) noticeKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	smile := func(v bool) string {
		if v { return "🔈" }
		return "🔇"
	}
	yesNo := func(v bool) string {
		if v { return "Да" }
		return "Нет"
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: smile(chat.NoticeChanges) + " Оповещение о новых днях: " + yesNo(chat.NoticeChanges), CallbackData: "notice_toggle:notice_changes"}},
			{{Text: smile(chat.NoticeNextWeek) + " Оповещение о новой неделе: " + yesNo(chat.NoticeNextWeek), CallbackData: "notice_toggle:notice_next_week"}},
			{{Text: smile(chat.NoticeCalls) + " Оповещение о звонках: " + yesNo(chat.NoticeCalls), CallbackData: "notice_toggle:notice_calls"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) viewKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	onOff := func(v bool) string {
		if v { return "✅" }
		return "🚫"
	}
	yesNo := func(v bool) string {
		if v { return "Да" }
		return "Нет"
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.HidePastDays) + " Скрывать прошедшие дни", CallbackData: "view_toggle:hide_past_days"}},
			{{Text: onOff(chat.ShowParserTime) + " Время последней загрузки расписания", CallbackData: "view_toggle:show_parser_time"}},
			{{Text: onOff(chat.ShowHints) + " Показывать подсказки: " + yesNo(chat.ShowHints), CallbackData: "view_toggle:show_hints"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) diffKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	onOff := func(v bool) string {
		if v { return "✅" }
		return "🚫"
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.DiffEnabled) + " Включить раздел \"Что изменилось\"", CallbackData: "diff_toggle:diff_enabled"}},
			{{Text: fmt.Sprintf("🧾 Лимит строк: %d", chat.DiffMaxLines), CallbackData: "diff_toggle:diff_max_lines"}},
			{{Text: "⚙️ Расширенные", CallbackData: "diff_advanced"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) diffAdvancedKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	onOff := func(v bool) string {
		if v { return "✅" }
		return "🚫"
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: onOff(chat.DiffAutoInWeek) + " Показывать diff после /week", CallbackData: "diff_toggle:diff_auto_in_week"}},
			{{Text: onOff(chat.DiffAutoInUpdates) + " Показывать diff в уведомлениях", CallbackData: "diff_toggle:diff_auto_in_updates"}},
			{{Text: onOff(chat.DiffShowBeforeAfter) + " Показывать \"старое -> новое\"", CallbackData: "diff_toggle:diff_show_before_after"}},
			{{Text: "⬅️ Базовые настройки", CallbackData: "diff_menu"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) schedulesKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "🕐 Звонки: управление", CallbackData: "calls_menu"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
}

func (b *Bot) currentSettingsText(chat *Chat) string {
	yesNo := func(v bool) string {
		if v { return "да" }
		return "нет"
	}
	lines := []string{
		fmt.Sprintf(`Показывать кнопку расписания "📄 На день": %s`, yesNo(chat.ShowDaily)),
		fmt.Sprintf(`Показывать кнопку расписания "📑 На неделю": %s`, yesNo(chat.ShowWeekly)),
		fmt.Sprintf(`Показывать кнопку "🕐 Звонки": %s`, yesNo(chat.ShowCalls)),
		fmt.Sprintf(`Показывать кнопку "💡 О боте": %s`, yesNo(chat.ShowAbout)),
		fmt.Sprintf(`Показывать кнопку "👩‍🎓 Группа": %s`, yesNo(chat.ShowFastGroup)),
		fmt.Sprintf(`Показывать кнопку "👩‍🏫 Преподаватель": %s`, yesNo(chat.ShowFastTeacher)),
		"",
		fmt.Sprintf("Скрывать прошедшие дни в расписании на неделю: %s", yesNo(chat.HidePastDays)),
		fmt.Sprintf("Отображать в сообщении время последней загрузки расписания: %s", yesNo(chat.ShowParserTime)),
		fmt.Sprintf("Подсказки под расписанием: %s", yesNo(chat.ShowHints)),
		fmt.Sprintf(`Раздел "Что изменилось": %s`, yesNo(chat.DiffEnabled)),
		fmt.Sprintf("Diff после /week: %s", yesNo(chat.DiffAutoInWeek)),
		fmt.Sprintf("Diff в уведомлениях: %s", yesNo(chat.DiffAutoInUpdates)),
		fmt.Sprintf(`Показывать старое -> новое: %s`, yesNo(chat.DiffShowBeforeAfter)),
		fmt.Sprintf("Лимит строк diff: %d", chat.DiffMaxLines),
		"",
		fmt.Sprintf("Оповещение о добавлении нового дня: %s", yesNo(chat.NoticeChanges)),
		fmt.Sprintf("Оповещение о добавлении новой недели: %s", yesNo(chat.NoticeNextWeek)),
		fmt.Sprintf("Оповещение об изменении звонков: %s", yesNo(chat.NoticeCalls)),
		fmt.Sprintf("Оповещение об ошибке парсера: %s", yesNo(chat.NoticeParserErrors)),
		"",
		"~~~ Системные (отладочная информация) ~~~",
		fmt.Sprintf("Режим чата: %s", string(chat.Mode)),
		fmt.Sprintf("Выбранная группа: %s", chat.Group),
		fmt.Sprintf("Выбранный учитель: %s", chat.Teacher),
		fmt.Sprintf("ID последнего сообщения: %d", chat.LastMsgID),
		fmt.Sprintf("Разрешено ли отправлять боту сообщения: %s", yesNo(chat.AllowSendMess)),
	}
	return strings.Join(lines, "\n")
}
