package telegram

import "github.com/mymmrac/telego"

func mainMenuKeyboard(loc func(string, string, map[string]interface{}) string) *telego.InlineKeyboardMarkup {
	t := func(key string) string { return loc("ru", key, nil) }
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: t("button_day"), CallbackData: "day"},
				{Text: t("button_week"), CallbackData: "week"},
			},
			{
				{Text: t("button_calls"), CallbackData: "calls"},
			},
			{
				{Text: t("button_group"), CallbackData: "group"},
				{Text: t("button_teacher"), CallbackData: "teacher"},
			},
			{
				{Text: t("button_image"), CallbackData: "image"},
			},
			{
				{Text: t("button_ics"), CallbackData: "ics"},
			},
			{
				{Text: t("button_settings"), CallbackData: "settings"},
				{Text: t("button_about"), CallbackData: "about"},
			},
		},
	}
}

func settingsKeyboard(loc func(string, string, map[string]interface{}) string) *telego.InlineKeyboardMarkup {
	t := func(key string) string { return loc("ru", key, nil) }
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: t("button_setup"), CallbackData: "setup"},
			},
			{
				{Text: t("button_cancel"), CallbackData: "cancel"},
			},
		},
	}
}

func cancelKeyboard(loc func(string, string, map[string]interface{}) string) *telego.InlineKeyboardMarkup {
	t := func(key string) string { return loc("ru", key, nil) }
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: t("button_cancel"), CallbackData: "cancel"},
			},
		},
	}
}

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
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "На неделю", CallbackData: "timetable_" + string(typeName[0]) + ":" + value + ":0:0:1"}},
		},
	}
}

func selectModeKeyboard(loc func(string, string, map[string]interface{}) string) *telego.InlineKeyboardMarkup {
	t := func(key string) string { return loc("ru", key, nil) }
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: t("mode_guest"), CallbackData: "setup:guest"},
			},
			{
				{Text: t("mode_student"), CallbackData: "setup:student"},
				{Text: t("mode_teacher"), CallbackData: "setup:teacher"},
			},
			{
				{Text: t("mode_parent"), CallbackData: "setup:parent"},
				{Text: "🔙 Пропустить", CallbackData: "cancel"},
			},
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
	canShowCalls := chat.ShowCalls && b.cfg.Parser.Calls != nil && b.cfg.Parser.Calls.Enabled

	var level2 []telego.KeyboardButton
	if showFast && chat.ShowFastGroup {
		level2 = append(level2, telego.KeyboardButton{Text: t("button_group")})
	}
	if chat.ShowAbout && canShowCalls {
		level2 = append(level2, telego.KeyboardButton{Text: t("button_calls")})
	}
	if showFast && chat.ShowFastTeacher {
		label := t("button_teacher")
		if chat.ShowAbout && canShowCalls && chat.ShowFastGroup {
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
		Keyboard:        rows,
		IsPersistent:    true,
		ResizeKeyboard:  true,
	}
}

func sendReplyMainMenu(b *Bot, chatID int64, chat *Chat, text string) error {
	kb := replyMainMenu(b, chat)
	return b.SendTextWithReplyKeyboard(chatID, text, kb)
}
