package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
)

func (b *Bot) sendSettingsMenu(u *Update, chat *Chat) error {
	chat.Scene = sceneSettings
	b.saveChat(chat)
	return b.SendTextWithReplyKeyboard(u.ChatID, b.loc("settings_menu"), b.replySettingsMain())
}

type btnToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *btnToggleTextCmd) Name() string        { return "/btn_toggle_text_" + c.kind }
func (c *btnToggleTextCmd) Description() string { return "" }
func (c *btnToggleTextCmd) Scene() string       { return sceneSettings }

func (c *btnToggleTextCmd) MatchText(text string) bool {
	pairs := map[string][2]string{
		"daily":        {"📄", c.bot.loc("settext_1")},
		"weekly":       {"📑", c.bot.loc("settext_2")},
		"calls":        {"🕐", c.bot.loc("settext_3")},
		"about":        {"💡", c.bot.loc("settext_4")},
		"fast_group":   {"👩‍🎓", c.bot.loc("settext_5")},
		"fast_teacher": {"👩‍🏫", c.bot.loc("settext_6")},
	}
	pair, ok := pairs[c.kind]
	if !ok {
		return false
	}
	variants := []string{
		c.bot.loc("settext_80") + pair[0] + ` ` + pair[1] + `"`,
		c.bot.loc("settext_81") + pair[0] + ` ` + pair[1] + `"`,
		c.bot.loc("settext_82") + pair[1] + `"`,
		c.bot.loc("settext_83") + pair[1] + `"`,
	}
	for _, variant := range variants {
		if strings.EqualFold(text, variant) {
			return true
		}
	}
	return false
}

func (c *btnToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "daily":
		chat.ShowDaily = !chat.ShowDaily
		reply = fmt.Sprintf(c.bot.loc("settext_7"), c.bot.yesNo(chat.ShowDaily))
	case "weekly":
		chat.ShowWeekly = !chat.ShowWeekly
		reply = fmt.Sprintf(c.bot.loc("settext_8"), c.bot.yesNo(chat.ShowWeekly))
	case "calls":
		chat.ShowCalls = !chat.ShowCalls
		reply = fmt.Sprintf(c.bot.loc("settext_9"), c.bot.yesNo(chat.ShowCalls))
	case "about":
		chat.ShowAbout = !chat.ShowAbout
		reply = fmt.Sprintf(c.bot.loc("settext_10"), c.bot.yesNo(chat.ShowAbout))
	case "fast_group":
		chat.ShowFastGroup = !chat.ShowFastGroup
		reply = fmt.Sprintf(c.bot.loc("settext_11"), c.bot.yesNo(chat.ShowFastGroup))
	case "fast_teacher":
		chat.ShowFastTeacher = !chat.ShowFastTeacher
		reply = fmt.Sprintf(c.bot.loc("settext_12"), c.bot.yesNo(chat.ShowFastTeacher))
	}
	c.bot.saveChat(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsButtons(chat))
}

type noticeToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *noticeToggleTextCmd) Name() string        { return "/notice_toggle_text_" + c.kind }
func (c *noticeToggleTextCmd) Description() string { return "" }
func (c *noticeToggleTextCmd) Scene() string       { return sceneSettings }

func (c *noticeToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "changes":
		return text == c.bot.loc("settext_13") || text == c.bot.loc("settext_14")
	case "next_week":
		return text == c.bot.loc("settext_15") || text == c.bot.loc("settext_16")
	case "calls":
		return text == c.bot.loc("settext_17") || text == c.bot.loc("settext_18")
	}
	return false
}

func (c *noticeToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "changes":
		chat.NoticeChanges = !chat.NoticeChanges
		reply = fmt.Sprintf(c.bot.loc("settext_19"), c.bot.onOff(chat.NoticeChanges))
	case "next_week":
		chat.NoticeNextWeek = !chat.NoticeNextWeek
		reply = fmt.Sprintf(c.bot.loc("settext_20"), c.bot.onOff(chat.NoticeNextWeek))
	case "calls":
		chat.NoticeCalls = !chat.NoticeCalls
		reply = fmt.Sprintf(c.bot.loc("settext_21"), c.bot.onOff(chat.NoticeCalls))
	}
	c.bot.saveChat(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsNotice(chat))
}

type viewToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *viewToggleTextCmd) Name() string        { return "/view_toggle_text_" + c.kind }
func (c *viewToggleTextCmd) Description() string { return "" }
func (c *viewToggleTextCmd) Scene() string       { return sceneSettings }

