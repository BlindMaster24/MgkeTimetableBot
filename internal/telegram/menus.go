package telegram

import (
	"context"

	"github.com/mymmrac/telego"
)

type menuSpec struct {
	id       string
	name     string
	desc     string
	scene    string
	prompt   string
	openText []string
	keyboard func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup
	items    []func(b *Bot) Command
	open     func(b *Bot, u *Update, chat *Chat) error
}

func (b *Bot) menuSpecs() []menuSpec {
	return []menuSpec{
		{
			id:       "settings",
			name:     "settings",
			desc:     b.loc("cmd_settings"),
			scene:    sceneSettings,
			prompt:   b.loc("settings_menu"),
			openText: []string{b.loc("button_settings"), "Настройки"},
			keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsMain() },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &formatterSelectTextCmd{bot: b} },
				func(b *Bot) Command { return &settingsNavTextCmd{bot: b, kind: "to_settings"} },
				func(b *Bot) Command { return &settingsNavTextCmd{bot: b, kind: "to_main"} },
				func(b *Bot) Command { return &showCurrentSettingsTextCmd{bot: b} },
			},
		},
		{
			id:       "buttons",
			name:     "buttons",
			desc:     b.loc("menu_buttons_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_buttons_prompt"),
			openText: []string{b.loc("menu_buttons_label")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsButtons(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "daily"} },
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "weekly"} },
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "calls"} },
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "about"} },
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "fast_group"} },
				func(b *Bot) Command { return &btnToggleTextCmd{bot: b, kind: "fast_teacher"} },
			},
		},
		{
			id:       "formatter",
			name:     "formatter",
			desc:     b.loc("menu_formatter_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_formatter_prompt"),
			openText: []string{b.loc("menu_formatter_label")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsFormatters(chat) },
		},
		{
			id:       "notice",
			name:     "notice",
			desc:     b.loc("menu_notice_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_notice_prompt"),
			openText: []string{b.loc("menu_notice_label")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsNotice(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "changes"} },
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "next_week"} },
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "calls"} },
			},
		},
		{
			id:       "reminders",
			scene:    sceneSettings,
			prompt:   b.loc("settings_reminders_menu"),
			openText: []string{b.loc("button_settings_reminders")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsReminders(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 0} },
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 5} },
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 10} },
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 15} },
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 30} },
				func(b *Bot) Command { return &reminderLeadTextCmd{bot: b, minutes: 60} },
			},
		},
		{
			id:       "view",
			name:     "view",
			desc:     b.loc("menu_view_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_view_prompt"),
			openText: []string{b.loc("menu_view_label")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsView(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &viewToggleTextCmd{bot: b, kind: "hide_past_days"} },
				func(b *Bot) Command { return &viewToggleTextCmd{bot: b, kind: "show_parser_time"} },
				func(b *Bot) Command { return &viewToggleTextCmd{bot: b, kind: "show_hints"} },
			},
		},
		{
			id:       "diff",
			name:     "diff",
			desc:     b.loc("menu_diff_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_diff_prompt"),
			openText: []string{b.loc("menu_diff_label")},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsDiff(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "enabled"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "max_lines"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "advanced"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "auto_week"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "auto_updates"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "before_after"} },
				func(b *Bot) Command { return &diffToggleTextCmd{bot: b, kind: "back_basic"} },
			},
		},
		{
			id:       "schedules",
			scene:    sceneSettingsSchedules,
			prompt:   b.loc("menu_schedules_prompt"),
			openText: []string{b.loc("menu_schedules_label")},
			keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsSchedules() },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &callsManageTextCmd{bot: b, menu: "calls"} },
			},
		},
		{
			id:       "calls",
			scene:    sceneSettingsCalls,
			openText: []string{b.loc("menu_calls_label")},
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "show"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "refresh"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "edit"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "source_site"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "source_manual"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "source_config"} },
				func(b *Bot) Command { return &callsSettingsTextCmd{bot: b, kind: "source_auto"} },
			},
			open: func(b *Bot, u *Update, chat *Chat) error {
				return b.showCallsSettingsReply(u, chat)
			},
		},
		{
			id:       "aliases",
			scene:    sceneSettingsAlias,
			prompt:   b.loc("menu_aliases_prompt"),
			openText: []string{b.loc("menu_aliases_label"), b.loc("menu_aliases_label_alt")},
			keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsAliases() },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &aliasActionTextCmd{bot: b, kind: "list"} },
				func(b *Bot) Command { return &aliasActionTextCmd{bot: b, kind: "add"} },
				func(b *Bot) Command { return &aliasActionTextCmd{bot: b, kind: "remove"} },
				func(b *Bot) Command { return &aliasActionTextCmd{bot: b, kind: "clear"} },
			},
		},
		{
			id:       "subscriptions",
			name:     "subscriptions",
			desc:     b.loc("menu_subs_desc"),
			scene:    sceneSettings,
			prompt:   b.loc("menu_subs_prompt"),
			openText: []string{b.loc("menu_subs_label"), b.loc("menu_subs_label_alt")},
			keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySubscriptionsMenu() },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &subsActionTextCmd{bot: b, kind: "add_group"} },
				func(b *Bot) Command { return &subsActionTextCmd{bot: b, kind: "add_teacher"} },
				func(b *Bot) Command { return &subsActionTextCmd{bot: b, kind: "list"} },
				func(b *Bot) Command { return &subsActionTextCmd{bot: b, kind: "remove"} },
				func(b *Bot) Command { return &subsActionTextCmd{bot: b, kind: "test"} },
			},
		},
		{
			id:    "setup",
			scene: sceneSetup,
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &setupModeTextCmd{bot: b, kind: "guest"} },
				func(b *Bot) Command { return &setupModeTextCmd{bot: b, kind: "student"} },
				func(b *Bot) Command { return &setupModeTextCmd{bot: b, kind: "parent"} },
				func(b *Bot) Command { return &setupModeTextCmd{bot: b, kind: "skip"} },
			},
		},
	}
}

func (b *Bot) menuByID(id string) (menuSpec, bool) {
	for _, spec := range b.menuSpecs() {
		if spec.id == id {
			return spec, true
		}
	}
	return menuSpec{}, false
}

func (b *Bot) openMenu(u *Update, chat *Chat, spec menuSpec) error {
	chat.Scene = spec.scene
	if err := b.chatRepo.Save(chat); err != nil {
		return err
	}
	if spec.open != nil {
		return spec.open(b, u, chat)
	}
	return b.SendTextWithReplyKeyboard(u.ChatID, spec.prompt, spec.keyboard(b, chat))
}

func (b *Bot) registerMenus() {
	for _, spec := range b.menuSpecs() {
		if spec.name != "" {
			b.RegisterCommand(&menuCommand{bot: b, spec: spec})
		} else if len(spec.openText) > 0 {
			b.RegisterTextCommand(&menuCommand{bot: b, spec: spec})
		}
		for _, build := range spec.items {
			b.RegisterTextCommand(build(b))
		}
	}
}

type menuCommand struct {
	bot  *Bot
	spec menuSpec
}

func (c *menuCommand) Name() string {
	if c.spec.name != "" {
		return "/" + c.spec.name
	}
	return "/menu_" + c.spec.id
}

func (c *menuCommand) Description() string { return c.spec.desc }

func (c *menuCommand) Hidden() bool { return c.spec.name == "" }

func (c *menuCommand) AdminOnly() bool { return false }

func (c *menuCommand) MatchText(text string) bool {
	for _, candidate := range c.spec.openText {
		if text == candidate {
			return true
		}
	}
	return false
}

func (c *menuCommand) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.openMenu(u, chat, c.spec)
}
