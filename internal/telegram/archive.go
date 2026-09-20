package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

var archiveWeekRe = regexp.MustCompile(`(?i)^(week|неделя)\s+(\d+)$`)

type archiveCmd struct{ bot *Bot }

func (c *archiveCmd) Name() string { return "/archive" }
func (c *archiveCmd) Description() string {
	return "Архив расписания за прошедшие дни"
}

func (c *archiveCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	raw := strings.TrimSpace(strings.TrimPrefix(u.Text, "/archive"))
	if raw == "" {
		return u.Bot.SendText(u.ChatID, "День не указан")
	}

	archiveRepo, ok := c.bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	if match := archiveWeekRe.FindStringSubmatch(raw); match != nil {
		return c.showWeek(u, chat, archiveRepo, match[2])
	}

	return c.showDay(u, chat, archiveRepo, raw)
}

func (c *archiveCmd) showWeek(u *Update, chat *Chat, repo *archive.Repository, rawWeek string) error {
	weekNum, _ := strconv.Atoi(rawWeek)
	if weekNum < 1 {
		return u.Bot.SendText(u.ChatID, "Неверный номер недели")
	}

	week := utils.WeekIndexFromAcademicNumber(weekNum, c.bot.nowTime())
	minIdx, maxIdx := week.WeekDayIndexRange()
	return c.showRange(u, chat, repo, int64(minIdx), int64(maxIdx), buildWeekLabelFromWeek(week), "Нет данных за указанную неделю", week.Value())
}

const archiveDateFormatHint = "Неверный формат даты. Пример: /archive 12.02 или /archive 12.02.2026 или /archive week 5"

func (c *archiveCmd) showDay(u *Update, chat *Chat, repo *archive.Repository, raw string) error {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return u.Bot.SendText(u.ChatID, archiveDateFormatHint)
	}

	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year := c.bot.nowTime().Year()
	if len(parts) == 3 {
		year, _ = strconv.Atoi(parts[2])
	}

	if day < 1 || day > 31 || month < 1 || month > 12 {
		return u.Bot.SendText(u.ChatID, archiveDateFormatHint)
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	index := int64(utils.DayIndexFromDate(date))

	bounds, err := repo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения архива")
	}

	if index < bounds.Min || index > bounds.Max {
		from := utils.DayIndexToDate(int(bounds.Min)).Format("02.01.2006")
		to := utils.DayIndexToDate(int(bounds.Max)).Format("02.01.2006")
		return u.Bot.SendText(u.ChatID, "Вы указали день, который находится вне периода сохранённых дней.\n"+
			fmt.Sprintf("В базе хранятся дни, начиная с %s по %s", from, to))
	}

	return c.showRange(u, chat, repo, index, index, "", "Ничего не найдено на данный день", utils.WeekIndexFromDate(date).Value())
}

func (c *archiveCmd) showRange(u *Update, chat *Chat, repo *archive.Repository, minIdx, maxIdx int64, weekLabel, emptyMessage string, weekIndex int) error {
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("archive_need_group"))
		}
		days, err := repo.GroupDaysByRange(minIdx, maxIdx, chat.Group)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, emptyMessage)
		}
		return c.sendGroupDays(u, chat, days, weekLabel, weekIndex)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("archive_need_teacher"))
		}
		days, err := repo.TeacherDaysByRange(minIdx, maxIdx, chat.Teacher)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, emptyMessage)
		}
		return c.sendTeacherDays(u, chat, days, weekLabel, weekIndex)
	}

	return u.Bot.SendText(u.ChatID, c.bot.locData("archive_mode_unsupported", map[string]interface{}{"Mode": chat.Mode}))
}

func (c *archiveCmd) sendGroupDays(u *Update, chat *Chat, days []model.GroupDay, weekLabel string, weekIndex int) error {
	opts := c.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = weekLabel

	text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, groupDaysToMaps(days), opts)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, weekTimetableButton("На неделю", "group", chat.Group, weekIndex, true))
}

func (c *archiveCmd) sendTeacherDays(u *Update, chat *Chat, days []model.TeacherDay, weekLabel string, weekIndex int) error {
	opts := c.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = weekLabel

	text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, teacherDaysToMaps(days), opts)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, weekTimetableButton("На неделю", "teacher", chat.Teacher, weekIndex, true))
}

type endingsCmd struct{ bot *Bot }

func (c *endingsCmd) Name() string { return "/endings" }
func (c *endingsCmd) Description() string {
	return "Отображает сколько групп заканчивают к определённой паре"
}
func (c *endingsCmd) Handler(ctx context.Context, u *Update) error {
	groups := c.bot.cache.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, "Данные ещё не загружены")
	}

	type dayStat map[int]int
	stat := make(map[string]dayStat)

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dataMap, ok := groups[name].(map[string]any)
		if !ok {
			continue
		}
		daysArr, ok := dataMap["days"].([]any)
		if !ok {
			continue
		}

		days := make([]map[string]any, 0, len(daysArr))
		for _, d := range daysArr {
			if dayMap, ok := d.(map[string]any); ok {
				days = append(days, dayMap)
			}
		}
		sort.SliceStable(days, func(i, j int) bool {
			left, _ := days[i]["day"].(string)
			right, _ := days[j]["day"].(string)
			leftTime, leftErr := time.Parse("02.01.2006", left)
			rightTime, rightErr := time.Parse("02.01.2006", right)
			if leftErr != nil || rightErr != nil {
				return left < right
			}
			return leftTime.Before(rightTime)
		})

		relevant := c.bot.getDayRasp(days, false)
		if len(relevant) == 0 {
			continue
		}

		dayStr, _ := relevant[0]["day"].(string)
		lessons, _ := relevant[0]["lessons"].([]any)
		lastLesson := lastLessonIndex(lessons)
		if lastLesson == -1 {
			continue
		}
		if stat[dayStr] == nil {
			stat[dayStr] = make(dayStat)
		}
		stat[dayStr][lastLesson+1]++
	}

	if len(stat) == 0 {
		return u.Bot.SendText(u.ChatID, "Нет данных для отображения")
	}

	days := make([]string, 0, len(stat))
	for day := range stat {
		days = append(days, day)
	}
	sort.SliceStable(days, func(i, j int) bool {
		left, leftErr := time.Parse("02.01.2006", days[i])
		right, rightErr := time.Parse("02.01.2006", days[j])
		if leftErr != nil || rightErr != nil {
			return days[i] < days[j]
		}
		return left.Before(right)
	})

	var msg []string
	for _, day := range days {
		lessons := make([]int, 0, len(stat[day]))
		for lesson := range stat[day] {
			lessons = append(lessons, lesson)
		}
		sort.Ints(lessons)

		part := []string{"__ " + day + " __"}
		for _, lesson := range lessons {
			part = append(part, fmt.Sprintf("%d групп заканчивают к %d паре", stat[day][lesson], lesson))
		}
		msg = append(msg, strings.Join(part, "\n"))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(msg, "\n\n"))
}

func lastLessonIndex(lessons []any) int {
	last := -1

	for i, lesson := range lessons {
		switch v := lesson.(type) {
		case nil:
			continue
		case map[string]any:
			if _, ok := v["lesson"]; ok {
				last = i
			}
		case []any:
			for _, sub := range v {
				if subMap, ok := sub.(map[string]any); ok {
					if _, ok := subMap["lesson"]; ok {
						last = i
					}
				}
			}
		}
	}

	return last
}
