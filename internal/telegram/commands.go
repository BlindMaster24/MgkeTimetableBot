package telegram

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
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
func (c *startCmd) MatchText(text string) bool {
	return text == "Начать" || text == "Start" || text == "Меню" || text == "Главное меню"
}
func (c *startCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if chat.Mode == "" {
		chat.Scene = sceneSetup
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("setup_select_mode"), c.bot.replySelectMode())
	}

	return c.bot.showSchedule(u, chat)
}

func startButtonKeyboard() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard:       [][]telego.KeyboardButton{{{Text: "Начать"}}},
		ResizeKeyboard: true,
	}
}

func (b *Bot) showSchedule(u *Update, chat *Chat) error {
	groups := b.cache.GetGroups()
	teachers := b.cache.GetTeachers()

	if len(groups) == 0 && len(teachers) == 0 {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, b.loc("data_not_loaded"), replyMainMenu(b, chat))
	}

	var text string
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			text = b.locData("group_not_selected", map[string]interface{}{"Group": randomKey(groups)})
		} else {
			data, ok := groups[chat.Group]
			if !ok {
				text = b.loc("group_not_exists")
			} else {
				text = b.formatGroupFull(chat, chat.Group, data)
				if text == "" {
					text = b.loc("no_timetable")
				}
			}
		}
	case ModeTeacher:
		if chat.Teacher == "" {
			text = b.locData("teacher_not_selected", map[string]interface{}{"Teacher": randomKey(teachers)})
		} else {
			data, ok := teachers[chat.Teacher]
			if !ok {
				text = b.loc("nothing_found")
			} else {
				text = b.formatTeacherFull(chat, chat.Teacher, data)
				if text == "" {
					text = b.loc("no_timetable")
				}
			}
		}
	default:
		text = b.loc("main_menu")
	}

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, text, replyMainMenu(b, chat))
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
	for _, cmd := range c.bot.commandOrder {
		if ac, ok := cmd.(AdminCommand); ok && ac.AdminOnly() {
			continue
		}
		if hc, ok := cmd.(HiddenCommand); ok && hc.Hidden() {
			continue
		}
		text += fmt.Sprintf("\n%s - %s", cmd.Name(), cmd.Description())
	}
	return u.Bot.SendText(u.ChatID, text)
}

type cancelCmd struct{ bot *Bot }

func (c *cancelCmd) Name() string        { return "/cancel" }
func (c *cancelCmd) Description() string { return c.bot.loc("cmd_cancel") }
func (c *cancelCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_cancel") || text == "Отмена"
}
func (c *cancelCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = ""
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("input_cancelled"), replyMainMenu(c.bot, chat))
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
		chat.Scene = sceneSetup
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("setup_select_mode"), c.bot.replySelectMode())
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
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("setup_needed"), startButtonKeyboard())
	}
	return c.bot.showDaySchedule(u, chat)
}

func (b *Bot) showDaySchedule(u *Update, chat *Chat) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	if len(groups) == 0 && len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}

	var text string
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			text = b.locData("group_not_selected", map[string]interface{}{"Group": randomKey(groups)})
		} else {
			data, ok := groups[chat.Group]
			if !ok {
				text = b.loc("group_not_exists")
			} else {
				text = b.formatGroupDay(chat, data)
				if text == "" {
					text = b.loc("no_timetable")
				}
			}
		}
	case ModeTeacher:
		if chat.Teacher == "" {
			text = b.locData("teacher_not_selected", map[string]interface{}{"Teacher": randomKey(teachers)})
		} else {
			data, ok := teachers[chat.Teacher]
			if !ok {
				text = b.loc("nothing_found")
			} else {
				text = b.formatTeacherDay(chat, data)
				if text == "" {
					text = b.loc("no_timetable")
				}
			}
		}
	default:
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, b.loc("setup_needed"), startButtonKeyboard())
	}

	return b.sendOrEdit(u.ChatID, text, chat, nil)
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
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("setup_needed"), startButtonKeyboard())
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("about_bot"))
	}
	return c.bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("about_bot"), replyMainMenu(c.bot, chat))
}