func (c *viewToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "hide_past_days":
		return text == c.bot.loc("settext_22") || text == c.bot.loc("settext_23")
	case "show_parser_time":
		return text == c.bot.loc("settext_24") || text == c.bot.loc("settext_25")
	case "show_hints":
		return text == c.bot.loc("settext_26") || text == c.bot.loc("settext_27")
	case "delete_last_msg":
		return text == c.bot.noYesSmile(true, c.bot.loc("kbtext_53")) || text == c.bot.noYesSmile(false, c.bot.loc("kbtext_53"))
	case "delete_user_msg":
		return text == c.bot.noYesSmile(true, c.bot.loc("kbtext_54")) || text == c.bot.noYesSmile(false, c.bot.loc("kbtext_54"))
	case "show_gcal":
		return text == c.bot.noYesSmile(true, c.bot.loc("kbtext_55")) || text == c.bot.noYesSmile(false, c.bot.loc("kbtext_55"))
	case "show_ics":
		return text == c.bot.noYesSmile(true, c.bot.loc("kbtext_56")) || text == c.bot.noYesSmile(false, c.bot.loc("kbtext_56"))
	}
	return false
}

func (c *viewToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	var reply string
	switch c.kind {
	case "hide_past_days":
		chat.HidePastDays = !chat.HidePastDays
		reply = fmt.Sprintf(c.bot.loc("settext_28"), c.bot.yesNo(chat.HidePastDays))
	case "show_parser_time":
		chat.ShowParserTime = !chat.ShowParserTime
		reply = fmt.Sprintf(c.bot.loc("settext_29"), c.bot.yesNo(chat.ShowParserTime))
	case "show_hints":
		chat.ShowHints = !chat.ShowHints
		reply = fmt.Sprintf(c.bot.loc("settext_30"), c.bot.yesNo(chat.ShowHints))
	case "delete_last_msg":
		chat.DeleteLastMsg = !chat.DeleteLastMsg
		reply = fmt.Sprintf(c.bot.loc("settext_31"), c.bot.yesNo(chat.DeleteLastMsg))
	case "delete_user_msg":
		chat.DeleteUserMsg = !chat.DeleteUserMsg
		reply = fmt.Sprintf(c.bot.loc("settext_32"), c.bot.yesNo(chat.DeleteUserMsg))
	case "show_gcal":
		chat.ShowGoogleCalendar = !chat.ShowGoogleCalendar
		reply = fmt.Sprintf(c.bot.loc("settext_33"), c.bot.yesNo(chat.ShowGoogleCalendar))
	case "show_ics":
		chat.ShowICS = !chat.ShowICS
		reply = fmt.Sprintf(c.bot.loc("settext_34"), c.bot.yesNo(chat.ShowICS))
	}
	c.bot.saveChat(chat)
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsView(chat))
}

type diffToggleTextCmd struct {
	bot  *Bot
	kind string
}

func (c *diffToggleTextCmd) Name() string        { return "/diff_toggle_text_" + c.kind }
func (c *diffToggleTextCmd) Description() string { return "" }
func (c *diffToggleTextCmd) Scene() string       { return sceneSettings }

func (c *diffToggleTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "enabled":
		return text == c.bot.loc("settext_84") || text == c.bot.loc("settext_85")
	case "max_lines":
		return strings.HasPrefix(text, c.bot.loc("settext_31"))
	case "advanced":
		return text == c.bot.loc("settext_32")
	case "auto_week":
		return text == c.bot.loc("settext_33") || text == c.bot.loc("settext_34")
	case "auto_updates":
		return text == c.bot.loc("settext_35") || text == c.bot.loc("settext_36")
	case "before_after":
		return text == c.bot.loc("settext_86") || text == c.bot.loc("settext_87")
	case "back_basic":
		return text == c.bot.loc("settext_37")
	}
	return false
}

func (c *diffToggleTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "enabled":
		chat.DiffEnabled = !chat.DiffEnabled
		c.bot.saveChat(chat)
		reply := fmt.Sprintf(c.bot.loc("settext_38"), c.bot.yesNo(chat.DiffEnabled))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiff(chat))
	case "max_lines":
		presets := []int{10, 20, 30, 50}
		idx := 1
		for i, p := range presets {
			if p == chat.DiffMaxLines {
				idx = i
				break
			}
		}
		chat.DiffMaxLines = presets[(idx+1)%len(presets)]
		c.bot.saveChat(chat)
		reply := fmt.Sprintf(c.bot.loc("settext_39"), chat.DiffMaxLines)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiff(chat))
	case "advanced":
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_40"), c.bot.replySettingsDiffAdvanced(chat))
	case "auto_week":
		chat.DiffAutoInWeek = !chat.DiffAutoInWeek
		c.bot.saveChat(chat)
		reply := fmt.Sprintf(c.bot.loc("settext_41"), c.bot.yesNo(chat.DiffAutoInWeek))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "auto_updates":
		chat.DiffAutoInUpdates = !chat.DiffAutoInUpdates
		c.bot.saveChat(chat)
		reply := fmt.Sprintf(c.bot.loc("settext_42"), c.bot.yesNo(chat.DiffAutoInUpdates))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "before_after":
		chat.DiffShowBeforeAfter = !chat.DiffShowBeforeAfter
		c.bot.saveChat(chat)
		reply := fmt.Sprintf(c.bot.loc("settext_43"), c.bot.yesNo(chat.DiffShowBeforeAfter))
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, reply, c.bot.replySettingsDiffAdvanced(chat))
	case "back_basic":
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_44"), c.bot.replySettingsDiff(chat))
	}
	return nil
}

