package telegram

import (
	"context"
	"strings"
)

// textFormatCmd switches how the chat's messages are formatted on the wire:
// MarkdownV2 by default, HTML when the user asks for it.
type textFormatCmd struct{ bot *Bot }

func (c *textFormatCmd) Name() string        { return "/format" }
func (c *textFormatCmd) Description() string { return c.bot.loc("cmd_format") }

func (c *textFormatCmd) MatchText(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	if strings.EqualFold(fields[0], c.Name()) {
		return true
	}
	return strings.EqualFold(text, strings.TrimSuffix(c.bot.loc("cmd_format"), ":"))
}

func (c *textFormatCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	fields := strings.Fields(u.Text)
	if len(fields) < 2 {
		return u.Bot.SendText(u.ChatID, c.bot.locData("format_current", map[string]interface{}{"Format": formatName(chat.TextFormat)}))
	}

	var chosen string
	switch strings.ToLower(fields[1]) {
	case "markdown", "md", "m":
		chosen = FormatMarkdown
	case "html", "h":
		chosen = FormatHTML
	default:
		return u.Bot.SendText(u.ChatID, c.bot.loc("format_unknown"))
	}

	chat.TextFormat = chosen
	c.bot.saveChat(chat)
	return u.Bot.SendText(u.ChatID, c.bot.locData("format_set", map[string]interface{}{"Format": formatName(chosen)}))
}

// formatName spells the stored value for the user.
func formatName(format string) string {
	if format == FormatHTML {
		return "HTML"
	}
	return "Markdown"
}
