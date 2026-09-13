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
			desc:     "Настройки кнопок бота",
			scene:    sceneSettings,
			prompt:   "Меню настройки кнопок.",
			openText: []string{"⌨️ Кнопки"},
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
			desc:     "Настройки форматировщика",
			scene:    sceneSettings,
			prompt:   "Меню настройки форматировщика.",
			openText: []string{"📃 Форматировщик"},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsFormatters(chat) },
		},
		{
			id:       "notice",
			name:     "notice",
			desc:     "Настройки оповещений",
			scene:    sceneSettings,
			prompt:   "Меню настройки оповещений.",
			openText: []string{"🔊 Оповещения"},
			keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsNotice(chat) },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "changes"} },
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "next_week"} },
				func(b *Bot) Command { return &noticeToggleTextCmd{bot: b, kind: "calls"} },
			},
		},
		{
			id:       "view",
			name:     "view",
			desc:     "Настройки отображения расписания",
			scene:    sceneSettings,
			prompt:   "Меню настройки отображения внешнего вида расписания.",
			openText: []string{"🖼️ Отображение"},
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
			desc:     "Настройки отображения изменений расписания",
			scene:    sceneSettings,
			prompt:   "Меню настроек раздела \"Что изменилось\".",
			openText: []string{"📊 Сравнение"},
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
			prompt:   "Управление расписаниями.",
			openText: []string{"🗓️ Управление расписаниями"},
			keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySettingsSchedules() },
			items: []func(b *Bot) Command{
				func(b *Bot) Command { return &callsManageTextCmd{bot: b, menu: "calls"} },
			},
		},
		{
			id:       "calls",
			scene:    sceneSettingsCalls,
			openText: []string{"🕐 Звонки: управление"},
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
			prompt:   "Меню настройки алиасов.",
			openText: []string{"Алиасы", "Настройка алиасов"},
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
			desc:     "Управление подписками на другие группы/преподавателей",
			scene:    sceneSettings,
			prompt:   "Подписки позволяют получать уведомления об изменениях расписания другой группы или преподавателя.",
			openText: []string{"🔔 Подписки", "Подписки"},
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
