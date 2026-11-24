package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

var archiveWeekRe = regexp.MustCompile(`(?i)^(week|неделя)\s+(\d+)$`)

type archiveCmd struct{ bot *Bot }

func (c *archiveCmd) Name() string { return "/archive" }
func (c *archiveCmd) Description() string {
	return c.bot.loc("archtext_2")
}

func (c *archiveCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	raw := strings.TrimSpace(strings.TrimPrefix(u.Text, "/archive"))
	if raw == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_3"))
	}

	if c.bot.archive == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_4"))
	}

	if match := archiveWeekRe.FindStringSubmatch(raw); match != nil {
		return c.showWeek(ctx, u, chat, match[2])
	}

	return c.showDay(ctx, u, chat, raw)
}

func (c *archiveCmd) showWeek(ctx context.Context, u *Update, chat *Chat, rawWeek string) error {
	weekNum, _ := strconv.Atoi(rawWeek)
	if weekNum < 1 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_5"))
	}

	week := utils.WeekIndexFromAcademicNumber(weekNum, c.bot.nowTime())
	minIdx, maxIdx := week.WeekDayIndexRange()
	return c.showRange(ctx, u, chat, minIdx, maxIdx, c.bot.buildWeekLabelFromWeek(week), c.bot.loc("archtext_6"), week.Value())
}

func (c *archiveCmd) showDay(ctx context.Context, u *Update, chat *Chat, raw string) error {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_7"))
	}

	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year := c.bot.nowTime().Year()
	if len(parts) == 3 {
		year, _ = strconv.Atoi(parts[2])
	}

	if day < 1 || day > 31 || month < 1 || month > 12 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_7"))
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, utils.SiteLocation)
	// time.Date normalizes 31.02 into 03.03; the archive must not answer a date
	// the operator never typed.
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_7"))
	}
	index := int64(utils.DayIndexFromDate(date))

	bounds, err := c.bot.archive.DayIndexBounds(ctx)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_8"))
	}

	if index < bounds.Min || index > bounds.Max {
		from := utils.DayIndexToDate(int(bounds.Min)).Format("02.01.2006")
		to := utils.DayIndexToDate(int(bounds.Max)).Format("02.01.2006")
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_9")+
			fmt.Sprintf(c.bot.loc("archtext_10"), from, to))
	}

	return c.showRange(ctx, u, chat, int(index), int(index), "", c.bot.loc("archtext_11"), utils.WeekIndexFromDate(date).Value())
}

func (c *archiveCmd) showRange(ctx context.Context, u *Update, chat *Chat, minIdx, maxIdx int, weekLabel, emptyMessage string, weekIndex int) error {
	target, ok := scheduleTargetFor(chat)
	if !ok {
		return u.Bot.SendText(u.ChatID, c.bot.locData("archive_mode_unsupported", map[string]interface{}{"Mode": chat.Mode}))
	}

	if target.value == "" {
		key := "archive_need_group"
		if target.typeName == "teacher" {
			key = "archive_need_teacher"
		}
		return u.Bot.SendText(u.ChatID, c.bot.loc(key))
	}

	days, result := c.bot.archiveDaysForRange(ctx, target.typeName, target.value, minIdx, maxIdx)
	if result != archiveAnswered || len(days) == 0 {
		return u.Bot.SendText(u.ChatID, emptyMessage)
	}

	opts := c.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = weekLabel

	text := c.bot.formatDays(chat, target.typeName, target.value, days, opts)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, weekTimetableButton(c.bot.loc("archtext_12"), target.typeName, target.value, weekIndex, true))
}

type endingsCmd struct{ bot *Bot }

func (c *endingsCmd) Name() string { return "/endings" }
func (c *endingsCmd) Description() string {
	return c.bot.loc("archtext_13")
}
func (c *endingsCmd) Handler(ctx context.Context, u *Update) error {
	groups := c.bot.cache.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_14"))
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
		return u.Bot.SendText(u.ChatID, c.bot.loc("archtext_15"))
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
			part = append(part, fmt.Sprintf(c.bot.loc("archtext_16"), stat[day][lesson], lesson))
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
