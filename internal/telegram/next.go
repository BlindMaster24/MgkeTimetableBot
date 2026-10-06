package telegram

import (
	"context"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/reminder"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type nextLessonCmd struct{ bot *Bot }

func (c *nextLessonCmd) Name() string        { return "/next" }
func (c *nextLessonCmd) Description() string { return c.bot.loc("cmd_next") }

func (c *nextLessonCmd) MatchText(text string) bool {
	return strings.ToLower(strings.TrimSpace(text)) == "/next"
}

func (c *nextLessonCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	target, ok := scheduleTargetFor(chat)
	if !ok {
		return u.Bot.SendText(u.ChatID, c.bot.loc("next_no_target"))
	}
	if strings.TrimSpace(target.value) == "" {
		hint := map[string]any{}
		if target.hintMap != nil && target.hintField != "" {
			hint[target.hintField] = randomKey(target.hintMap(c.bot))
		}
		return u.Bot.SendText(u.ChatID, c.bot.locData(target.notSelectedKey, hint))
	}

	now := c.bot.nowTime()
	day, ok := c.bot.cache.TimetableDays(cacheKind(target.typeName), timetable.FormatDate(now))[target.value]
	if !ok {
		return u.Bot.SendText(u.ChatID, c.bot.loc("next_no_data"))
	}

	calls := reminder.Schedule{
		Weekdays: c.bot.cache.GetCallsWeekdays(),
		Saturday: c.bot.cache.GetCallsSaturday(),
	}

	var lines []string
	if current, ok := reminder.Current(day, now, calls); ok {
		lines = append(lines, c.bot.locData("next_current", nextLessonFields(current)))
	}
	if upcoming, ok := reminder.Upcoming(day, now, calls); ok {
		lines = append(lines, c.bot.locData("next_upcoming", nextLessonFields(upcoming)))
	}
	if len(lines) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("next_none"))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

func nextLessonFields(lesson reminder.LessonAt) map[string]any {
	return map[string]any{
		"Subject": lesson.Lesson.Subject,
		"Room":    lesson.Lesson.Room,
		"Start":   lesson.Start.Format("15:04"),
		"End":     lesson.End.Format("15:04"),
	}
}
