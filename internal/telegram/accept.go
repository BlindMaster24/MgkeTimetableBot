// Package telegram is the bot: commands, callbacks, menus, keyboards and scenes.
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
func (c *acceptBotCmd) Description() string { return c.bot.loc("accept_desc") }
func (c *acceptBotCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/acceptbot")
}

func (c *acceptBotCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("access_denied"))
	}

	raw := strings.TrimSpace(u.Text[len("/acceptBot"):])
	if raw == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("accept_enter_id"))
	}

	peerID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("accept_not_number"))
	}

	updated, err := c.bot.chatRepo.Accept("telegram", peerID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("accept_error_prefix")+err.Error())
	}

	if !updated {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("accept_not_found"), peerID))
	}

	return u.Bot.SendText(u.ChatID, fmt.Sprintf("ok:%d", peerID))
}
