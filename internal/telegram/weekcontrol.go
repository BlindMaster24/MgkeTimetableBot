package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%s:%s", typeLetter, value, weekIndex-1, payloadFlag(hidePastDays), payloadFlag(showHeader)),
		})
	}

	if hidePastDays && weekIndex == currentWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "🔼",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:0:%s", typeLetter, value, weekIndex, payloadFlag(showHeader)),
		})
	}

	if weekIndex+1 <= maxWeek {
		navRow = append(navRow, telego.InlineKeyboardButton{
			Text:         "➡️",
			CallbackData: fmt.Sprintf("timetable_%s:%s:%d:%s:%s", typeLetter, value, weekIndex+1, payloadFlag(hidePastDays), payloadFlag(showHeader)),
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
	if b.archive != nil {
		if bounds, err := b.archive.WeekIndexBounds(); err == nil && bounds.Max > 0 {
			return int(bounds.Min), int(bounds.Max)
		}
	}
	current := utils.WeekIndexFromDate(b.nowTime()).Value()
	return current - 1, current + 2
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

	if !b.hasCachedValue(typeName, p.Value) {
		return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
	}

	days := b.weekDays(daysFromArchive, typeName, p.Value, week)
	if p.HidePastDays && p.WeekIndex == b.relevantWeekIndex().Value() {
		days = b.removePastDays(days)
	}

	opts := b.fmtOpts(chat, p.ShowHeader)
	opts.WeekLabel = buildWeekLabelFromWeek(week)

	text := b.formatDays(chat, typeName, p.Value, days, opts)
	if text == "" {
		text = b.loc("no_timetable")
	}

	kb := b.weekControlKeyboardHeader(typeName, p.Value, p.WeekIndex, p.HidePastDays, p.ShowHeader)
	return b.sendOrEdit(u, text, kb)
}

func (b *Bot) showWeekSchedule(u *Update, chat *Chat) error {
	target, ok := scheduleTargetFor(chat)
	if !ok {
		return u.Bot.SendText(u.ChatID, b.loc("need_group"))
	}

	if target.value == "" {
		hint := randomKey(target.hintMap(b))
		return u.Bot.SendText(u.ChatID, b.locData(target.notSelectedKey, map[string]interface{}{target.hintField: hint}))
	}

	if !b.hasCachedValue(target.typeName, target.value) {
		return u.Bot.SendText(u.ChatID, b.loc(target.notExistsKey))
	}

	week := b.relevantWeekIndex()
	days := b.weekDays(daysFromCache, target.typeName, target.value, week)
	allDays := days
	if chat.HidePastDays {
		days = b.removePastDays(days)
		if target.rolloverWhenPastHidden && len(days) == 0 && len(allDays) > 0 {
			week = utils.WeekIndexFromNumber(week.Value() + 1)
			days = b.weekDays(daysFromCache, target.typeName, target.value, week)
		}
	}

	opts := b.fmtOpts(chat, false)
	opts.WeekLabel = buildWeekLabelFromWeek(week)
	text := b.formatDays(chat, target.typeName, target.value, days, opts)
	if text == "" {
		return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
	}

	kb := b.weekControlKeyboard(target.typeName, target.value, week.Value(), chat.HidePastDays)
	return b.sendOrEdit(u, text, kb)
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
