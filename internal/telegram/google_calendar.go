package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
)

type googleCalendarCmd struct{ bot *Bot }

func (c *googleCalendarCmd) Name() string        { return "/google_calendar" }
func (c *googleCalendarCmd) Description() string { return "Настройка Google Calendar" }
func (c *googleCalendarCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_google_calendar")
}
func (c *googleCalendarCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if c.bot.cfg.Google.OAuth.ClientID == "" {
		return u.Bot.SendText(u.ChatID, "Google Calendar не настроен на сервере.")
	}
	return c.bot.showGoogleCalendarMenu(u, chat)
}

func (b *Bot) showGoogleCalendarMenu(u *Update, chat *Chat) error {
	if chat.GoogleEmail == "" {
		return b.showGoogleAuth(u)
	}
	msg := fmt.Sprintf("Привязанный гугл аккаунт: %s.\nДействие с календарями:", chat.GoogleEmail)
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Список", CallbackData: "gcal:list"}, {Text: "Добавить", CallbackData: "gcal:add"}},
			{{Text: "Права", CallbackData: "gcal:permissions"}},
			{{Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, msg, kb)
}

func (b *Bot) showGoogleAuth(u *Update) error {
	if b.cfg.Google.OAuth.ClientID == "" {
		return b.SendText(u.ChatID, "Google Calendar не настроен.")
	}
	redirectURI := strings.TrimRight(b.cfg.Google.RedirectDomain, "/") + b.cfg.Google.URL
	url := fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=https://www.googleapis.com/auth/calendar&access_type=offline&prompt=consent",
		b.cfg.Google.OAuth.ClientID,
		redirectURI,
	)
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Привязать Google аккаунт", URL: url}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Гугл аккаунт не привязан, чтобы привязать, нажмите на кнопку ниже", kb)
}

type googleCalCb struct{ bot *Bot }

func (cb *googleCalCb) Prefix() string { return "gcal:" }
func (cb *googleCalCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	action := strings.TrimPrefix(u.Data, "gcal:")

	switch action {
	case "menu":
		return cb.bot.showGoogleCalendarMenu(u, chat)
	case "list":
		return cb.bot.showGoogleCalendarList(u, chat)
	case "add":
		return cb.bot.showGoogleCalendarAdd(u, chat)
	case "permissions":
		return cb.bot.showGoogleCalendarPermissions(u, chat)
	}

	return cb.bot.showGoogleCalendarMenu(u, chat)
}

func (b *Bot) showGoogleCalendarList(u *Update, chat *Chat) error {
	if chat.GoogleEmail == "" {
		return b.showGoogleAuth(u)
	}
	msg := "Нет добавленных календарей"
	if chat.Mode == ModeStudent && chat.Group != "" {
		msg = fmt.Sprintf("Календарь группы %s будет добавлен при настройке.", chat.Group)
	} else if chat.Mode == ModeTeacher && chat.Teacher != "" {
		msg = fmt.Sprintf("Календарь преподавателя %s будет добавлен при настройке.", chat.Teacher)
	}
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Назад", CallbackData: "gcal:menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, msg, kb)
}

func (b *Bot) showGoogleCalendarAdd(u *Update, chat *Chat) error {
	if chat.GoogleEmail == "" {
		return b.showGoogleAuth(u)
	}
	var msg string
	switch chat.Mode {
	case ModeStudent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, "Вы ещё не выбрали группу")
		}
		msg = fmt.Sprintf("Календарь для группы %s будет добавлен.", chat.Group)
	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, "Вы ещё не выбрали преподавателя")
		}
		msg = fmt.Sprintf("Календарь для преподавателя %s будет добавлен.", chat.Teacher)
	default:
		return u.Bot.SendText(u.ChatID, "Режим чата не позволяет добавить календарь")
	}
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Назад", CallbackData: "gcal:menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, msg, kb)
}

func (b *Bot) showGoogleCalendarPermissions(u *Update, chat *Chat) error {
	if chat.GoogleEmail == "" {
		return b.showGoogleAuth(u)
	}
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Назад", CallbackData: "gcal:menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Выберите календарь для управления правами:", kb)
}
