package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/mymmrac/telego"
)

func (b *Bot) loc(key string) string {
	return b.i18n.T("ru", key, nil)
}

func (b *Bot) locData(key string, data map[string]interface{}) string {
	return b.i18n.T("ru", key, data)
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
		text := b.formatGroupFull(chat, chat.Group, data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, b.mainMenuKeyboard(chat))

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.locData("teacher_not_selected", map[string]interface{}{"Teacher": randomKey(teachers)}))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}
		text := b.formatTeacherFull(chat, chat.Teacher, data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, b.mainMenuKeyboard(chat))
	}

	return u.Bot.SendTextWithKeyboard(u.ChatID, b.loc("main_menu"), b.mainMenuKeyboard(chat))
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
		text := b.formatGroupDay(chat, data)
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
		text := b.formatTeacherDay(chat, data)
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
	return b.showWeekScheduleWithKeyboard(u, chat, "", "")
}

type callsCmd struct{ bot *Bot }

func (c *callsCmd) Name() string        { return "/calls" }
func (c *callsCmd) Description() string { return c.bot.loc("cmd_calls") }
func (c *callsCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_calls")
}
func (c *callsCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	c.bot.showCallsFull(u, chat)
	return nil
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.loc("settings_menu"), c.bot.settingsKeyboardFull(chat))
}

type imageCmd struct{ bot *Bot }

func (c *imageCmd) Name() string        { return "/image" }
func (c *imageCmd) Description() string { return c.bot.loc("cmd_image") }
func (c *imageCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_image")
}
func (c *imageCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if chat.Mode == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("setup_needed"))
	}

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
		}
		data, ok := c.bot.cache.GetGroups()[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, c.bot.loc("group_not_exists"))
		}
		path, err := imagepkg.RenderGroupFromCache(chat.Group, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, c.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
		}
		data, ok := c.bot.cache.GetTeachers()[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, c.bot.loc("teacher_not_exists"))
		}
		path, err := imagepkg.RenderTeacherFromCache(chat.Teacher, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, c.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}

	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}

type buttonsCmd struct{ bot *Bot }

func (c *buttonsCmd) Name() string        { return "/buttons" }
func (c *buttonsCmd) Description() string { return "Настройка кнопок" }
func (c *buttonsCmd) MatchText(text string) bool {
	return text == "⌨️ Кнопки"
}
func (c *buttonsCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Настройка кнопок", c.bot.buttonsKeyboard(chat))
}

type formatterCmd struct{ bot *Bot }

func (c *formatterCmd) Name() string        { return "/formatter" }
func (c *formatterCmd) Description() string { return "Выбор формата расписания" }
func (c *formatterCmd) MatchText(text string) bool {
	return text == "📃 Форматировщик"
}
func (c *formatterCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Выберите формат расписания:", c.bot.formatterKeyboard(chat))
}

type forceParseCmd struct{ bot *Bot }

func (c *forceParseCmd) Name() string        { return "/forceparse" }
func (c *forceParseCmd) Description() string { return c.bot.loc("cmd_forceparse") }
func (c *forceParseCmd) Handler(ctx context.Context, u *Update) error {
	if c.bot.parseFunc == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("parse_not_available"))
	}
	if err := u.Bot.SendText(u.ChatID, c.bot.loc("force_parse_started")); err != nil {
		return err
	}
	go func() {
		if err := c.bot.parseFunc(); err != nil {
			c.bot.log.Error().Err(err).Msg("force parse error")
			c.bot.SendText(u.ChatID, c.bot.loc("force_parse_error"))
			return
		}
		c.bot.SendText(u.ChatID, c.bot.loc("force_parse_done"))
	}()
	return nil
}

type resetCacheCmd struct{ bot *Bot }

func (c *resetCacheCmd) Name() string        { return "/resetcache" }
func (c *resetCacheCmd) Description() string { return c.bot.loc("cmd_resetcache") }
func (c *resetCacheCmd) Handler(ctx context.Context, u *Update) error {
	c.bot.cache.Reset()
	if err := c.bot.cache.Save(); err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("reset_cache_error"))
	}
	return u.Bot.SendText(u.ChatID, c.bot.loc("reset_cache_done"))
}

type eulaCmd struct{ bot *Bot }

func (c *eulaCmd) Name() string        { return "/eula" }
func (c *eulaCmd) Description() string { return "Лицензионное соглашение" }
func (c *eulaCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("eula_text"))
}

type apiCmd struct{ bot *Bot }

