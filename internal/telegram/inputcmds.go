package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

var (
	groupDayButtonRe     = regexp.MustCompile(`(?i)^(👩‍🎓\s)?Группа(\s?День)?$`)
	groupWeekButtonRe    = regexp.MustCompile(`(?i)^(👩‍🎓\s)?Группа\s?Неделя$`)
	groupImageButtonRe   = regexp.MustCompile(`(?i)^(👩‍🎓\s)?Группа\s?(фото(графия)?|таблица)$`)
	teacherDayButtonRe   = regexp.MustCompile(`(?i)^(👩‍🏫\s)?(Учитель|Преподаватель|Препод\.?)(\s?День)?$`)
	teacherWeekButtonRe  = regexp.MustCompile(`(?i)^(👩‍🏫\s)?(Учитель|Преподаватель|Препод\.?)\s?Неделя$`)
	teacherImageButtonRe = regexp.MustCompile(`(?i)^(👩‍🏫\s)?(Учитель|Преподаватель)\s?(фотография|таблица)$`)
)

type getGroupWeekCmd struct{ bot *Bot }

func (c *getGroupWeekCmd) Name() string { return "/groupweek" }
func (c *getGroupWeekCmd) Description() string {
	return "Узнать расписание на неделю указанной группы (не зависит от текущего вашего)"
}
func (c *getGroupWeekCmd) MatchText(text string) bool {
	return groupWeekButtonRe.MatchString(text)
}
func (c *getGroupWeekCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "week")
}

type getGroupImageCmd struct{ bot *Bot }

func (c *getGroupImageCmd) Name() string { return "/groupimage" }
func (c *getGroupImageCmd) Description() string {
	return "Сгенерировать фотографию расписания группы (не зависит от текущего вашего)"
}
func (c *getGroupImageCmd) MatchText(text string) bool {
	return groupImageButtonRe.MatchString(text)
}
func (c *getGroupImageCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetGroup(u, "image")
}

type getTeacherWeekCmd struct{ bot *Bot }

func (c *getTeacherWeekCmd) Name() string { return "/teacherweek" }
func (c *getTeacherWeekCmd) Description() string {
	return "Узнать расписание на неделю указанного преподавателя (не зависит от текущего вашего)"
}
func (c *getTeacherWeekCmd) MatchText(text string) bool {
	return teacherWeekButtonRe.MatchString(text)
}
func (c *getTeacherWeekCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.startGetTeacher(u, "week")
}

type getTeacherImageCmd struct{ bot *Bot }

func (c *getTeacherImageCmd) Name() string { return "/teacherimage" }
func (c *getTeacherImageCmd) Description() string {
	return "Сгенерировать фотографию расписания преподавателя (не зависит от текущего вашего)"
}
func (c *getTeacherImageCmd) MatchText(text string) bool {
	return teacherImageButtonRe.MatchString(text)
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

	chat.Scene = sceneGetGroup + ":" + kind
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

	chat.Scene = sceneGetTeacher + ":" + kind
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

	invalid := normalized == ""
	if !invalid {
		for _, ch := range normalized {
			if ch < '0' || ch > '9' {
				invalid = true
				break
			}
		}
	}

	message := "Это не число"
	if !invalid && len(normalized) > 3 {
		invalid = true
		message = "Номер группы введён неверно"
	}
	if invalid && kind == "set" {
		message = fmt.Sprintf("Неправильный синтаксис команды\n\nПример:\n/setGroup %s", randomKey(groups))
	}
	if invalid {
		if normalized == "" {
			return u.Bot.SendText(u.ChatID, message)
		}
		return b.sendGroupError(u, chat, message)
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
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Группа этого чата была успешно изменена на '%s'", normalized), replyMainMenu(b, chat))
	}

	return b.sendGroupResult(u, chat, normalized, kind)
}

func (b *Bot) sendGroupError(u *Update, chat *Chat, msg string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, msg, replyMainMenu(b, chat))
}

func (b *Bot) sendGroupResult(u *Update, chat *Chat, group, kind string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)

	data := b.cache.GetGroups()[group]

	switch kind {
	case "week":
		week := b.relevantWeekIndex()
		return b.showWeekDays(u, chat, "group", group, &week)
	case "image":
		_, days := b.relevantWeekDays(data)
		path, err := imagepkg.RenderGroupDays(group, days, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	default:
		text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(group, b.getDayRasp(extractDays(data), true, 2), b.fmtOpts(chat, true))
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, getWeekTimetableKeyboard("group", group))
	}
}

