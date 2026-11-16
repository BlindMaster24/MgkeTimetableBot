package telegram

import (
	"fmt"
	"strings"
)

const MAX_SUBSCRIPTIONS = 5

func (b *Bot) formatSubscriptionsList(list []Subscription) string {
	var lines []string
	for i, s := range list {
		label := b.loc("subs_group_label")
		if s.Type == "teacher" {
			label = b.loc("subs_teacher_label")
		}
		lines = append(lines, fmt.Sprintf("%d. %s %s", i+1, label, s.Value))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) subTestPrompt(u *Update) error {
	list, _ := b.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, b.loc("subs_empty"), b.replySubscriptionsMenu())
	}

	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}
	chat.Scene = "sub_test_pick"
	b.chatRepo.Save(chat)

	prompt := b.loc("subs_test_prompt") + b.formatSubscriptionsList(list)
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, b.replySubscriptionsMenu())
}