type formatterSelectTextCmd struct {
	bot *Bot
}

func (c *formatterSelectTextCmd) Name() string        { return "/formatter_select_text" }
func (c *formatterSelectTextCmd) Description() string { return "" }
func (c *formatterSelectTextCmd) Scene() string       { return sceneSettings }

func (c *formatterSelectTextCmd) MatchText(text string) bool {
	opts := formatter.FormatOptions{Labels: c.bot.formatterLabels()}
	for _, f := range formatter.AllFormatters {
		if strings.EqualFold(text, f.Label(opts)) || strings.EqualFold(text, f.Label(opts)+c.bot.loc("settext_45")) {
			return true
		}
	}
	return false
}

func (c *formatterSelectTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	opts := formatter.FormatOptions{Labels: c.bot.formatterLabels()}
	for i, f := range formatter.AllFormatters {
		if strings.HasPrefix(u.Text, f.Label(opts)) {
			chat.Formatter = i
			c.bot.saveChat(chat)
			return c.bot.SendTextWithReplyKeyboard(
				u.ChatID,
				fmt.Sprintf(c.bot.loc("settext_46"), f.Label(opts)),
				c.bot.replySettingsFormatters(chat),
			)
		}
	}
	return nil
}

type settingsNavTextCmd struct {
	bot  *Bot
	kind string
}

func (c *settingsNavTextCmd) Name() string        { return "/settings_nav_" + c.kind }
func (c *settingsNavTextCmd) Description() string { return "" }

func (c *settingsNavTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "to_settings":
		return text == c.bot.loc("settext_47")
	case "to_main":
		return text == c.bot.loc("settext_48")
	}
	return false
}

func (c *settingsNavTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if c.kind == "to_settings" {
		return c.bot.sendSettingsMenu(u, chat)
	}

	chat.Scene = ""
	c.bot.saveChat(chat)
	return c.bot.showSchedule(u, chat)
}

type callsManageTextCmd struct {
	bot  *Bot
	menu string
}

func (c *callsManageTextCmd) Name() string        { return "/calls_manage_text" }
func (c *callsManageTextCmd) Description() string { return "" }
func (c *callsManageTextCmd) Scene() string       { return sceneSettingsSchedules }

func (c *callsManageTextCmd) MatchText(text string) bool {
	return text == c.bot.loc("settext_49")
}

func (c *callsManageTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	spec, ok := c.bot.menuByID(c.menu)
	if !ok {
		return nil
	}
	return c.bot.openMenu(u, chat, spec)
}

type showCurrentSettingsTextCmd struct {
	bot *Bot
}

func (c *showCurrentSettingsTextCmd) Name() string        { return "/show_current_settings_text" }
func (c *showCurrentSettingsTextCmd) Description() string { return "" }
func (c *showCurrentSettingsTextCmd) Scene() string       { return sceneSettings }

func (c *showCurrentSettingsTextCmd) MatchText(text string) bool {
	return text == c.bot.loc("settext_50")
}

func (c *showCurrentSettingsTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, b_currentSettingsText(c.bot, chat), c.bot.replySettingsMain())
}

func b_currentSettingsText(b *Bot, chat *Chat) string {
	return b.currentSettingsText(chat)
}

type aliasActionTextCmd struct {
	bot  *Bot
	kind string
}

func (c *aliasActionTextCmd) Name() string        { return "/alias_action_text_" + c.kind }
func (c *aliasActionTextCmd) Description() string { return "" }
func (c *aliasActionTextCmd) Scene() string       { return sceneSettingsAlias }

func (c *aliasActionTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "list":
		return text == c.bot.loc("settext_51")
	case "add":
		return text == c.bot.loc("settext_52")
	case "remove":
		return text == c.bot.loc("settext_53")
	case "clear":
		return text == c.bot.loc("settext_54")
	}
	return false
}