func (b *Bot) resolveTeacherInput(u *Update, chat *Chat, input, kind string) error {
	teachers := b.cache.GetTeachers()

	if len(input) < 3 {
		message := "Фамилия введена некорректно"
		if kind == "set" {
			message = fmt.Sprintf("Неправильный синтаксис команды\n\nПример:\n/setTeacher %s", randomKey(teachers))
		}
		return b.sendTeacherError(u, chat, message)
	}

	matched, tooMany := matchTeacherList(input, teachers, b.cache.GetTeamNames())
	if len(matched) == 0 {
		return b.sendTeacherError(u, chat, "Данный преподаватель не найден")
	}
	if tooMany {
		return b.sendTeacherError(u, chat, "Слишком много результатов для выборки.")
	}
	if len(matched) > 1 {
		chat.Scene = sceneGetTeacher + ":" + kind
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
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf("Преподвателя этого чата был успешно изменен на '%s'", teacher), replyMainMenu(b, chat))
	}

	return b.sendTeacherResult(u, chat, teacher, kind)
}

func (b *Bot) sendTeacherError(u *Update, chat *Chat, msg string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, msg, replyMainMenu(b, chat))
}

func (b *Bot) sendTeacherResult(u *Update, chat *Chat, teacher, kind string) error {
	chat.Scene = ""
	b.chatRepo.Save(chat)

	data := b.cache.GetTeachers()[teacher]

	switch kind {
	case "week":
		chat.Scene = sceneGetTeacherWeek + ":" + teacher
		b.chatRepo.Save(chat)
		return u.Bot.SendTextWithKeyboard(u.ChatID, b.loc("history_enter_week"), withCancelButton(teacherHistoryKeyboard(chat)))
	case "image":
		_, days := b.relevantWeekDays(data)
		path, err := imagepkg.RenderTeacherDays(teacher, days, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	default:
		text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(teacher, b.getDayRasp(extractDays(data), true, 2), b.fmtOpts(chat, true))
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, getWeekTimetableKeyboard("teacher", teacher))
	}
}

type teacherWeekScene struct{ bot *Bot }

func (s *teacherWeekScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	teacher := strings.TrimPrefix(chat.Scene, sceneGetTeacherWeek+":")

	weekIndex := s.bot.parseWeekIndex(u.Text)
	if weekIndex == nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_invalid_week"))
	}

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	return s.bot.showWeekDays(u, chat, "teacher", teacher, weekIndex)
}

type setGroupCmd struct{ bot *Bot }

func (c *setGroupCmd) Hidden() bool { return true }

func (c *setGroupCmd) Name() string { return "/setgroup" }
func (c *setGroupCmd) Description() string {
	return "Изменить группу этого чата"
}
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

func (c *setTeacherCmd) Hidden() bool { return true }

func (c *setTeacherCmd) Name() string { return "/setteacher" }
func (c *setTeacherCmd) Description() string {
	return "Изменить преподавателя этого чата"
}
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

func matchTeacherList(input string, candidates map[string]any, fullNames map[string]string) ([]string, bool) {
	const matchLimit = 5
	var matched []string
	search := strings.ToLower(strings.ReplaceAll(input, ".", ""))

	for _, key := range sortedKeys(candidates) {
		needle := strings.ToLower(strings.ReplaceAll(key, ".", ""))
		if !strings.Contains(needle, search) {
			continue
		}
		matched = append(matched, key)
		if needle == search || len(matched) > matchLimit {
			break
		}
	}

	for _, key := range sortedKeys(fullNames) {
		if len(matched) > matchLimit {
			break
		}
		if containsString(matched, key) {
			continue
		}
		if !strings.Contains(strings.ToLower(fullNames[key]), strings.ToLower(input)) {
			continue
		}
		matched = append(matched, key)
	}

	return matched, len(matched) > matchLimit
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (b *Bot) getFormatter(chat *Chat) formatter.Formatter {
	return formatter.GetByIndex(chat.Formatter)
}

func (b *Bot) relevantWeekDays(data any) (utils.WeekIndex, []map[string]any) {
	week := b.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()
	days := extractDaysFromRange(data, minIdx, maxIdx)
	return week, days
}
