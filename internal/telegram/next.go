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
	schedule := c.bot.callsScheduleFor(chat)
	calls := reminder.Schedule{Weekdays: schedule.Weekdays, Saturday: schedule.Saturday}

	var lines []string
	// "Следующая пара" is a question about the future, not about today: after
	// the bell rings on Friday the answer used to be "no lessons left" even
	// though Monday's first lesson was known.
	for offset := 0; offset < 7; offset++ {
		date := now.AddDate(0, 0, offset)
		day, ok := c.bot.cache.TimetableDays(cacheKind(target.typeName), timetable.FormatDate(date))[target.value]
		if !ok {
			continue
		}
		if offset == 0 {
			if current, ok := reminder.Current(day, now, calls); ok {
				lines = append(lines, c.bot.locData("next_current", nextLessonFields(current)))
			}
			if upcoming, ok := reminder.Upcoming(day, now, calls); ok {
				lines = append(lines, c.bot.locData("next_upcoming", nextLessonFields(upcoming)))
			}
			continue
		}
		if upcoming, ok := reminder.NextLesson(day, now, calls); ok {
			fields := nextLessonFields(upcoming)
			fields["Date"] = date.Format("02.01")
			lines = append(lines, c.bot.locData("next_upcoming_day", fields))
		}
		if len(lines) > 0 {
			break
		}
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
