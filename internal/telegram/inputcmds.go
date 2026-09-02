package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type getGroupCmd struct{ bot *Bot }

func (c *getGroupCmd) Name() string        { return "/group" }
func (c *getGroupCmd) Description() string { return c.bot.loc("cmd_group") }
func (c *getGroupCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_group") || text == "Группа День" || text == "👩‍🎓 Группа День"
}
func (c *getGroupCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "day")
}

type getGroupWeekCmd struct{ bot *Bot }

func (c *getGroupWeekCmd) Name() string        { return "/groupweek" }
func (c *getGroupWeekCmd) Description() string { return "Узнать расписание на неделю указанной группы (не зависит от текущего вашего)" }
func (c *getGroupWeekCmd) MatchText(text string) bool {
	return text == "Группа Неделя" || text == "👩‍🎓 Группа Неделя"
}
func (c *getGroupWeekCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "week")
}

type getGroupImageCmd struct{ bot *Bot }

func (c *getGroupImageCmd) Name() string        { return "/groupimage" }
func (c *getGroupImageCmd) Description() string { return "Сгенерировать фотографию расписания группы (не зависит от текущего вашего)" }
func (c *getGroupImageCmd) MatchText(text string) bool {
	return strings.HasPrefix(text, "Группафото") || strings.HasPrefix(text, "Группа таблица")
}
func (c *getGroupImageCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "image")
}

type getTeacherCmd struct{ bot *Bot }

func (c *getTeacherCmd) Name() string        { return "/teacher" }
func (c *getTeacherCmd) Description() string { return c.bot.loc("cmd_teacher") }
func (c *getTeacherCmd) MatchText(text string) bool {
	return text == c.bot.loc("button_teacher") || text == "Преподаватель День" || text == "Учитель День"
}
func (c *getTeacherCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetTeacher(u, "day")
}

type getTeacherWeekCmd struct{ bot *Bot }

func (c *getTeacherWeekCmd) Name() string        { return "/teacherweek" }
func (c *getTeacherWeekCmd) Description() string { return "Узнать расписание на неделю указанного преподавателя (не зависит от текущего вашего)" }
func (c *getTeacherWeekCmd) MatchText(text string) bool {
	return text == "Преподаватель Неделя" || text == "Учитель Неделя"
}
func (c *getTeacherWeekCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetTeacher(u, "week")
}

type getTeacherImageCmd struct{ bot *Bot }

func (c *getTeacherImageCmd) Name() string        { return "/teacherimage" }
func (c *getTeacherImageCmd) Description() string { return "Сгенерировать фотографию расписания преподавателя (не зависит от текущего вашего)" }
func (c *getTeacherImageCmd) MatchText(text string) bool {
	return strings.HasPrefix(text, "Преподавательфотография") || strings.HasPrefix(text, "Учительфотография") || strings.HasPrefix(text, "Преподаватель таблица")
}
func (c *getTeacherImageCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetTeacher(u, "image")
}

func (b *Bot) startGetGroup(u *Update, kind string) error {
	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}
	groups := b.cache.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}

	if arg := extractCommandArg(u.Text); arg != "" {
		return b.resolveGroupInput(u, chat, arg, kind)
	}

	chat.Scene = "get_group:" + kind
	b.chatRepo.Save(chat)

	prompt := fmt.Sprintf("%s (например, %s)", b.loc("enter_group_number"), randomKey(groups))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(groupHistoryKeyboard(chat)))
}

func (b *Bot) startGetTeacher(u *Update, kind string) error {
	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}
	teachers := b.cache.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}

	if arg := extractCommandArg(u.Text); arg != "" {
		return b.resolveTeacherInput(u, chat, arg, kind)
	}

	chat.Scene = "get_teacher:" + kind
	b.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите фамилию преподавателя или выберите из списка ниже (например, %s)", randomKey(teachers))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(teacherHistoryKeyboard(chat)))
}

func extractCommandArg(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (b *Bot) resolveGroupInput(u *Update, chat *Chat, input, kind string) error {
	groups := b.cache.GetGroups()

	normalized := strings.TrimRight(strings.TrimSpace(input), "*")
	if normalized == "" {
		return u.Bot.SendText(u.ChatID, "Это не число")
	}
	for _, ch := range normalized {
		if ch < '0' || ch > '9' {
			return b.sendGroupError(u, chat, "Это не число")
		}
	}
	if len(normalized) > 3 {
		return b.sendGroupError(u, chat, "Номер группы введён неверно")
	}
	if _, ok := groups[normalized]; !ok {
		return b.sendGroupError(u, chat, "Данной учебной группы не существует")
	}

	chat.AppendGroupHistory(normalized)

	if kind == "set" {
		chat.Group = normalized
		chat.Mode = ModeStudent
		chat.Teacher = ""
		chat.Scene = ""
		b.chatRepo.Save(chat)
		return u.Bot.SendTextWithKeyboard(u.ChatID, fmt.Sprintf("Группа этого чата была успешно изменена на '%s'", normalized), b.mainMenuKeyboard(chat))
	}

	return b.sendGroupResult(u, chat, normalized, kind)
}

func (b *Bot) sendGroupError(u *Update, chat *Chat, msg string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)
	return u.Bot.SendTextWithKeyboard(u.ChatID, msg, b.mainMenuKeyboard(chat))
}

