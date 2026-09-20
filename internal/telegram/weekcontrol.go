package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

func (b *Bot) weekControlKeyboard(typeName, value string, weekIndex int, hidePastDays bool) *telego.InlineKeyboardMarkup {
	return b.weekControlKeyboardHeader(typeName, value, weekIndex, hidePastDays, false)
}

func (b *Bot) weekControlKeyboardHeader(typeName, value string, weekIndex int, hidePastDays bool, showHeader bool) *telego.InlineKeyboardMarkup {
	minWeek, maxWeek := b.weekIndexBounds()
	currentWeek := utils.WeekIndexFromDate(b.nowTime()).Value()

	typeLetter := string(typeName[0])

	keyboard := &telego.InlineKeyboardMarkup{
		InlineKeyboard: make([][]telego.InlineKeyboardButton, 0),
	}

	var navRow []telego.InlineKeyboardButton

	if weekIndex-1 >= minWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "⬅️",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%s:%s", typeLetter, value, weekIndex-1, boolToInt(hidePastDays), boolToInt(showHeader)),
		})
	}

	if hidePastDays && weekIndex == currentWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "🔼",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:0:%s", typeLetter, value, weekIndex, boolToInt(showHeader)),
		})
	}

	if weekIndex+1 <= maxWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "➡️",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%s:%s", typeLetter, value, weekIndex+1, boolToInt(hidePastDays), boolToInt(showHeader)),
		})
	}

	if len(navRow) > 0 {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []telego.InlineKeyboardButton{
		{
			Text:         "📷 Сгенерировать изображение",
			CallbackData: fmt.Sprintf("image_%s:%s:%d", typeLetter, value, weekIndex),
		},
	})

	return keyboard
}

func (b *Bot) weekIndexBounds() (int, int) {
	if archiveRepo, ok := b.archive.(*archive.Repository); ok && archiveRepo != nil {
		if bounds, err := archiveRepo.WeekIndexBounds(); err == nil && bounds.Max > 0 {
			return int(bounds.Min), int(bounds.Max)
		}
	}
	current := utils.WeekIndexFromDate(b.nowTime()).Value()
	return current - 1, current + 2
}

func boolToInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (b *Bot) relevantWeekIndex() utils.WeekIndex {
	date := b.nowTime()
	weekIndex := utils.WeekIndexFromDate(date)
	if date.Weekday() == time.Sunday {
		weekIndex = utils.WeekIndexFromNumber(weekIndex.Value() + 1)
	}
	relevant := weekIndex.Value()
	if b.cache.Groups != nil && b.cache.Groups.LastWeekIndex > 0 && b.cache.Groups.LastWeekIndex < relevant {
		relevant = b.cache.Groups.LastWeekIndex
	}
	if b.cache.Teachers != nil && b.cache.Teachers.LastWeekIndex > 0 && b.cache.Teachers.LastWeekIndex < relevant {
		relevant = b.cache.Teachers.LastWeekIndex
	}
	return utils.WeekIndexFromNumber(relevant)
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
	payload := strings.TrimPrefix(data, "timetable_"+string(typeName[0])+":")

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
		} else {
			p.HidePastDays = chat.HidePastDays
		}
		if len(parts) >= 4 {
			p.ShowHeader = parts[3] == "1" || parts[3] == "true"
		} else {
			p.ShowHeader = true
		}
	}

	if p.WeekIndex == 0 {
		p.WeekIndex = b.relevantWeekIndex().Value()
	}

	week := utils.WeekIndexFromNumber(p.WeekIndex)
	minIdx, maxIdx := week.WeekDayIndexRange()

	var exists bool

	switch typeName {
	case "group":
		_, exists = b.cache.GetGroups()[p.Value]
	case "teacher":
		_, exists = b.cache.GetTeachers()[p.Value]
	}

	if !exists {
		return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
	}

	days := b.archiveDaysForWeek(typeName, p.Value, minIdx, maxIdx)
	if p.HidePastDays && p.WeekIndex == b.relevantWeekIndex().Value() {
		days = b.removePastDays(days)
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

	kb := b.weekControlKeyboardHeader(typeName, p.Value, p.WeekIndex, p.HidePastDays, p.ShowHeader)
	return b.sendOrEdit(u, text, kb)
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
		idx := utils.DayIndexFromDate(t)
		if idx >= minIdx && idx <= maxIdx {
			result = append(result, day)
		}
	}
	return result
}