type groupCmd struct{ bot *Bot }

func (c *groupCmd) Name() string        { return "/group" }
func (c *groupCmd) Description() string { return c.bot.loc("cmd_group") }
func (c *groupCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_group")
}
func (c *groupCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "day")
}

type teacherCmd struct{ bot *Bot }

func (c *teacherCmd) Name() string        { return "/teacher" }
func (c *teacherCmd) Description() string { return c.bot.loc("cmd_teacher") }
func (c *teacherCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_teacher")
}
func (c *teacherCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetTeacher(u, "day")
}

type imageCmd struct{ bot *Bot }

func (c *imageCmd) Hidden() bool { return true }

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

type buttonsReloadCmd struct{ bot *Bot }

func (c *buttonsReloadCmd) Name() string { return "/buttons_reload" }
func (c *buttonsReloadCmd) Description() string {
	return "Обновить клавиатуру бота"
}
func (c *buttonsReloadCmd) MatchText(text string) bool {
	return strings.EqualFold(text, "/buttons_reload") || strings.EqualFold(text, "/button_reload")
}
func (c *buttonsReloadCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Клавиатура обновлена", replyMainMenu(c.bot, chat))
}

type forceParseCmd struct{ bot *Bot }

func (c *forceParseCmd) AdminOnly() bool { return true }

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

func (c *resetCacheCmd) Hidden() bool { return true }

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

type flushCacheCmd struct{ bot *Bot }

func (c *flushCacheCmd) Hidden() bool { return true }

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

	if chat.Accepted && chat.NeedUpdateButtons {
		chat.NeedUpdateButtons = false
		chat.Scene = ""
		b.chatRepo.Save(chat)
		b.SendTextWithReplyKeyboard(u.ChatID, "Клавиатура была принудительно пересоздана (обновлена)", replyMainMenu(b, chat))
	}

	if b.dispatchTextCommand(ctx, u, chat) {
		return
	}

	if b.dispatchInputScene(ctx, u, chat) {
		return
	}

	chat.Scene = ""
	b.chatRepo.Save(chat)
	b.SendTextWithReplyKeyboard(u.ChatID, "Команда не найдена", replyMainMenu(b, chat))
}

func (b *Bot) dispatchTextCommand(ctx context.Context, u *Update, chat *Chat) bool {
	if cmd, ok := b.commands[u.Text]; ok {
		if sm, ok := cmd.(SceneMatcher); ok && sm.Scene() != "" && sm.Scene() != chat.Scene {
			return false
		}
		if err := cmd.Handler(ctx, u); err != nil {
			b.log.Error().Err(err).Str("cmd", u.Text).Msg("command error")
		}
		return true
	}

	if len(u.Text) > 1 && u.Text[0] == '/' {
		cmdName := u.Text[1:]
		if idx := strings.IndexByte(cmdName, ' '); idx >= 0 {
			cmdName = cmdName[:idx]
		}
		if cmd, ok := b.commands["/"+cmdName]; ok {
			if err := cmd.Handler(ctx, u); err != nil {
				b.log.Error().Err(err).Str("cmd", cmdName).Msg("command error")
			}
			return true
		}
	}

	for _, cmd := range b.commandOrder {
		if !matchesText(cmd, u.Text, chat) {
			continue
		}
		if err := cmd.Handler(ctx, u); err != nil {
			b.log.Error().Err(err).Msg("text match error")
		}
		return true
	}

	for _, cmd := range b.textCommands {
		if !matchesText(cmd, u.Text, chat) {
			continue
		}
		if err := cmd.Handler(ctx, u); err != nil {
			b.log.Error().Err(err).Msg("text match error")
		}
		return true
	}
	return false
}