func (c *apiCmd) Name() string        { return "/api" }
func (c *apiCmd) Description() string { return "Просмотр API ключа" }
func (c *apiCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, c.bot.loc("api_info"))
}

type diffCmd struct{ bot *Bot }

func (c *diffCmd) Name() string        { return "/diff" }
func (c *diffCmd) Description() string { return "Настройки diff" }
func (c *diffCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.showDiffSettings(u, chat)
}

type flushCacheCmd struct{ bot *Bot }

func (c *flushCacheCmd) Name() string        { return "/flushcache" }
func (c *flushCacheCmd) Description() string { return "Сбросить кеш в БД" }
func (c *flushCacheCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if err := c.bot.cache.Save(); err != nil {
		return u.Bot.SendText(u.ChatID, "❌ Ошибка: "+err.Error())
	}
	return u.Bot.SendText(u.ChatID, "✅ Кеш сброшен в БД")
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

	var matched string
	for key := range groups {
		if strings.EqualFold(key, input) {
			matched = key
			break
		}
	}
	if matched == "" {
		for key := range groups {
			if strings.Contains(strings.ToLower(key), strings.ToLower(input)) {
				matched = key
				break
			}
		}
	}

	if matched == "" {
		b.SendText(u.ChatID, b.loc("invalid_group_number"))
		return
	}

	chat.Group = matched
	chat.Mode = ModeStudent
	chat.Scene = ""
	b.chatRepo.Save(chat)
	b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": matched}))

	data, ok := groups[matched]
	if ok {
		text := b.formatGroupFull(chat, matched, data)
		if text != "" {
			b.SendTextWithKeyboard(u.ChatID, text, b.mainMenuKeyboard(chat))
		}
	}
}

func (b *Bot) handleSetTeacher(ctx context.Context, u *Update, chat *Chat) {
	teachers := b.GetRaspCache().GetTeachers()
	if len(teachers) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)

	var matched string
	for key := range teachers {
		if strings.EqualFold(key, input) {
			matched = key
			break
		}
	}
	if matched == "" {
		for key := range teachers {
			if strings.Contains(strings.ToLower(key), strings.ToLower(input)) {
				matched = key
				break
			}
		}
	}

	if matched == "" {
		b.SendText(u.ChatID, b.loc("teacher_not_found"))
		return
	}

	chat.Teacher = matched
	chat.Mode = ModeTeacher
	chat.Scene = ""
	b.chatRepo.Save(chat)
	b.SendText(u.ChatID, b.locData("keyboard_select", map[string]interface{}{"Value": matched}))

	data, ok := teachers[matched]
	if ok {
		text := b.formatTeacherFull(chat, matched, data)
		if text != "" {
			b.SendTextWithKeyboard(u.ChatID, text, b.mainMenuKeyboard(chat))
		}
	}
}

func (b *Bot) mainMenuKeyboard(chat *Chat) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton

	if chat.Mode == "" {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_setup"), CallbackData: "setup"},
		})
	} else if chat.Mode == ModeGuest {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_group"), CallbackData: "group"},
			{Text: b.loc("button_teacher"), CallbackData: "teacher"},
		})
	} else {
		canShow := (chat.Mode == ModeStudent || chat.Mode == ModeParent) && chat.Group != "" ||
			chat.Mode == ModeTeacher && chat.Teacher != ""

		if canShow {
			var row []telego.InlineKeyboardButton
			if chat.ShowDaily {
				row = append(row, telego.InlineKeyboardButton{Text: b.loc("button_day"), CallbackData: "day"})
			}
			if chat.ShowWeekly {
				row = append(row, telego.InlineKeyboardButton{Text: b.loc("button_week"), CallbackData: "week"})
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}

	if chat.Mode != "" && chat.ShowFastGroup {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_group"), CallbackData: "group"},
		})
	}

	if chat.ShowCalls {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_calls"), CallbackData: "calls"},
		})
	}

	if chat.Mode != "" && chat.ShowFastTeacher {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_teacher"), CallbackData: "teacher"},
		})
	}

	var bottomRow []telego.InlineKeyboardButton
	if chat.ShowAbout {
		bottomRow = append(bottomRow, telego.InlineKeyboardButton{Text: b.loc("button_about"), CallbackData: "about"})
	}
	bottomRow = append(bottomRow, telego.InlineKeyboardButton{Text: b.loc("button_settings"), CallbackData: "settings"})
	rows = append(rows, bottomRow)

	if len(rows) == 0 {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: b.loc("button_settings"), CallbackData: "settings"},
		})
	}

	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}
