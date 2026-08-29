package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/mymmrac/telego"
)

func removePastDays(days []map[string]any) []map[string]any {
	now := time.Now()
	today := now.Format("02.01.2006")

	startIdx := -1
	for i, day := range days {
		dateStr, _ := day["day"].(string)
		if dateStr >= today {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return nil
	}

	result := days[startIdx:]

	if len(result) > 0 {
		firstDay, _ := result[0]["day"].(string)
		if firstDay == today {
			lessons, _ := result[0]["lessons"].([]any)
			if len(lessons) == 0 && len(result) > 1 {
				result = result[1:]
			}
		}
	}

	return result
}

func (b *Bot) weekControlKeyboard(chat *Chat, typeName, value string) *telego.InlineKeyboardMarkup {
	onOff := func(v bool) string {
		if v {
			return "📷 Сгенерировать изображение"
		}
		return ""
	}

	imgText := onOff(true)
	if imgText == "" {
		return nil
	}

	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: imgText, CallbackData: fmt.Sprintf("image_%s:%s", typeName, value)},
			},
		},
	}
}

func (b *Bot) showWeekScheduleWithKeyboard(u *Update, chat *Chat, typeName, value string) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_group"))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}

		days := extractDays(data)
		if chat.HidePastDays {
			days = removePastDays(days)
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabel(days)
		text := formatter.GetByIndex(chat.Formatter).FormatGroupFull("", days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard(chat, "group", chat.Group)
		if kb != nil {
			return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
		}
		return u.Bot.SendText(u.ChatID, text)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_teacher"))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}

		days := extractDays(data)
		if chat.HidePastDays {
			days = removePastDays(days)
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabel(days)
		text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull("", days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard(chat, "teacher", chat.Teacher)
		if kb != nil {
			return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
		}
		return u.Bot.SendText(u.ChatID, text)
	}

	return u.Bot.SendText(u.ChatID, b.loc("need_group"))
}

func extractWeekTypeAndValue(data string) (string, string) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
