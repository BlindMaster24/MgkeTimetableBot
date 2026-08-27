package telegram

import (
	"context"
	"fmt"

	"github.com/mymmrac/telego"
)

func (b *Bot) loc(key string) string {
	return b.i18n.T("ru", key, nil)
}

type startCmd struct{ bot *Bot }

func (c *startCmd) Name() string        { return "/start" }
func (c *startCmd) Description() string { return c.bot.loc("cmd_start") }
func (c *startCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("welcome"), mainMenuKeyboard(c.bot.i18n.T))
}

type helpCmd struct{ bot *Bot }

func (c *helpCmd) Name() string        { return "/help" }
func (c *helpCmd) Description() string { return c.bot.loc("cmd_help") }
func (c *helpCmd) Handler(ctx context.Context, u *Update) error {
	text := c.bot.loc("help_commands")
	for _, cmd := range c.bot.commands {
		text += fmt.Sprintf("\n/%s — %s", cmd.Name(), cmd.Description())
	}
	return u.Bot.SendText(u.ChatID, text)
}

type cancelCmd struct{ bot *Bot }

func (c *cancelCmd) Name() string        { return "/cancel" }
func (c *cancelCmd) Description() string { return c.bot.loc("cmd_cancel") }
func (c *cancelCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_cancel")
}
func (c *cancelCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("input_cancelled"))
}

type setupCmd struct{ bot *Bot }

func (c *setupCmd) Name() string        { return "/setup" }
func (c *setupCmd) Description() string { return c.bot.loc("cmd_setup") }
func (c *setupCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_setup")
}
func (c *setupCmd) Handler(ctx context.Context, u *Update) error {
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: c.bot.loc("mode_student"), CallbackData: "setup:student"},
				{Text: c.bot.loc("mode_teacher"), CallbackData: "setup:teacher"},
			},
			{
				{Text: c.bot.loc("mode_parent"), CallbackData: "setup:parent"},
				{Text: c.bot.loc("mode_guest"), CallbackData: "setup:guest"},
			},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("setup_select_mode"), kb)
}

type dayCmd struct{ bot *Bot }

func (c *dayCmd) Name() string        { return "/day" }
func (c *dayCmd) Description() string { return c.bot.loc("cmd_day") }
func (c *dayCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_day")
}
func (c *dayCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}

type weekCmd struct{ bot *Bot }

func (c *weekCmd) Name() string        { return "/week" }
func (c *weekCmd) Description() string { return c.bot.loc("cmd_week") }
func (c *weekCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_week")
}
func (c *weekCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}

type callsCmd struct{ bot *Bot }

func (c *callsCmd) Name() string        { return "/calls" }
func (c *callsCmd) Description() string { return c.bot.loc("cmd_calls") }
func (c *callsCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_calls")
}
func (c *callsCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.formatCallsSchedule())
}

type aboutCmd struct{ bot *Bot }

func (c *aboutCmd) Name() string        { return "/about" }
func (c *aboutCmd) Description() string { return c.bot.loc("cmd_about") }
func (c *aboutCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_about")
}
func (c *aboutCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("about_bot"))
}

type groupCmd struct{ bot *Bot }

func (c *groupCmd) Name() string        { return "/group" }
func (c *groupCmd) Description() string { return c.bot.loc("cmd_group") }
func (c *groupCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_group")
}
func (c *groupCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("enter_group_number"))
}

type teacherCmd struct{ bot *Bot }

func (c *teacherCmd) Name() string        { return "/teacher" }
func (c *teacherCmd) Description() string { return c.bot.loc("cmd_teacher") }
func (c *teacherCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_teacher")
}
func (c *teacherCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("enter_teacher_name"))
}

type settingsCmd struct{ bot *Bot }

func (c *settingsCmd) Name() string        { return "/settings" }
func (c *settingsCmd) Description() string { return c.bot.loc("cmd_settings") }
func (c *settingsCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_settings")
}
func (c *settingsCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("settings_menu"), settingsKeyboard(c.bot.i18n.T))
}

type imageCmd struct{ bot *Bot }

func (c *imageCmd) Name() string        { return "/image" }
func (c *imageCmd) Description() string { return c.bot.loc("cmd_image") }
func (c *imageCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_image")
}
func (c *imageCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}
