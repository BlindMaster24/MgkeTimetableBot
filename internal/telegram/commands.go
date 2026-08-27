package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"


)

func (b *Bot) loc(key string) string {
	return b.i18n.T("ru", key, nil)
}

func (b *Bot) locData(key string, data map[string]interface{}) string {
	return b.i18n.T("ru", key, data)
}

func getDayName(dateStr string) string {
	t, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return ""
	}
	names := []string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}
	return names[t.Weekday()]
}

type startCmd struct{ bot *Bot }

func (c *startCmd) Name() string        { return "/start" }
func (c *startCmd) Description() string { return c.bot.loc("cmd_start") }
func (c *startCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if chat.Mode == "" {
		chat.Scene = "setup"
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("setup_select_mode"), selectModeKeyboard(c.bot.i18n.T))
	}

	return c.bot.showSchedule(u, chat)
}

func (b *Bot) showSchedule(u *Update, chat *Chat) error {
	groups := b.cache.GetGroups()
	teachers := b.cache.GetTeachers()

	if len(groups) == 0 && len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.locData("group_not_selected", map[string]interface{}{"Group": randomKey(groups)}))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}
		text := formatGroupDay(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, mainMenuKeyboard(b.i18n.T))

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.locData("teacher_not_selected", map[string]interface{}{"Teacher": randomKey(teachers)}))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}
		text := formatTeacherDay(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, mainMenuKeyboard(b.i18n.T))
	}

	return u.Bot.SendTextWithKeyboard(u.ChatID, b.loc("main_menu"), mainMenuKeyboard(b.i18n.T))
}

func randomKey(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	return keys[rand.Intn(len(keys))]
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = ""
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendText(u.ChatID, c.bot.loc("input_cancelled"))
}

type setupCmd struct{ bot *Bot }

func (c *setupCmd) Name() string        { return "/setup" }
func (c *setupCmd) Description() string { return c.bot.loc("cmd_setup") }
func (c *setupCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_setup")
}
func (c *setupCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "setup"
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("setup_select_mode"), selectModeKeyboard(c.bot.i18n.T))
}

type dayCmd struct{ bot *Bot }

func (c *dayCmd) Name() string        { return "/day" }
func (c *dayCmd) Description() string { return c.bot.loc("cmd_day") }
func (c *dayCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_day")
}
func (c *dayCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if chat.Mode == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("setup_needed"))
	}
	return c.bot.showDaySchedule(u, chat)
}

func (b *Bot) showDaySchedule(u *Update, chat *Chat) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	if len(groups) == 0 && len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_group"))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}
		text := formatGroupDay(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendText(u.ChatID, text)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_teacher"))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}
		text := formatTeacherDay(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendText(u.ChatID, text)
	}

	return u.Bot.SendText(u.ChatID, b.loc("need_group"))
}

type weekCmd struct{ bot *Bot }

func (c *weekCmd) Name() string        { return "/week" }
func (c *weekCmd) Description() string { return c.bot.loc("cmd_week") }
func (c *weekCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_week")
}
func (c *weekCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if chat.Mode == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("setup_needed"))
	}
	return c.bot.showWeekSchedule(u, chat)
}

func (b *Bot) showWeekSchedule(u *Update, chat *Chat) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_group"))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}
		text := formatGroupWeek(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendText(u.ChatID, text)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_teacher"))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}
		text := formatTeacherWeek(data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendText(u.ChatID, text)
	}

	return u.Bot.SendText(u.ChatID, b.loc("need_group"))
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "set_group"
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendText(u.ChatID, c.bot.loc("enter_group_number"))
}

type teacherCmd struct{ bot *Bot }

func (c *teacherCmd) Name() string        { return "/teacher" }
func (c *teacherCmd) Description() string { return c.bot.loc("cmd_teacher") }
func (c *teacherCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_teacher")
}
func (c *teacherCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "set_teacher"
		c.bot.chatRepo.Save(chat)
	}
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil && chat.Mode == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("setup_needed"))
	}
	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}

func (b *Bot) handleMessageText(ctx context.Context, u *Update) {
	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return
	}

	switch chat.Scene {
	case "set_group":
		b.handleSetGroup(ctx, u, chat)
		return
	case "set_teacher":
		b.handleSetTeacher(ctx, u, chat)
		return
	}

	for _, cmd := range b.commands {
		if tm, ok := cmd.(TextMatcher); ok {
			if tm.MatchText(u.Text) {
				if err := cmd.Handler(ctx, u); err != nil {
					b.log.Error().Err(err).Msg("text match error")
				}
				return
			}
		}
	}
}

func (b *Bot) handleSetGroup(ctx context.Context, u *Update, chat *Chat) {
	groups := b.GetRaspCache().GetGroups()
	if len(groups) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	for key := range groups {
		if strings.EqualFold(key, input) {
			chat.Group = key
			chat.Mode = ModeStudent
			chat.Scene = ""
			b.chatRepo.Save(chat)
			b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": key}))
			return
		}
	}

	for key := range groups {
		if strings.Contains(strings.ToLower(key), strings.ToLower(input)) {
			chat.Group = key
			chat.Mode = ModeStudent
			chat.Scene = ""
			b.chatRepo.Save(chat)
			b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": key}))
			return
		}
	}

	b.SendText(u.ChatID, b.loc("invalid_group_number"))
}

func (b *Bot) handleSetTeacher(ctx context.Context, u *Update, chat *Chat) {
	teachers := b.GetRaspCache().GetTeachers()
	if len(teachers) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	for key := range teachers {
		if strings.EqualFold(key, input) {
			chat.Teacher = key
			chat.Mode = ModeTeacher
			chat.Scene = ""
			b.chatRepo.Save(chat)
			b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": key}))
			return
		}
	}

	for key := range teachers {
		if strings.Contains(strings.ToLower(key), strings.ToLower(input)) {
			chat.Teacher = key
			chat.Mode = ModeTeacher
			chat.Scene = ""
			b.chatRepo.Save(chat)
			b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": key}))
			return
		}
	}

	b.SendText(u.ChatID, b.loc("teacher_not_found"))
}
