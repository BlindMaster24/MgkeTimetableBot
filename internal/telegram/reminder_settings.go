package telegram

import (
	"context"
	"strconv"
	"strings"
)

var reminderLeadPresets = []int{0, 5, 10, 15, 30, 60}

func reminderLeadKey(minutes int) string {
	if minutes <= 0 {
		return "reminder_lead_default"
	}
	return "reminder_lead_" + strconv.Itoa(minutes)
}

func (b *Bot) reminderLeadLabel(minutes int) string {
	return b.loc(reminderLeadKey(minutes))
}

func (b *Bot) reminderLeadReply(chat *Chat) string {
	label := b.reminderLeadLabel(chat.ReminderLeadMinutes)
	effective := chat.ReminderLeadMinutes
	if effective <= 0 {
		effective = b.cfg.Timetable.ReminderLeadMinutes
	}
	if effective <= 0 {
		return b.locData("reminder_lead_disabled", map[string]any{"Label": label})
	}
	return b.locData("reminder_lead_set", map[string]any{"Label": label, "Value": effective})
}

type reminderLeadTextCmd struct {
	bot     *Bot
	minutes int
}

func (c *reminderLeadTextCmd) Name() string {
	return "/reminder_lead_text_" + strconv.Itoa(c.minutes)
}

func (c *reminderLeadTextCmd) Description() string { return "" }
func (c *reminderLeadTextCmd) Scene() string       { return sceneSettings }

func (c *reminderLeadTextCmd) MatchText(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), c.bot.reminderLeadLabel(c.minutes))
}

func (c *reminderLeadTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	chat.ReminderLeadMinutes = c.minutes
	if err := c.bot.chatRepo.Save(chat); err != nil {
		return err
	}
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.reminderLeadReply(chat), c.bot.replySettingsReminders(chat))
}
