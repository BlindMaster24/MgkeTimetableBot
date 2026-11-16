package telegram

import (
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/mymmrac/telego"
)

type SceneMatcher interface {
	Scene() string
}

func settingsBotNavRow(b *Bot) []telego.KeyboardButton {
	return []telego.KeyboardButton{
		{Text: b.loc("kbtext_1")},
		{Text: b.loc("kbtext_2")},
	}
}

func (b *Bot) replySettingsMain() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_3")}},
			{{Text: b.loc("kbtext_4")}},
			{{Text: b.loc("kbtext_5")}, {Text: b.loc("kbtext_6")}},
			{{Text: b.loc("kbtext_7")}, {Text: b.loc("kbtext_8")}, {Text: b.loc("kbtext_9")}, {Text: b.loc("kbtext_10")}},
			{{Text: b.loc("button_settings_reminders")}},
			{{Text: b.loc("kbtext_11")}, {Text: b.loc("kbtext_12")}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsButtons(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{
				{Text: noYesSmile(chat.ShowDaily, b.loc("kbtext_13"))},
				{Text: noYesSmile(chat.ShowWeekly, b.loc("kbtext_14"))},
			},
			{
				{Text: noYesSmile(chat.ShowCalls, b.loc("kbtext_15"))},
				{Text: noYesSmile(chat.ShowAbout, b.loc("kbtext_16"))},
			},
			{
				{Text: noYesSmile(chat.ShowFastGroup, b.loc("kbtext_17"))},
				{Text: noYesSmile(chat.ShowFastTeacher, b.loc("kbtext_18"))},
			},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsNotice(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmileVolume(chat.NoticeChanges, b.loc("kbtext_19"))}},
			{{Text: noYesSmileVolume(chat.NoticeNextWeek, b.loc("kbtext_20"))}},
			{{Text: noYesSmileVolume(chat.NoticeCalls, b.loc("kbtext_21"))}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsReminders(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("reminder_lead_default")}},
			{{Text: b.loc("reminder_lead_5")}, {Text: b.loc("reminder_lead_10")}, {Text: b.loc("reminder_lead_15")}},
			{{Text: b.loc("reminder_lead_30")}, {Text: b.loc("reminder_lead_60")}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsView(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.HidePastDays, b.loc("kbtext_22"))}},
			{{Text: noYesSmile(chat.ShowParserTime, b.loc("kbtext_23"))}},
			{{Text: noYesSmile(chat.ShowHints, b.loc("kbtext_24")) + yesNoCapital(chat.ShowHints)}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsDiff(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.DiffEnabled, b.loc("kbtext_25"))}},
			{{Text: fmt.Sprintf(b.loc("kbtext_26"), chat.DiffMaxLines)}},
			{{Text: b.loc("kbtext_27")}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsDiffAdvanced(chat *Chat) *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: noYesSmile(chat.DiffAutoInWeek, b.loc("kbtext_28"))}},
			{{Text: noYesSmile(chat.DiffAutoInUpdates, b.loc("kbtext_29"))}},
			{{Text: noYesSmile(chat.DiffShowBeforeAfter, b.loc("kbtext_30"))}},
			{{Text: b.loc("kbtext_31")}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsFormatters(chat *Chat) *telego.ReplyKeyboardMarkup {
	var rows [][]telego.KeyboardButton
	var currentRow []telego.KeyboardButton
	for i, f := range formatter.AllFormatters {
		label := f.Label(formatter.FormatOptions{Labels: b.formatterLabels()})
		if chat.Formatter == i {
			label += b.loc("kbtext_32")
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
	rows = append(rows, settingsBotNavRow(b))
	return &telego.ReplyKeyboardMarkup{
		Keyboard:       rows,
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsSchedules() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_33")}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyCallsSettings(chat *Chat, admin bool) *telego.ReplyKeyboardMarkup {
	var rows [][]telego.KeyboardButton
	rows = append(rows, []telego.KeyboardButton{{Text: b.loc("kbtext_34")}})

	if names := b.campusNames(); len(names) > 1 {
		selected := b.selectedCampus(chat)
		var row []telego.KeyboardButton
		for _, name := range names {
			row = append(row, telego.KeyboardButton{Text: campusButtonText(name, selected == name)})
			if len(row) == 2 {
				rows = append(rows, row)
				row = nil
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
		if selected != "" {
			rows = append(rows, []telego.KeyboardButton{{Text: campusButtonPrefix + campusAutoLabel}})
		}
	}

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

		rows = append(rows, []telego.KeyboardButton{{Text: b.loc("kbtext_35")}})
		rows = append(rows, []telego.KeyboardButton{{Text: b.loc("kbtext_36")}})
		rows = append(rows, []telego.KeyboardButton{
			{Text: check(activeSource == "site") + b.loc("kbtext_37")},
			{Text: check(activeSource == "manual") + b.loc("kbtext_38")},
		})
		rows = append(rows, []telego.KeyboardButton{
			{Text: check(activeSource == "config") + b.loc("kbtext_39")},
			{Text: check(autoActive) + b.loc("kbtext_40")},
		})
	}

	rows = append(rows, settingsBotNavRow(b))
	return &telego.ReplyKeyboardMarkup{
		Keyboard:       rows,
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySettingsAliases() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_41")}, {Text: b.loc("kbtext_42")}},
			{{Text: b.loc("kbtext_43")}},
			{{Text: b.loc("kbtext_44")}},
			settingsBotNavRow(b),
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replySubscriptionsMenu() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_45")}, {Text: b.loc("kbtext_46")}},
			{{Text: b.loc("kbtext_47")}, {Text: b.loc("kbtext_48")}, {Text: b.loc("kbtext_49")}},
			settingsBotNavRow(b),
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
			{{Text: t("mode_parent")}, {Text: b.loc("kbtext_50")}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyCancel() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_51")}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}

func (b *Bot) replyStartButton() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: b.loc("kbtext_52")}},
		},
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}