func (b *Bot) archiveDaysForWeek(typeName, value string, minIdx, maxIdx int) []map[string]any {
	archiveRepo, ok := b.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		var dataRaw any
		var exists bool
		if typeName == "teacher" {
			dataRaw, exists = b.cache.GetTeachers()[value]
		} else {
			dataRaw, exists = b.cache.GetGroups()[value]
		}
		if !exists {
			return nil
		}
		return extractDaysFromRange(dataRaw, minIdx, maxIdx)
	}

	from := int64(minIdx)
	to := int64(maxIdx)

	if typeName == "teacher" {
		teacherDays, err := archiveRepo.TeacherDaysByRange(from, to, value)
		if err != nil {
			return nil
		}
		return teacherDaysToMaps(teacherDays)
	}

	groupDays, err := archiveRepo.GroupDaysByRange(from, to, value)
	if err != nil {
		return nil
	}
	return groupDaysToMaps(groupDays)
}

func groupDaysToMaps(days []model.GroupDay) []map[string]any {
	var result []map[string]any
	for _, d := range days {
		lessonsRaw, err := json.Marshal(d.Lessons)
		if err != nil {
			continue
		}
		var lessons []any
		if err := json.Unmarshal(lessonsRaw, &lessons); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"day":     d.Day,
			"lessons": lessons,
		})
	}
	return result
}

func teacherDaysToMaps(days []model.TeacherDay) []map[string]any {
	var result []map[string]any
	for _, d := range days {
		lessonsRaw, err := json.Marshal(d.Lessons)
		if err != nil {
			continue
		}
		var lessons []any
		if err := json.Unmarshal(lessonsRaw, &lessons); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"day":     d.Day,
			"lessons": lessons,
		})
	}
	return result
}

func (b *Bot) showWeekScheduleWithKeyboard(u *Update, chat *Chat, typeName, value string) error {
	groups := b.GetRaspCache().GetGroups()
	teachers := b.GetRaspCache().GetTeachers()

	week := b.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, b.locData("group_not_selected", map[string]interface{}{"Group": randomKey(groups)}))
		}
		data, ok := groups[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}

		days := extractDaysFromRange(data, minIdx, maxIdx)
		allDays := days
		if chat.HidePastDays {
			days = b.removePastDays(days)
			if len(days) == 0 && len(allDays) > 0 {
				week = utils.WeekIndexFromNumber(week.Value() + 1)
				minIdx, maxIdx = week.WeekDayIndexRange()
				days = extractDaysFromRange(data, minIdx, maxIdx)
			}
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabelFromWeek(week)
		text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard("group", chat.Group, week.Value(), chat.HidePastDays)
		return b.sendOrEdit(u, text, kb)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, b.locData("teacher_not_selected", map[string]interface{}{"Teacher": randomKey(teachers)}))
		}
		data, ok := teachers[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}

		days := extractDaysFromRange(data, minIdx, maxIdx)
		if chat.HidePastDays {
			days = b.removePastDays(days)
		}

		opts := b.fmtOpts(chat, false)
		opts.WeekLabel = buildWeekLabelFromWeek(week)
		text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, days, opts)
		if text == "" {
			return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
		}

		kb := b.weekControlKeyboard("teacher", chat.Teacher, week.Value(), chat.HidePastDays)
		return b.sendOrEdit(u, text, kb)
	}

	return u.Bot.SendText(u.ChatID, b.loc("need_group"))
}

func buildWeekLabelFromWeek(week utils.WeekIndex) string {
	d1, d2 := week.WeekRange()
	weekNum := week.AcademicWeekNumber()
	return fmt.Sprintf("Учебная неделя №%d (%s-%s)", weekNum,
		d1.Format("02.01.2006"), d2.Format("02.01.2006"))
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
		t1.Format("02.01.2006"), t2.Format("02.01.2006"))
}

func extractWeekTypeAndValue(data string) (string, string) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
