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
