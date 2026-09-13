package telegram

import (
	"fmt"
	"strings"
)

const MAX_SUBSCRIPTIONS = 5

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

func (b *Bot) subTestPrompt(u *Update) error {
	list, _ := b.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Подписок нет.", b.replySubscriptionsMenu())
	}

	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}
	chat.Scene = "sub_test_pick"
	b.chatRepo.Save(chat)

	prompt := "Введите номер подписки для проверки:\n" + b.formatSubscriptionsList(list)
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, b.replySubscriptionsMenu())
}
