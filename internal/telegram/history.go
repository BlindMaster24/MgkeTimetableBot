package telegram

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type historyCmd struct{ bot *Bot }

func (c *historyCmd) Name() string        { return "/history" }
func (c *historyCmd) Description() string { return c.bot.loc("cmd_history") }
func (c *historyCmd) MatchText(text string) bool {
	return text == "📚 История"
}
func (c *historyCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	teachers := c.bot.cache.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	c.bot.chatRepo.SetScene(chat, "history_teacher")
	c.bot.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите фамилию преподавателя или выберите из списка ниже (например, %s)", randomKey(teachers))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(teacherHistoryKeyboard(chat)))
}

type historyTeacherScene struct{ bot *Bot }

func (s *historyTeacherScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	teachers := s.bot.cache.GetTeachers()
	input := strings.TrimSpace(u.Text)

	if len(input) < 3 {
		return u.Bot.SendText(u.ChatID, "Фамилия введена некорректно")
	}

	matched, tooMany := matchTeacherList(input, teachers, s.bot.cache.GetTeamNames())
	if len(matched) == 0 {
		return u.Bot.SendText(u.ChatID, "Данный преподаватель не найден")
	}
	if tooMany {
		return u.Bot.SendText(u.ChatID, "Слишком много результатов для выборки.")
	}
	if len(matched) > 1 {
		msg := "Найдено несколько преподавателей.\nКакой именно нужен?\n\n" + strings.Join(matched, "\n")
		return u.Bot.SendTextWithKeyboard(u.ChatID, msg, withCancelButton(verticalValuesKeyboard(matched)))
	}

	teacher := matched[0]
	chat.AppendTeacherHistory(teacher)
	chat.Teacher = teacher
	chat.Mode = ModeTeacher
	chat.Scene = sceneHistoryWeek
	s.bot.chatRepo.Save(chat)

	prompt := "Введите номер учебной недели или дату (дд.мм или дд.мм.гггг)"
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(teacherHistoryKeyboard(chat)))
}

type historyWeekScene struct{ bot *Bot }

func (s *historyWeekScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	weekIndex := s.bot.parseWeekIndex(u.Text)
	if weekIndex == nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_invalid_week"))
	}

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	return s.bot.showWeekDays(u, chat, "teacher", chat.Teacher, weekIndex)
}

func (b *Bot) showWeekDays(u *Update, chat *Chat, kind, value string, weekIndex *utils.WeekIndex) error {
	if b.archive == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	bounds, err := b.archive.WeekIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("history_no_data"))
	}

	wi := weekIndex.Value()
	if int64(wi) < bounds.Min || int64(wi) > bounds.Max {
		return u.Bot.SendText(u.ChatID, b.loc("history_no_data"))
	}

	opts := b.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = buildWeekLabelFromWeek(*weekIndex)

	minIdx, maxIdx := weekIndex.WeekDayIndexRange()
	days, result := b.archiveDaysForRange(kind, value, minIdx, maxIdx)
	if result != archiveAnswered {
		return u.Bot.SendText(u.ChatID, b.loc("history_no_data"))
	}

	text := b.formatDays(chat, kind, value, days, opts)

	if text == "" {
		text = b.loc("no_timetable")
	}

	kb := b.weekControlKeyboard(kind, value, weekIndex.Value(), false)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

var regexp3 = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?$`)

func (b *Bot) parseWeekIndex(text string) *utils.WeekIndex {
	value := strings.TrimSpace(strings.ToLower(text))
	if value == "" {
		wi := b.relevantWeekIndex()
		return &wi
	}

	if matched, _ := strconv.ParseInt(value, 10, 64); matched > 0 {
		wi := utils.WeekIndexFromAcademicNumber(int(matched), b.nowTime())
		return &wi
	}

	dateMatch := regexp3.FindStringSubmatch(value)
	if dateMatch != nil {
		day, _ := strconv.Atoi(dateMatch[1])
		month, _ := strconv.Atoi(dateMatch[2])
		year := b.nowTime().Year()
		if dateMatch[3] != "" {
			year, _ = strconv.Atoi(dateMatch[3])
			if year < 100 {
				year += 2000
			}
		}
		if day > 0 && month > 0 && year > 0 {
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
			wi := utils.WeekIndexFromDate(date)
			return &wi
		}
	}

	return nil
}