func matchesText(cmd Command, text string, chat *Chat) bool {
	matcher, ok := cmd.(TextMatcher)
	if !ok || !matcher.MatchText(text) {
		return false
	}
	if scoped, ok := cmd.(SceneMatcher); ok && scoped.Scene() != "" && scoped.Scene() != chat.Scene {
		return false
	}
	return true
}

func findClosest(input string, candidates map[string]any) (string, bool) {
	for key := range candidates {
		if strings.EqualFold(key, input) {
			return key, true
		}
	}
	for key := range candidates {
		if strings.Contains(strings.ToLower(key), strings.ToLower(input)) {
			return key, true
		}
	}
	return "", false
}

func (b *Bot) handleSetGroup(ctx context.Context, u *Update, chat *Chat) {
	groups := b.GetRaspCache().GetGroups()
	if len(groups) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	matched, _ := findClosest(input, groups)

	if matched == "" {
		b.SendText(u.ChatID, b.loc("invalid_group_number"))
		return
	}

	chat.Group = matched
	chat.Mode = ModeStudent
	chat.Teacher = ""
	chat.Scene = ""
	b.chatRepo.Save(chat)
	b.SendTextWithReplyKeyboard(u.ChatID, b.loc("about_bot"), replyMainMenu(b, chat))
}

func (b *Bot) handleSetTeacher(ctx context.Context, u *Update, chat *Chat) {
	teachers := b.GetRaspCache().GetTeachers()
	if len(teachers) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	matched, _ := findClosest(input, teachers)

	if matched == "" {
		b.SendText(u.ChatID, b.loc("teacher_not_found"))
		return
	}

	chat.Teacher = matched
	chat.Mode = ModeTeacher
	chat.Group = ""
	chat.Scene = ""
	b.chatRepo.Save(chat)
	b.SendTextWithReplyKeyboard(u.ChatID, b.loc("about_bot"), replyMainMenu(b, chat))
}

type devCmd struct{ bot *Bot }

func (c *devCmd) Hidden() bool { return true }

func (c *devCmd) Name() string        { return "/dev" }
func (c *devCmd) Description() string { return "Исходный код бота" }
func (c *devCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "Привет! Хочешь помочь сделать бота лучше?\n\nhttps://github.com/BlindMaster24/MgkeTimetableBot")
}

type mathCmd struct{ bot *Bot }

func (c *mathCmd) Hidden() bool { return true }

func (c *mathCmd) Name() string        { return "/math" }
func (c *mathCmd) Description() string { return "Математический калькулятор" }
func (c *mathCmd) Handler(ctx context.Context, u *Update) error {
	text := strings.TrimPrefix(u.Text, "/math")
	text = strings.TrimSpace(text)
	if text == "" {
		return u.Bot.SendText(u.ChatID, "/math <пример>\n\nПримеры:\n/math 2+2\n/math (100+50)*2\n/math 1000/3")
	}

	allowed := "0123456789+-*/():. "
	for _, ch := range text {
		if !strings.ContainsRune(allowed, ch) {
			return u.Bot.SendText(u.ChatID, "В примере есть лишние символы.\n\nРазрешены: 0-9 + - * / ( ) : .")
		}
	}

	text = strings.ReplaceAll(text, "×", "*")
	text = strings.ReplaceAll(text, "÷", "/")
	text = strings.ReplaceAll(text, "к", "000")
	text = strings.ReplaceAll(text, "k", "000")
	text = strings.ReplaceAll(text, ",", ".")

	if strings.Count(text, "(") != strings.Count(text, ")") {
		return u.Bot.SendText(u.ChatID, "Неправильное количество скобок")
	}

	result, err := evalMath(text)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка: "+err.Error())
	}

	return u.Bot.SendText(u.ChatID, fmt.Sprintf("%g", result))
}

