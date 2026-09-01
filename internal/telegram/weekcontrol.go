package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

func removePastDays(days []map[string]any) []map[string]any {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	startIdx := -1
	for i, day := range days {
		dateStr, _ := day["day"].(string)
		dayTime, err := time.Parse("02.01.2006", dateStr)
		if err != nil {
			continue
		}
		if !dayTime.Before(today) {
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
		if firstDay == today.Format("02.01.2006") {
			lessons, _ := result[0]["lessons"].([]any)
			if len(lessons) == 0 && len(result) > 1 {
				result = result[1:]
			}
		}
	}

	return result
}

func (b *Bot) weekControlKeyboard(typeName, value string, weekIndex int, hidePastDays bool) *telego.InlineKeyboardMarkup {
	currentWeek := utils.WeekIndexFromDate(time.Now())
	minWeek := 0
	maxWeek := currentWeek.Value() + 2

	keyboard := &telego.InlineKeyboardMarkup{
		InlineKeyboard: make([][]telego.InlineKeyboardButton, 0),
	}

	var navRow []telego.InlineKeyboardButton

	if weekIndex-1 >= minWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "⬅️",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%v:%v", typeName, value, weekIndex-1, boolToInt(hidePastDays), false),
		})
	}

	if hidePastDays && weekIndex == currentWeek.Value() {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "🔼",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%v:%v", typeName, value, weekIndex, false, false),
		})
	}

	if weekIndex+1 <= maxWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "➡️",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%v:%v", typeName, value, weekIndex+1, boolToInt(hidePastDays), false),
		})
	}

	if len(navRow) > 0 {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []telego.InlineKeyboardButton{
		{
			Text:         "📷 Сгенерировать изображение",
			CallbackData: fmt.Sprintf("image_%s:%s", typeName, value),
		},
	})

	return keyboard
}

func boolToInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

type timetableGroupCb struct{ bot *Bot }

func (cb *timetableGroupCb) Prefix() string { return "timetable_g:" }
func (cb *timetableGroupCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.handleTimetableCb(u, chat, "group", u.Data)
}

type timetableTeacherCb struct{ bot *Bot }

func (cb *timetableTeacherCb) Prefix() string { return "timetable_t:" }
func (cb *timetableTeacherCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	return cb.bot.handleTimetableCb(u, chat, "teacher", u.Data)
}

func (b *Bot) handleTimetableCb(u *Update, chat *Chat, typeName, data string) error {
	payload := strings.TrimPrefix(data, "timetable_"+typeName+":")

	type timetablePayload struct {
		Value        string
		WeekIndex    int
		HidePastDays bool
		ShowHeader   bool
	}

	var p timetablePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		parts := strings.SplitN(payload, ":", 4)
		if len(parts) >= 1 {
			p.Value = parts[0]
		}
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &p.WeekIndex)
		}
		if len(parts) >= 3 {
			p.HidePastDays = parts[2] == "1" || parts[2] == "true"
		}
		if len(parts) >= 4 {
			p.ShowHeader = parts[3] == "1" || parts[3] == "true"
		}
	}

	if p.WeekIndex == 0 {
		p.WeekIndex = utils.WeekIndexFromDate(time.Now()).Value()
	}

	week := utils.WeekIndexFromNumber(p.WeekIndex)
	minIdx, maxIdx := week.WeekDayIndexRange()

	var dataRaw any
	var exists bool

	switch typeName {
	case "group":
		dataRaw, exists = b.cache.GetGroups()[p.Value]
	case "teacher":
		dataRaw, exists = b.cache.GetTeachers()[p.Value]
	}

	if !exists {
		return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
	}
	dataMap, _ := dataRaw.(map[string]any)

	days := extractDaysFromRange(dataMap, minIdx, maxIdx)
	if chat.HidePastDays && p.WeekIndex == utils.WeekIndexFromDate(time.Now()).Value() {
		days = removePastDays(days)
	}

	opts := b.fmtOpts(chat, p.ShowHeader)
	opts.WeekLabel = buildWeekLabelFromWeek(week)

	var text string
	switch typeName {
	case "group":
		text = formatter.GetByIndex(chat.Formatter).FormatGroupFull(p.Value, days, opts)
	case "teacher":
		text = formatter.GetByIndex(chat.Formatter).FormatTeacherFull(p.Value, days, opts)
	}

	if text == "" {
		text = b.loc("no_timetable")
	}

	kb := b.weekControlKeyboard(typeName, p.Value, p.WeekIndex, chat.HidePastDays)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

func extractDaysFromRange(data any, minIdx, maxIdx int) []map[string]any {
	allDays := extractDays(data)
	if len(allDays) == 0 {
		return nil
	}

	var result []map[string]any
	for _, day := range allDays {
		dateStr, _ := day["day"].(string)
		t, err := time.Parse("02.01.2006", dateStr)
		if err != nil {
			continue
		}
		di := utils.WeekIndexFromDate(t)
		idx := di.Value()
		if idx >= minIdx && idx <= maxIdx {
			result = append(result, day)
		}
	}
	return result
}

func (b *Bot) showWeekScheduleWithKeyboard(u *Update, chat *Chat, typeName, value string) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	week := utils.WeekIndexFromDate(time.Now())
	minIdx, maxIdx := week.WeekDayIndexRange()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_group"))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}

		days := extractDaysFromRange(data, minIdx, maxIdx)
		if chat.HidePastDays {
			days = removePastDays(days)
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabelFromWeek(week)
		text := formatter.GetByIndex(chat.Formatter).FormatGroupFull("", days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard("group", chat.Group, week.Value(), chat.HidePastDays)
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.loc("need_teacher"))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}

		days := extractDaysFromRange(data, minIdx, maxIdx)
		if chat.HidePastDays {
			days = removePastDays(days)
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabelFromWeek(week)
		text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull("", days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard("teacher", chat.Teacher, week.Value(), chat.HidePastDays)
		return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
	}

	return u.Bot.SendText(u.ChatID, b.loc("need_group"))
}

func buildWeekLabelFromWeek(week utils.WeekIndex) string {
	d1, d2 := week.WeekRange()
	weekNum := week.AcademicWeekNumber()
	return fmt.Sprintf("Учебная неделя №%d (%s-%s)", weekNum,
		d1.Format("02.01"), d2.Format("02.01"))
}

func buildWeekLabel(days []map[string]any) string {
	if len(days) == 0 {
		return ""
	}

	firstDay, _ := days[0]["day"].(string)
	lastDay, _ := days[len(days)-1]["day"].(string)

	t1, err1 := time.Parse("02.01.2006", firstDay)
	t2, err2 := time.Parse("02.01.2006", lastDay)
	if err1 != nil || err2 != nil {
		return ""
	}

	week := utils.WeekIndexFromDate(t1)
	weekNum := week.AcademicWeekNumber()

	return fmt.Sprintf("Учебная неделя №%d (%s-%s)", weekNum,
		t1.Format("02.01"), t2.Format("02.01"))
}

func extractWeekTypeAndValue(data string) (string, string) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
