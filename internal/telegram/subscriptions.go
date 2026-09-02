package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
)

const MAX_SUBSCRIPTIONS = 5

type subsAddGroupCb struct{ bot *Bot }

func (cb *subsAddGroupCb) Prefix() string { return "subs_add_group" }
func (cb *subsAddGroupCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	groups := cb.bot.cache.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	count, _ := cb.bot.chatRepo.CountSubscriptions(u.UserID)
	if count >= MAX_SUBSCRIPTIONS {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("Достигнут лимит подписок (%d).", MAX_SUBSCRIPTIONS))
	}

	chat.Scene = "sub_add_group"
	cb.bot.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите номер группы, на которую хотите подписаться (например, %s)", randomKey(groups))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, cb.bot.historyKeyboardFor(chat))
}

func (b *Bot) historyKeyboardFor(chat *Chat) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton
	for _, g := range chat.HistoryGroup {
		rows = append(rows, []telego.InlineKeyboardButton{{Text: g, CallbackData: "answer:" + g}})
	}
	for _, t := range chat.HistoryTeacher {
		rows = append(rows, []telego.InlineKeyboardButton{{Text: t, CallbackData: "answer:" + t}})
	}
	if len(rows) == 0 {
		return nil
	}
	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}

type subsAddTeacherCb struct{ bot *Bot }

func (cb *subsAddTeacherCb) Prefix() string { return "subs_add_teacher" }
func (cb *subsAddTeacherCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	teachers := cb.bot.cache.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	count, _ := cb.bot.chatRepo.CountSubscriptions(u.UserID)
	if count >= MAX_SUBSCRIPTIONS {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("Достигнут лимит подписок (%d).", MAX_SUBSCRIPTIONS))
	}

	chat.Scene = "sub_add_teacher"
	cb.bot.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите фамилию преподавателя (например, %s)", randomKey(cb.bot.cache.GetTeachers()))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, cb.bot.historyKeyboardFor(chat))
}

type subsListCb struct{ bot *Bot }

func (cb *subsListCb) Prefix() string { return "subs_list" }
func (cb *subsListCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return cb.bot.showSubscriptionsList(u)
}

type subsRemoveCb struct{ bot *Bot }

func (cb *subsRemoveCb) Prefix() string { return "subs_remove" }
func (cb *subsRemoveCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	list, _ := cb.bot.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendText(u.ChatID, "Подписок нет.")
	}

	chat.Scene = "sub_remove"
	cb.bot.chatRepo.Save(chat)

	return u.Bot.SendText(u.ChatID, "Введите номер подписки для удаления:\n\n"+cb.bot.formatSubscriptionsList(list))
}

type subsCheckCb struct{ bot *Bot }

func (cb *subsCheckCb) Prefix() string { return "subs_check" }
func (cb *subsCheckCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return cb.bot.showSubscriptionsList(u)
}

func (b *Bot) showSubscriptionsList(u *Update) error {
	list, _ := b.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendText(u.ChatID, "Подписок нет.")
	}

	return u.Bot.SendText(u.ChatID, b.formatSubscriptionsList(list))
}

func (b *Bot) formatSubscriptionsList(list []Subscription) string {
	var lines []string
	for i, s := range list {
		label := "Группа"
		if s.Type == "teacher" {
			label = "Преподаватель"
		}
		lines = append(lines, fmt.Sprintf("%d. %s %s", i+1, label, s.Value))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) subscriptionsKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "➕ Группа", CallbackData: "subs_add_group"},
				{Text: "➕ Преподаватель", CallbackData: "subs_add_teacher"},
			},
			{
				{Text: "📋 Мои подписки", CallbackData: "subs_list"},
				{Text: "❌ Удалить подписку", CallbackData: "subs_remove"},
				{Text: "🧪 Проверить", CallbackData: "subs_check"},
			},
			{
				{Text: "Меню настроек", CallbackData: "settings"},
				{Text: "Главное меню", CallbackData: "main_menu"},
			},
		},
	}
}
