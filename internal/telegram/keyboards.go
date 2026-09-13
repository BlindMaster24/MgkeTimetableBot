package telegram

import (
	"fmt"

	"github.com/mymmrac/telego"
)

func withCancelButton(kb *telego.InlineKeyboardMarkup) *telego.InlineKeyboardMarkup {
	if kb == nil {
		kb = &telego.InlineKeyboardMarkup{}
	}
	kb.InlineKeyboard = append(kb.InlineKeyboard, []telego.InlineKeyboardButton{
		{Text: "Отмена", CallbackData: "cancel"},
	})
	return kb
}

func groupHistoryKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	kb := &telego.InlineKeyboardMarkup{}
	for _, g := range chat.HistoryGroup {
		kb.InlineKeyboard = append(kb.InlineKeyboard, []telego.InlineKeyboardButton{
			{Text: g, CallbackData: "answer:" + g},
		})
	}
	return kb
}

func teacherHistoryKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	kb := &telego.InlineKeyboardMarkup{}
	for _, t := range chat.HistoryTeacher {
		kb.InlineKeyboard = append(kb.InlineKeyboard, []telego.InlineKeyboardButton{
			{Text: t, CallbackData: "answer:" + t},
		})
	}
	return kb
}

func verticalValuesKeyboard(values []string) *telego.InlineKeyboardMarkup {
	kb := &telego.InlineKeyboardMarkup{}
	for _, v := range values {
		kb.InlineKeyboard = append(kb.InlineKeyboard, []telego.InlineKeyboardButton{
			{Text: v, CallbackData: "answer:" + v},
		})
	}
	return kb
}

func getWeekTimetableKeyboard(typeName, value string) *telego.InlineKeyboardMarkup {
	return weekTimetableButton("На неделю", typeName, value, 0, true)
}

func weekTimetableButton(label, typeName, value string, weekIndex int, showHeader bool) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: label, CallbackData: fmt.Sprintf("timetable_%s:%s:%d:0:%s", string(typeName[0]), value, weekIndex, boolToInt(showHeader))}},
		},
	}
}

func replyMainMenu(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup {
	t := func(key string) string { return b.loc(key) }
	canShow := (chat.Mode == ModeStudent || chat.Mode == ModeParent || chat.Mode == ModeTeacher) &&
		((chat.Mode == ModeStudent || chat.Mode == ModeParent) && chat.Group != "" ||
			chat.Mode == ModeTeacher && chat.Teacher != "")

	var rows [][]telego.KeyboardButton

	if chat.Mode == "" {
		rows = append(rows, []telego.KeyboardButton{
			{Text: t("button_setup")},
		})
	} else if chat.Mode == ModeGuest {
		rows = append(rows, []telego.KeyboardButton{
			{Text: t("button_group")},
			{Text: t("button_teacher")},
		})
	} else {
		if canShow {
			var row []telego.KeyboardButton
			if chat.ShowDaily {
				row = append(row, telego.KeyboardButton{Text: t("button_day")})
			}
			if chat.ShowWeekly {
				row = append(row, telego.KeyboardButton{Text: t("button_week")})
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}

	showFast := chat.Mode == ModeStudent || chat.Mode == ModeParent || chat.Mode == ModeTeacher
	canShowCalls := chat.ShowCalls && (b.cfg.Parser.Calls == nil || b.cfg.Parser.Calls.Enabled)

	var level2 []telego.KeyboardButton
	if showFast && chat.ShowFastGroup {
		level2 = append(level2, telego.KeyboardButton{Text: t("button_group")})
	}
	if chat.ShowAbout && canShowCalls {
		level2 = append(level2, telego.KeyboardButton{Text: t("button_calls")})
	}
	if showFast && chat.ShowFastTeacher {
		label := t("button_teacher")
		if chat.ShowAbout && chat.ShowCalls && showFast && chat.ShowFastGroup {
			label = t("button_teacher_short")
		}
		level2 = append(level2, telego.KeyboardButton{Text: label})
	}
	if len(level2) > 0 {
		rows = append(rows, level2)
	}

	var level3 []telego.KeyboardButton
	if !chat.ShowAbout && canShowCalls {
		level3 = append(level3, telego.KeyboardButton{Text: t("button_calls")})
	}
	if b.cfg.Google.OAuth.ClientID != "" {
		level3 = append(level3, telego.KeyboardButton{Text: t("button_google_calendar")})
	}
	if b.cfg.Calendar.ICS.Enabled {
		level3 = append(level3, telego.KeyboardButton{Text: t("button_ics")})
	}
	level3 = append(level3, telego.KeyboardButton{Text: t("button_settings")})
	if chat.Mode == ModeTeacher {
		level3 = append(level3, telego.KeyboardButton{Text: t("button_history")})
	}
	if chat.ShowAbout {
		level3 = append(level3, telego.KeyboardButton{Text: t("button_about")})
	}
	if len(level3) > 0 {
		rows = append(rows, level3)
	}

	if len(rows) == 0 {
		rows = append(rows, []telego.KeyboardButton{
			{Text: t("button_settings")},
		})
	}

	return &telego.ReplyKeyboardMarkup{
		Keyboard:       rows,
		IsPersistent:   true,
		ResizeKeyboard: true,
	}
}
