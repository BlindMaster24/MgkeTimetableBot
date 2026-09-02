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
