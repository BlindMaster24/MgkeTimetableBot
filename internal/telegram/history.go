package telegram

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
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

	matched, tooMany := matchTeacherList(input, teachers)
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

	return nil
}

type historyWeekScene struct{ bot *Bot }

func (s *historyWeekScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	archiveRepo, ok := s.bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	weekIndex := s.parseWeekIndex(u.Text)
	if weekIndex == nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_invalid_week"))
	}

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	wi := weekIndex.Value()
	if int64(wi) < bounds.Min || int64(wi) > bounds.Max {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	minIdx, maxIdx := weekIndex.WeekDayIndexRange()
	days, err := archiveRepo.TeacherDaysByRange(int64(minIdx), int64(maxIdx), chat.Teacher)
	if err != nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	var dayMaps []map[string]any
	for _, d := range days {
		dm := map[string]any{"day": d.Day}
		var lessonsAny []any
		for _, l := range d.Lessons {
			if l != nil {
				lessonsAny = append(lessonsAny, l)
			}
		}
		dm["lessons"] = lessonsAny
		dayMaps = append(dayMaps, dm)
	}

	opts := s.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = buildWeekLabelFromWeek(*weekIndex)

	text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, dayMaps, opts)
	if text == "" {
		text = s.bot.loc("no_timetable")
	}

	kb := s.bot.weekControlKeyboard("teacher", chat.Teacher, weekIndex.Value(), false)

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

var regexp3 = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?$`)

func (s *historyWeekScene) parseWeekIndex(text string) *utils.WeekIndex {
	value := strings.TrimSpace(strings.ToLower(text))
	if value == "" {
		wi := utils.WeekIndexFromDate(time.Now())
		return &wi
	}

	if matched, _ := strconv.ParseInt(value, 10, 64); matched > 0 {
		wi := utils.WeekIndexFromNumber(int(matched))
		return &wi
	}

	dateMatch := regexp3.FindStringSubmatch(value)
	if dateMatch != nil {
		day, _ := strconv.Atoi(dateMatch[1])
		month, _ := strconv.Atoi(dateMatch[2])
		year := time.Now().Year()
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
