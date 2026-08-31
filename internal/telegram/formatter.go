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
	opts := formatter.FormatOptions{
		IsTelegram:     true,
		ShowParserTime: chat.ShowParserTime,
		ShowHints:      chat.ShowHints,
		HasParserError: !b.cache.SuccessUpdate,
		TeacherNames:   b.cache.GetTeamNames(),
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
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, getDayRasp(extractDays(data)), b.fmtOpts(chat, true))
}

func (b *Bot) formatTeacherDay(chat *Chat, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, getDayRasp(extractDays(data)), b.fmtOpts(chat, true))
}



func (b *Bot) formatGroupFull(chat *Chat, group string, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(group, extractDays(data), b.fmtOpts(chat, true))
}

func (b *Bot) formatTeacherFull(chat *Chat, teacher string, data any) string {
	return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(teacher, extractDays(data), b.fmtOpts(chat, true))
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

func getDayRasp(days []map[string]any) []map[string]any {
	if len(days) == 0 {
		return nil
	}

	now := time.Now()
	today := now.Format("02.01.2006")

	for _, day := range days {
		dateStr, _ := day["day"].(string)
		if dateStr == today {
			return []map[string]any{day}
		}
	}

	return []map[string]any{days[0]}
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
