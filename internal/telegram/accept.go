package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type acceptBotCmd struct{ bot *Bot }

func (c *acceptBotCmd) AdminOnly() bool { return true }

func (c *acceptBotCmd) Hidden() bool { return true }

func (c *acceptBotCmd) Name() string        { return "/acceptbot" }
func (c *acceptBotCmd) Description() string { return "Выдать доступ чату" }
func (c *acceptBotCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/acceptbot")
}

func (c *acceptBotCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	raw := strings.TrimSpace(u.Text[len("/acceptBot"):])
	if raw == "" {
		return u.Bot.SendText(u.ChatID, "Введите id пользователя")
	}

	peerID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "это не число")
	}

	updated, err := c.bot.chatRepo.Accept("telegram", peerID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка выдачи доступа: "+err.Error())
	}

	if !updated {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("Чат %d не найден", peerID))
	}

	return u.Bot.SendText(u.ChatID, fmt.Sprintf("ok:%d", peerID))
}
