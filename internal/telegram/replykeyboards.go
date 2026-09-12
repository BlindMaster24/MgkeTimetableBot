package telegram

import (
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/mymmrac/telego"
)

type SceneMatcher interface {
	Scene() string
}

func noYesSmile(v bool, text string) string {
	if v {
		return "✅ " + text
	}
	return "🚫 " + text
}

func sourceCheck(label string, active bool) string {
	if active {
		return "✅ " + label
	}
	return label
}

func noYesSmileVolume(v bool, text string) string {
	if v {
		return "🔈 " + text + ": Да"
	}
	return "🔇 " + text + ": Нет"
}

func settingsBotNavRow() []telego.KeyboardButton {
	return []telego.KeyboardButton{
		{Text: "Меню настроек"},
		{Text: "Главное меню"},
	}
}

func (b *Bot) replySettingsMain() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "📚 Первоначальная настройка"}},
			{{Text: "🗓️ Управление расписаниями"}},
			{{Text: "⌨️ Кнопки"}, {Text: "📃 Форматировщик"}},
			{{Text: "🔊 Оповещения"}, {Text: "🔔 Подписки"}, {Text: "🖼️ Отображение"}, {Text: "📊 Сравнение"}},
			{{Text: "Показать текущие"}, {Text: "Главное меню"}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsButtons(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{
				{Text: noYesSmile(chat.ShowDaily, `Кнопка "📄 На день"`)},
				{Text: noYesSmile(chat.ShowWeekly, `Кнопка "📑 На неделю"`)},
			},
			{
				{Text: noYesSmile(chat.ShowCalls, `Кнопка "🕐 Звонки"`)},
				{Text: noYesSmile(chat.ShowAbout, `Кнопка "💡 О боте"`)},
			},
			{
				{Text: noYesSmile(chat.ShowFastGroup, `Кнопка "👩‍🎓 Группа"`)},
				{Text: noYesSmile(chat.ShowFastTeacher, `Кнопка "👩‍🏫 Преподаватель"`)},
			},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsNotice(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmileVolume(chat.NoticeChanges, "Оповещение о новых днях")}},
			{{Text: noYesSmileVolume(chat.NoticeNextWeek, "Оповещение о новой неделе")}},
			{{Text: noYesSmileVolume(chat.NoticeCalls, "Оповещение о звонках")}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsView(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.HidePastDays, "Скрывать прошедшие дни")}},
			{{Text: noYesSmile(chat.ShowParserTime, "Время последней загрузки расписания")}},
			{{Text: noYesSmileVolume(chat.ShowHints, "Показывать подсказки")}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsDiff(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.DiffEnabled, `Включить раздел "Что изменилось"`)}},
			{{Text: fmt.Sprintf("🧾 Лимит строк: %d", chat.DiffMaxLines)}},
			{{Text: "⚙️ Расширенные"}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsDiffAdvanced(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.DiffAutoInWeek, "Показывать diff после /week")}},
			{{Text: noYesSmile(chat.DiffAutoInUpdates, "Показывать diff в уведомлениях")}},
			{{Text: noYesSmile(chat.DiffShowBeforeAfter, `Показывать "старое -> новое"`)}},
			{{Text: "⬅️ Базовые настройки"}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsFormatters(chat *Chat) *telego.ReplyKeyboardMarkup {
	var rows [][]telego.KeyboardButton
	var currentRow []telego.KeyboardButton
	for i, f := range formatter.AllFormatters {
		label := f.Label()
		if chat.Formatter == i {
			label += " (выбран)"
		}
		currentRow = append(currentRow, telego.KeyboardButton{Text: label})
		if (i+1)%2 == 0 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}
	rows = append(rows, settingsBotNavRow())
	return &telego.ReplyKeyboardMarkup{
		Keyboard:       rows,
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsSchedules() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "🕐 Звонки: управление"}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyCallsSettings(chat *Chat, admin bool) *telego.ReplyKeyboardMarkup {
	var rows [][]telego.KeyboardButton
	rows = append(rows, []telego.KeyboardButton{{Text: "📊 Показать"}})

	if admin {
		calls := b.cache.GetCalls()
		check := func(v bool) string {
			if v {
				return "✅ "
			}
			return ""
		}
		activeSource := calls.Active.Source
		if calls.OverrideSource != "" {
			activeSource = calls.OverrideSource
		}
		autoActive := calls.OverrideSource == ""

		rows = append(rows, []telego.KeyboardButton{{Text: "✅ Обновить с сайта"}})
		rows = append(rows, []telego.KeyboardButton{{Text: "✏️ Изменить вручную"}})
		rows = append(rows, []telego.KeyboardButton{
			{Text: check(activeSource == "site") + "Источник: сайт"},
			{Text: check(activeSource == "manual") + "Источник: вручную"},
		})
		rows = append(rows, []telego.KeyboardButton{
			{Text: check(activeSource == "config") + "Источник: конфиг"},
			{Text: check(autoActive) + "Источник: авто"},
		})
	}

	rows = append(rows, settingsBotNavRow())
	return &telego.ReplyKeyboardMarkup{
		Keyboard:       rows,
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsAliases() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "Список"}, {Text: "Добавить"}},
			{{Text: "Удалить"}},
			{{Text: "Отчистить все"}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySubscriptionsMenu() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "➕ Группа"}, {Text: "➕ Преподаватель"}},
			{{Text: "📋 Мои подписки"}, {Text: "❌ Удалить подписку"}, {Text: "🧪 Проверить"}},
			settingsBotNavRow(),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySelectMode() *telego.ReplyKeyboardMarkup {
	t := func(key string) string { return b.loc(key) }
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: t("mode_guest")}},
			{{Text: t("mode_student")}, {Text: t("mode_teacher")}},
			{{Text: t("mode_parent")}, {Text: "🔙 Пропустить"}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyCancel() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "Отмена"}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyStartButton() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: "Начать"}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}