func evalMath(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	idx := 0

	var parse func(int) (float64, error)
	parse = func(minPrec int) (float64, error) {
		if idx >= len(expr) {
			return 0, fmt.Errorf("пустое выражение")
		}

		var left float64

		if expr[idx] == '(' {
			idx++
			v, err := parse(0)
			if err != nil {
				return 0, err
			}
			left = v
			if idx >= len(expr) || expr[idx] != ')' {
				return 0, fmt.Errorf("ожидалась закрывающая скобка")
			}
			idx++
		} else if (expr[idx] >= '0' && expr[idx] <= '9') || expr[idx] == '.' {
			start := idx
			for idx < len(expr) && ((expr[idx] >= '0' && expr[idx] <= '9') || expr[idx] == '.') {
				idx++
			}
			v, err := strconv.ParseFloat(expr[start:idx], 64)
			if err != nil {
				return 0, err
			}
			left = v
		} else if expr[idx] == '-' {
			idx++
			v, err := parse(3)
			if err != nil {
				return 0, err
			}
			left = -v
		} else {
			return 0, fmt.Errorf("неожиданный символ '%c'", expr[idx])
		}

		for idx < len(expr) {
			op := expr[idx]
			prec := 0
			switch op {
			case '+', '-':
				prec = 1
			case '*', '/':
				prec = 2
			case '^':
				prec = 3
			}
			if prec < minPrec {
				break
			}
			idx++
			right, err := parse(prec + 1)
			if err != nil {
				return 0, err
			}
			switch op {
			case '+':
				left += right
			case '-':
				left -= right
			case '*':
				left *= right
			case '/':
				if right == 0 {
					return 0, fmt.Errorf("деление на ноль")
				}
				left /= right
			case '^':
				left = math.Pow(left, right)
			}
		}
		return left, nil
	}

	return parse(0)
}

func (b *Bot) handleSubAddGroup(ctx context.Context, u *Update, chat *Chat) {
	groups := b.GetRaspCache().GetGroups()
	if len(groups) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	matched, _ := findClosest(input, groups)

	chat.Scene = ""
	b.chatRepo.Save(chat)

	if matched == "" {
		b.SendText(u.ChatID, b.loc("invalid_group_number"))
		return
	}

	added, _ := b.chatRepo.AddSubscription(u.UserID, "group", matched)
	if added {
		b.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Подписка на группу %s добавлена.", matched), b.replySubscriptionsMenu())
		return
	}
	b.SendTextWithReplyKeyboard(u.ChatID, "Такая подписка уже существует.", b.replySubscriptionsMenu())
}

func (b *Bot) handleSubAddTeacher(ctx context.Context, u *Update, chat *Chat) {
	teachers := b.GetRaspCache().GetTeachers()
	if len(teachers) == 0 {
		return
	}

	input := strings.TrimSpace(u.Text)
	matched, _ := findClosest(input, teachers)

	chat.Scene = ""
	b.chatRepo.Save(chat)

	if matched == "" {
		b.SendText(u.ChatID, b.loc("teacher_not_found"))
		return
	}

	added, _ := b.chatRepo.AddSubscription(u.UserID, "teacher", matched)
	if added {
		b.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Подписка на преподавателя %s добавлена.", matched), b.replySubscriptionsMenu())
		return
	}
	b.SendTextWithReplyKeyboard(u.ChatID, "Такая подписка уже существует.", b.replySubscriptionsMenu())
}

func (b *Bot) handleSubRemove(ctx context.Context, u *Update, chat *Chat) {
	input := strings.TrimSpace(u.Text)

	list, _ := b.chatRepo.GetSubscriptions(u.UserID)
	chat.Scene = ""
	b.chatRepo.Save(chat)

	if len(list) == 0 {
		b.SendText(u.ChatID, "Подписок нет.")
		return
	}

	var idx int
	for _, c := range input {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		}
	}

	if idx < 1 || idx > len(list) {
		b.SendTextWithReplyKeyboard(u.ChatID, "Неверный номер подписки.", b.replySubscriptionsMenu())
		return
	}

	target := list[idx-1]
	b.chatRepo.RemoveSubscription(u.UserID, target.ID)
	b.SendTextWithReplyKeyboard(u.ChatID, "Подписка удалена.", b.replySubscriptionsMenu())
}