func (b *Bot) sendGroupResult(u *Update, chat *Chat, group, kind string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)

	data := b.cache.GetGroups()[group]

	switch kind {
	case "week":
		return b.sendGroupWeek(u, chat, group, data)
	case "image":
		path, err := imagepkg.RenderGroupFromCache(group, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	default:
		text := b.formatGroupFull(chat, group, data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, getWeekTimetableKeyboard("group", group))
	}
}

func (b *Bot) sendGroupWeek(u *Update, chat *Chat, group string, data any) error {
	week, days := b.relevantWeekDays(data)
	if chat.HidePastDays {
		days = removePastDays(days)
	}

	opts := b.fmtOpts(chat, true)
	opts.WeekLabel = buildWeekLabelFromWeek(week)
	text := b.getFormatter(chat).FormatGroupFull(group, days, opts)
	if text == "" {
		return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
	}

	kb := b.weekControlKeyboard("group", group, week.Value(), false)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

func (b *Bot) resolveTeacherInput(u *Update, chat *Chat, input, kind string) error {
	teachers := b.cache.GetTeachers()

	if len(input) < 3 {
		return b.sendTeacherError(u, chat, "Фамилия введена некорректно")
	}

	matched, tooMany := matchTeacherList(input, teachers)
	if len(matched) == 0 {
		return b.sendTeacherError(u, chat, "Данный преподаватель не найден")
	}
	if tooMany {
		return b.sendTeacherError(u, chat, "Слишком много результатов для выборки.")
	}
	if len(matched) > 1 {
		chat.Scene = "get_teacher:" + kind
		b.chatRepo.Save(chat)
		msg := "Найдено несколько преподавателей.\nКакой именно нужен?\n\n" + strings.Join(matched, "\n")
		return u.Bot.SendTextWithKeyboard(u.ChatID, msg, withCancelButton(verticalValuesKeyboard(matched)))
	}

	teacher := matched[0]
	chat.AppendTeacherHistory(teacher)

	if kind == "set" {
		chat.Teacher = teacher
		chat.Mode = ModeTeacher
		chat.Group = ""
		chat.Scene = ""
		b.chatRepo.Save(chat)
		return u.Bot.SendTextWithKeyboard(u.ChatID, fmt.Sprintf("Преподвателя этого чата был успешно изменен на '%s'", teacher), b.mainMenuKeyboard(chat))
	}

	return b.sendTeacherResult(u, chat, teacher, kind)
}

func (b *Bot) sendTeacherError(u *Update, chat *Chat, msg string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)
	return u.Bot.SendTextWithKeyboard(u.ChatID, msg, b.mainMenuKeyboard(chat))
}

func (b *Bot) sendTeacherResult(u *Update, chat *Chat, teacher, kind string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)

	data := b.cache.GetTeachers()[teacher]

	switch kind {
	case "week":
		return b.sendTeacherWeek(u, chat, teacher, data)
	case "image":
		path, err := imagepkg.RenderTeacherFromCache(teacher, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	default:
		text := b.formatTeacherFull(chat, teacher, data)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, getWeekTimetableKeyboard("teacher", teacher))
	}
}

func (b *Bot) sendTeacherWeek(u *Update, chat *Chat, teacher string, data any) error {
	week, days := b.relevantWeekDays(data)
	if chat.HidePastDays {
		days = removePastDays(days)
	}

	opts := b.fmtOpts(chat, true)
	opts.WeekLabel = buildWeekLabelFromWeek(week)
	text := b.getFormatter(chat).FormatTeacherFull(teacher, days, opts)
	if text == "" {
		return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
	}

	kb := b.weekControlKeyboard("teacher", teacher, week.Value(), false)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

type setGroupCmd struct{ bot *Bot }

func (c *setGroupCmd) Name() string        { return "/setgroup" }
func (c *setGroupCmd) Description() string { return "Изменить группу этого чата" }
func (c *setGroupCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/setgroup")
}
func (c *setGroupCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.resolveGroupInput(u, chat, extractCommandArg(u.Text), "set")
}

type setTeacherCmd struct{ bot *Bot }

func (c *setTeacherCmd) Name() string        { return "/setteacher" }
func (c *setTeacherCmd) Description() string { return "Изменить преподавателя этого чата" }
func (c *setTeacherCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/setteacher")
}
func (c *setTeacherCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	return c.bot.resolveTeacherInput(u, chat, extractCommandArg(u.Text), "set")
}

func matchTeacherList(input string, candidates map[string]any) ([]string, bool) {
	const matchLimit = 5
	var matched []string
	search := strings.ToLower(strings.ReplaceAll(input, ".", ""))

	for key := range candidates {
		needle := strings.ToLower(strings.ReplaceAll(key, ".", ""))
		if !strings.Contains(needle, search) {
			continue
		}
		if needle == search {
			return []string{key}, false
		}
		matched = append(matched, key)
		if len(matched) > matchLimit {
			return matched, true
		}
	}
	return matched, false
}

func (b *Bot) getFormatter(chat *Chat) formatter.Formatter {
	return formatter.GetByIndex(chat.Formatter)
}

func (b *Bot) relevantWeekDays(data any) (utils.WeekIndex, []map[string]any) {
	week := utils.WeekIndexFromDate(time.Now())
	minIdx, maxIdx := week.WeekDayIndexRange()
	days := extractDaysFromRange(data, minIdx, maxIdx)
	return week, days
}
