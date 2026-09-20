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
		return u.Bot.SendText(u.ChatID, "День не указан. Пример: /archive 12.02 или /archive week 5")
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

	week := utils.WeekIndexFromNumber(weekNum)
	minIdx, maxIdx := week.WeekDayIndexRange()
	return c.showRange(u, chat, repo, int64(minIdx), int64(maxIdx), buildWeekLabelFromWeek(week), "Нет данных за указанную неделю")
}

func (c *archiveCmd) showDay(u *Update, chat *Chat, repo *archive.Repository, raw string) error {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return u.Bot.SendText(u.ChatID, "Неверный формат. Пример: /archive 12.02 или /archive 12.02.2026")
	}

	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year := time.Now().Year()
	if len(parts) == 3 {
		year, _ = strconv.Atoi(parts[2])
	}

	if day < 1 || day > 31 || month < 1 || month > 12 {
		return u.Bot.SendText(u.ChatID, "Неверная дата")
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	index := int64(utils.DayIndexFromDate(date))

	bounds, err := repo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения архива")
	}

	if index < bounds.Min || index > bounds.Max {
		return u.Bot.SendText(u.ChatID, "Дата вне периода сохранённых данных")
	}

	return c.showRange(u, chat, repo, index, index, "", "Ничего не найдено на данный день")
}

func (c *archiveCmd) showRange(u *Update, chat *Chat, repo *archive.Repository, minIdx, maxIdx int64, weekLabel, emptyMessage string) error {
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
		}
		days, err := repo.GroupDaysByRange(minIdx, maxIdx, chat.Group)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, emptyMessage)
		}
		return c.sendGroupDays(u, chat, days, weekLabel)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
		}
		days, err := repo.TeacherDaysByRange(minIdx, maxIdx, chat.Teacher)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, emptyMessage)
		}
		return c.sendTeacherDays(u, chat, days, weekLabel)
	}

	return u.Bot.SendText(u.ChatID, "Выберите группу или учителя")
}

func (c *archiveCmd) sendGroupDays(u *Update, chat *Chat, days []model.GroupDay, weekLabel string) error {
	opts := c.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = weekLabel

	text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, groupDaysToMaps(days), opts)
	return u.Bot.SendText(u.ChatID, text)
}

func (c *archiveCmd) sendTeacherDays(u *Update, chat *Chat, days []model.TeacherDay, weekLabel string) error {
	opts := c.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = weekLabel

	text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, teacherDaysToMaps(days), opts)
	return u.Bot.SendText(u.ChatID, text)
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

	for _, raw := range groups {
		dataMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		daysArr, ok := dataMap["days"].([]any)
		if !ok {
			continue
		}
		for _, d := range daysArr {
			dayMap, ok := d.(map[string]any)
			if !ok {
				continue
			}
			dayStr, _ := dayMap["day"].(string)
			lessons, _ := dayMap["lessons"].([]any)
			lastLesson := -1
			for i, l := range lessons {
				if l == nil {
					continue
				}
				switch v := l.(type) {
				case map[string]any:
					if _, ok := v["lesson"]; ok {
						lastLesson = i
					}
				case []any:
					for _, sub := range v {
						if subMap, ok := sub.(map[string]any); ok {
							if _, ok := subMap["lesson"]; ok {
								lastLesson = i
							}
						}
					}
				}
			}
			if lastLesson == -1 {
				continue
			}
			if stat[dayStr] == nil {
				stat[dayStr] = make(dayStat)
			}
			stat[dayStr][lastLesson+1]++
		}
	}

	if len(stat) == 0 {
		return u.Bot.SendText(u.ChatID, "Нет данных для отображения")
	}

	var msg []string
	for day, counts := range stat {
		part := []string{"__ " + day + " __"}
		for lesson, count := range counts {
			part = append(part, fmt.Sprintf("%d групп заканчивают к %d паре", count, lesson))
		}
		msg = append(msg, strings.Join(part, "\n"))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(msg, "\n\n"))
}