func (c *aliasActionTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "list":
		return c.bot.showAliasList(u, u.UserID)
	case "add":
		chat.Scene = sceneAliasAdd
		c.bot.saveChat(chat)
		return u.Bot.SendText(u.ChatID, c.bot.loc("settext_55"))
	case "remove":
		return c.bot.showAliasRemoveList(u, u.UserID)
	case "clear":
		c.bot.aliasRepo.Clear(u.UserID)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_56"), c.bot.replySettingsAliases())
	}
	return nil
}

type subsActionTextCmd struct {
	bot  *Bot
	kind string
}

func (c *subsActionTextCmd) Name() string        { return "/subs_action_text_" + c.kind }
func (c *subsActionTextCmd) Description() string { return "" }

func (c *subsActionTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "add_group":
		return text == c.bot.loc("settext_57")
	case "add_teacher":
		return text == c.bot.loc("settext_58")
	case "list":
		return text == c.bot.loc("settext_59")
	case "remove":
		return text == c.bot.loc("settext_60")
	case "test":
		return text == c.bot.loc("settext_61")
	}
	return false
}

func (c *subsActionTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "add_group":
		groups := c.bot.cache.GetGroups()
		if len(groups) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		count, _ := c.bot.chatRepo.CountSubscriptions(u.UserID)
		if count >= MAX_SUBSCRIPTIONS {
			return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("settext_62"), MAX_SUBSCRIPTIONS))
		}
		chat.Scene = sceneSubAddGroup
		c.bot.saveChat(chat)
		prompt := fmt.Sprintf(c.bot.loc("settext_63"), randomKey(groups))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "add_teacher":
		teachers := c.bot.cache.GetTeachers()
		if len(teachers) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		count, _ := c.bot.chatRepo.CountSubscriptions(u.UserID)
		if count >= MAX_SUBSCRIPTIONS {
			return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("settext_64"), MAX_SUBSCRIPTIONS))
		}
		chat.Scene = sceneSubAddTeacher
		c.bot.saveChat(chat)
		prompt := fmt.Sprintf(c.bot.loc("settext_65"), randomKey(teachers))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "list":
		list, _ := c.bot.chatRepo.GetSubscriptions(u.UserID)
		if len(list) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_66"), c.bot.replySubscriptionsMenu())
		}
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.formatSubscriptionsList(list), c.bot.replySubscriptionsMenu())
	case "remove":
		list, _ := c.bot.chatRepo.GetSubscriptions(u.UserID)
		if len(list) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_67"), c.bot.replySubscriptionsMenu())
		}
		chat.Scene = sceneSubRemove
		c.bot.saveChat(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("settext_68")+c.bot.formatSubscriptionsList(list), c.bot.replySubscriptionsMenu())
	case "test":
		return c.bot.subTestPrompt(u)
	}
	return nil
}

type setupModeTextCmd struct {
	bot  *Bot
	kind string
}

func (c *setupModeTextCmd) Name() string        { return "/setup_mode_text_" + c.kind }
func (c *setupModeTextCmd) Description() string { return "" }
func (c *setupModeTextCmd) Scene() string       { return sceneSetup }

func (c *setupModeTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "guest":
		return text == c.bot.loc("settext_69") || text == c.bot.loc("settext_70")
	case "student":
		return text == c.bot.loc("settext_71") || text == c.bot.loc("settext_72") || text == c.bot.loc("settext_73") || text == c.bot.loc("settext_74")
	case "parent":
		return text == c.bot.loc("settext_75") || text == c.bot.loc("settext_76")
	case "skip":
		return text == c.bot.loc("settext_77") || text == c.bot.loc("settext_78")
	}
	return false
}

func (c *setupModeTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	switch c.kind {
	case "guest":
		chat.Mode = ModeGuest
		chat.Scene = ""
		c.bot.saveChat(chat)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("about_bot"), replyMainMenu(c.bot, chat))
	case "student", "parent":
		groups := c.bot.cache.GetGroups()
		if len(groups) == 0 {
			return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
		}
		if c.kind == "student" {
			chat.Mode = ModeStudent
		} else {
			chat.Mode = ModeParent
		}
		chat.Scene = sceneSetGroup
		c.bot.saveChat(chat)
		prompt := fmt.Sprintf(c.bot.loc("settext_79"), c.bot.loc("setup_enter_group"), randomKey(groups))
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, c.bot.replyCancel())
	case "skip":
		chat.Scene = ""
		c.bot.saveChat(chat)
		return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("about_bot"), replyMainMenu(c.bot, chat))
	}
	return nil
}
