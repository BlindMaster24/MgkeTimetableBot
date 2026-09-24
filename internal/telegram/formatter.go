package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
)

var hints = []string{
	"Настроить оповещения можно в настройках (/notice)",
	"Не нравится вид расписания? Попробуй новый в настройках! (/formatter)",
	"Мешают лишние кнопки? Убери их в настройках! (/buttons)",
	"Установил не ту группу или учителя? Измени в настройках! (/setup)",
	"Ты разработчик? Имеешь навыки в программировании на Go? Сделай бота лучше! (/dev)",
	"Проблема с ботом? Не бойся писать разработчику (/about)",
	"Вопрос по боту? Не бойся спросить у разработчика (/about)",
	"Мешают подсказки? Убери в настройках! (/view)",
	"Хочешь интегрировать расписание в свой проект? У бота есть API! (/api)",
}

func (b *Bot) getRandHint() string {
	if len(hints) == 0 {
		return ""
	}
	return hints[time.Now().UnixNano()%int64(len(hints))]
}

func (b *Bot) getFormatterOpts(chat *Chat) formatter.FormatOptions {
	update := b.cache.GetGroupsUpdateTime()
	if chat.Mode == ModeTeacher {
		update = b.cache.GetTeachersUpdateTime()
	}

	opts := formatter.FormatOptions{
		IsTelegram:       true,
		ShowParserTime:   chat.ShowParserTime,
		ParserUpdateTime: update.UnixMilli(),
		ShowHints:        chat.ShowHints,
		HasParserError:   !b.cache.SuccessUpdate,
		TeacherNames:     b.cache.GetTeamNames(),
		Now:              b.nowTime(),
	}

	if opts.ShowHints && !opts.HasParserError {
		opts.RandHint = b.getRandHint()

		if (chat.Mode == ModeStudent || chat.Mode == ModeParent) && chat.Group == "" {
			opts.RandHint = "Выберите группу в настройках (/setup)"
		} else if chat.Mode == "teacher" && chat.Teacher == "" {
			opts.RandHint = "Выберите преподавателя в настройках (/setup)"
		} else if !chat.ShowDaily && !chat.ShowWeekly {
			opts.RandHint = "Верни кнопки расписания в настройках (/buttons)"
		}
	}

	return opts
}

func (b *Bot) fmtOpts(chat *Chat, showHeader bool) formatter.FormatOptions {
	opts := b.getFormatterOpts(chat)
	opts.ShowHeader = showHeader
	return opts
}

func (b *Bot) formatGroupDay(chat *Chat, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, b.getDayRasp(extractDays(data)), b.fmtOpts(chat, false))
}

func (b *Bot) formatTeacherDay(chat *Chat, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, b.getDayRasp(extractDays(data)), b.fmtOpts(chat, false))
}

func (b *Bot) formatGroupFull(chat *Chat, group string, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(group, extractDays(data), b.fmtOpts(chat, false))
}

func (b *Bot) formatTeacherFull(chat *Chat, teacher string, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(teacher, extractDays(data), b.fmtOpts(chat, false))
}

func extractDays(data any) []map[string]any {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return nil
	}

	var result []map[string]any
	for _, d := range daysArr {
		if m, ok := d.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func (b *Bot) getDayRasp(days []map[string]any, args ...any) []map[string]any {
	autoskip := true
	maxDays := 1
	for _, arg := range args {
		switch v := arg.(type) {
		case bool:
			autoskip = v
		case int:
			maxDays = v
		}
	}

	nextDays := b.removePastDaysArgs(days, autoskip)
	if len(nextDays) == 0 {
		return nil
	}

	showDays := []map[string]any{nextDays[0]}
	for i := 1; i < maxDays; i++ {
		if i >= len(nextDays) {
			break
		}
		showDays = append(showDays, nextDays[i])
	}
	return showDays
}

func (b *Bot) nowTime() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

func (b *Bot) nowInTime(includedDays []int, timeFrom, timeTo string) bool {
	parseMin := func(s string) (int, bool) {
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return 0, false
		}
		var h, m int
		if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
			return 0, false
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
			return 0, false
		}
		return h*60 + m, true
	}
	fromMin, ok1 := parseMin(timeFrom)
	toMin, ok2 := parseMin(timeTo)
	if !ok1 || !ok2 {
		return false
	}
	now := b.nowTime()
	nowMin := now.Hour()*60 + now.Minute()
	weekday := int(now.Weekday())
	for _, d := range includedDays {
		if d == weekday && nowMin >= fromMin && nowMin <= toMin {
			return true
		}
	}
	return false
}

func (b *Bot) removePastDays(days []map[string]any) []map[string]any {
	return b.removePastDaysArgs(days, true)
}

func (b *Bot) removePastDaysArgs(days []map[string]any, autoskip bool) []map[string]any {
	isSaturday := b.nowTime().Weekday() == time.Saturday

	idx := -1
	for i, day := range days {
		dateStr, _ := day["day"].(string)
		t, err := time.Parse("02.01.2006", dateStr)
		if err != nil {
			continue
		}
		now := b.nowTime()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dayMidnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
		if !dayMidnight.Before(today) {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil
	}

	nextDays := days[idx:]

	if autoskip && len(nextDays) > 0 {
		currentDay := nextDays[0]
		var timetable [][2][2]string
		if isSaturday {
			timetable = b.cfg.Timetable.Saturday
		} else {
			timetable = b.cfg.Timetable.Weekdays
		}
		if len(timetable) > 0 {
			lessons, _ := currentDay["lessons"].([]any)
			todayLessons := len(lessons)
			if todayLessons == 0 || todayLessons > len(timetable) {
				todayLessons = len(timetable)
			}
			lastLessonTime := timetable[todayLessons-1][1][1]

			dateStr, _ := currentDay["day"].(string)
			now := b.nowTime()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			t, err := time.Parse("02.01.2006", dateStr)
			isToday := err == nil && time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location()).Equal(today)

			includedDays := []int{1, 2, 3, 4, 5}
			if isSaturday {
				includedDays = []int{6}
			}

			if isToday && !b.nowInTime(includedDays, "00:00", lastLessonTime) {
				nextDays = nextDays[1:]
			}
		}
	}

	return nextDays
}

func (b *Bot) formatCallsSchedule() string {
	timetable := b.cfg.Timetable
	var sb strings.Builder
	if len(timetable.Weekdays) > 0 {
		sb.WriteString(b.loc("calls_weekdays"))
		sb.WriteString("\n")
		for i, slot := range timetable.Weekdays {
			sb.WriteString(fmt.Sprintf("  %s-%s / %s-%s", slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
			if i < len(timetable.Weekdays)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if len(timetable.Saturday) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(b.loc("calls_saturday"))
		sb.WriteString("\n")
		for i, slot := range timetable.Saturday {
			sb.WriteString(fmt.Sprintf("  %s-%s / %s-%s", slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
			if i < len(timetable.Saturday)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if sb.Len() == 0 {
		return b.loc("calls_not_configured")
	}
	return sb.String()
}
