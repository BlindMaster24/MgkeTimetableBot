package notification

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

const (
	getWeekTimetableCallback = "timetable_%s:%s:%d:0:1"
	getCallsCallback         = "calls_full"
)

type MessageOptions struct {
	InlineKeyboard [][]KeyboardButton
}

type KeyboardButton struct {
	Text string
	Data string
}

type EventSender interface {
	SendText(chatID int64, text string) error
	SendTextWithButtons(chatID int64, text string, buttons []KeyboardButton) error
}

type ChatMode string

const (
	ModeStudent ChatMode = "student"
	ModeTeacher ChatMode = "teacher"
	ModeParent  ChatMode = "parent"
)

type EventChat struct {
	ID      int64
	PeerID  int64
	Mode    string
	Group   string
	Teacher string

	NoticeChanges    bool
	NoticeNextWeek   bool
	NoticeCalls      bool
	NoticeParserErrs bool
	AllowSendMess    bool

	Formatter      int
	HidePastDays   bool
	ShowHints      bool
	ShowParserTime bool
}

type EventChatFinder interface {
	FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*EventChat, error)
	FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*EventChat, error)
	FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*EventChat, error)
	FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*EventChat, error)
	FindChatsWithNotice(service string, notice string) ([]*EventChat, error)
	FindAdminChats(service string) ([]*EventChat, error)
}

type EventNotifier struct {
	cache  *cache.RaspCache
	cfg    *config.Config
	log    *logger.Logger
	sender EventSender
	chats  EventChatFinder
}

func NewEventNotifier(c *cache.RaspCache, cfg *config.Config, log *logger.Logger, sender EventSender, chats EventChatFinder) *EventNotifier {
	return &EventNotifier{cache: c, cfg: cfg, log: log, sender: sender, chats: chats}
}

func getDayPhrase(day string, nextDayPhrase string) string {
	t, err := time.Parse("02.01.2006", day)
	if err != nil {
		return nextDayPhrase
	}

	dayIdx := utils.DayIndexFromDate(t)
	todayIdx := utils.DayIndexFromDate(time.Now())

	if dayIdx == todayIdx {
		return "сегодня"
	}
	if dayIdx == todayIdx+1 {
		return "завтра"
	}

	if utils.WeekIndexFromDate(t).IsFutureWeek() {
		return "следующую неделю"
	}

	return nextDayPhrase
}

func (n *EventNotifier) dayHeader(isUpdate bool, phrase, kind, value, ownValue string) string {
	if ownValue != "" && ownValue == value {
		if isUpdate {
			return fmt.Sprintf("🆕 Изменено расписание %s\n", phrase)
		}
		return fmt.Sprintf("📢 Расписание на %s\n", phrase)
	}
	label := valueLabel(kind, value)
	if isUpdate {
		return fmt.Sprintf("🆕 %s: изменено расписание %s\n", label, phrase)
	}
	return fmt.Sprintf("📢 %s: расписание на %s\n", label, phrase)
}

func (n *EventNotifier) formatDay(kind, value string, day map[string]any, chat *EventChat) string {
	days := []map[string]any{day}
	opts := formatter.FormatOptions{
		ShowHeader:   false,
		IsTelegram:   true,
		TeacherNames: n.cache.GetTeamNames(),
	}
	if kind == cache.KindTeachers {
		return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(value, days, opts)
	}
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(value, days, opts)
}

func hasAlertableLessons(day map[string]any, filters []config.LessonFilter) bool {
	lessons, _ := day["lessons"].([]any)
	for _, l := range lessons {
		var items []any
		if arr, ok := l.([]any); ok {
			items = arr
		} else if l != nil {
			items = []any{l}
		}
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			lesson, _ := m["lesson"].(string)
			typ, _ := m["type"].(string)

			valid := true
			for _, f := range filters {
				if f.Lesson == lesson && f.Type == typ {
					valid = false
					break
				}
			}
			if valid {
				return true
			}
		}
	}
	return false
}

func (n *EventNotifier) mergeChats(base, extra []*EventChat) []*EventChat {
	if len(extra) == 0 {
		return base
	}
	known := make(map[int64]bool, len(base))
	for _, c := range base {
		known[c.ID] = true
	}
	for _, c := range extra {
		if !known[c.ID] {
			base = append(base, c)
		}
	}
	return base
}

func (n *EventNotifier) sendDay(kind, value string, isUpdate bool, phrase string, day map[string]any, chats []*EventChat) {
	for _, chat := range chats {
		if chat.PeerID != 0 {
			chat.ID = chat.PeerID
		}
		own := chat.Group
		if kind == cache.KindTeachers {
			own = chat.Teacher
		}
		header := n.dayHeader(isUpdate, phrase, kind, value, own)
		msg := header + "\n" + n.formatDay(kind, value, day, chat)
		if err := n.sender.SendText(chat.ID, msg); err != nil {
			n.log.Error().Err(err).Int64("chatID", chat.ID).Msg("day notification send failed")
		}
	}
}

func (n *EventNotifier) AddDay(ev *cache.DayEvent) {
	dayIdx := dayIndex(dayString(ev.Day))
	if last := n.cache.LastNoticedDay(ev.Kind, ev.Value); last != 0 && dayIdx <= int(last) {
		return
	}
	n.cache.SetLastNoticedDay(ev.Kind, ev.Value, int64(dayIdx))

	if !hasAlertableLessons(ev.Day, n.filtersFor(ev.Kind)) {
		return
	}

	kindLower := "group"
	if ev.Kind == cache.KindTeachers {
		kindLower = "teacher"
	}

	var base, subs []*EventChat
	var err error
	if kindLower == "teacher" {
		base, err = n.chats.FindChatsByTeachers("telegram", []string{ev.Value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByTeacher("telegram", ev.Value, true)
		}
	} else {
		base, err = n.chats.FindChatsByGroups("telegram", []string{ev.Value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByGroup("telegram", ev.Value, true)
		}
	}
	if err != nil {
		n.log.Error().Err(err).Msg("failed to find chats for day event")
		return
	}

	chats := n.mergeChats(base, subs)
	if len(chats) == 0 {
		return
	}

	phrase := getDayPhrase(dayString(ev.Day), "следующий день")
	n.sendDay(ev.Kind, ev.Value, false, phrase, ev.Day, chats)
}

func (n *EventNotifier) UpdateDay(ev *cache.DayEvent) {
	if !hasAlertableLessons(ev.Day, n.filtersFor(ev.Kind)) {
		return
	}

	kindLower := "group"
	if ev.Kind == cache.KindTeachers {
		kindLower = "teacher"
	}

	var base, subs []*EventChat
	var err error
	if kindLower == "teacher" {
		base, err = n.chats.FindChatsByTeachers("telegram", []string{ev.Value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByTeacher("telegram", ev.Value, true)
		}
	} else {
		base, err = n.chats.FindChatsByGroups("telegram", []string{ev.Value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByGroup("telegram", ev.Value, true)
		}
	}
	if err != nil {
		n.log.Error().Err(err).Msg("failed to find chats for day update event")
		return
	}

	chats := n.mergeChats(base, subs)
	if len(chats) == 0 {
		return
	}

	phrase := getDayPhrase(dayString(ev.Day), "день")
	n.sendDay(ev.Kind, ev.Value, true, phrase, ev.Day, chats)
}

func (n *EventNotifier) filtersFor(kind string) []config.LessonFilter {
	if kind == cache.KindTeachers {
		return n.cfg.Parser.AlertableIgnoreFilter.Teacher
	}
	return n.cfg.Parser.AlertableIgnoreFilter.Group
}

func dayString(day map[string]any) string {
	s, _ := day["day"].(string)
	return s
}

func (n *EventNotifier) CronDayAll(index int) {
	n.CronDay(cache.KindGroups, index, false)
	n.CronDay(cache.KindTeachers, index, false)
}

func (n *EventNotifier) CronDay(kind string, index int, latest bool) {
	var timetable map[string]any
	if kind == cache.KindTeachers {
		timetable = n.cache.GetTeachers()
	} else {
		timetable = n.cache.GetGroups()
	}

	var values []string
	for value, v := range timetable {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		todayDay, lessonsLen := todayDayOf(entry)
		if todayDay == nil {
			continue
		}

		if latest {
			if lessonsLen >= index+1 {
				values = append(values, value)
			}
		} else {
			if lessonsLen == index+1 {
				values = append(values, value)
			}
		}
		if lessonsLen == 0 && index+1 == n.cfg.Parser.LessonIndexIfEmpty {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return
	}

	var chats []*EventChat
	var err error
	if kind == cache.KindTeachers {
		chats, err = n.chats.FindChatsByTeachers("telegram", values, true)
	} else {
		chats, err = n.chats.FindChatsByGroups("telegram", values, true)
	}
	if err != nil || len(chats) == 0 {
		return
	}

	keyed := make(map[string][]*EventChat)
	for _, chat := range chats {
		key := chat.Group
		if kind == cache.KindTeachers {
			key = chat.Teacher
		}
		keyed[key] = append(keyed[key], chat)
	}

	for value, groupChats := range keyed {
		entry, ok := timetable[value].(map[string]any)
		if !ok {
			continue
		}

		nextDays := futureDays(entry)
		if len(nextDays) == 0 {
			continue
		}

		isEmpty := true
		for _, d := range nextDays {
			lessons, _ := d["lessons"].([]any)
			if len(lessons) > 0 {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			continue
		}

		day := nextDays[0]
		dayIdx := dayIndex(dayString(day))
		if last := n.cache.LastNoticedDay(kind, value); last != 0 && dayIdx <= int(last) {
			continue
		}

		n.cache.SetLastNoticedDay(kind, value, int64(dayIdx))

		phrase := getDayPhrase(dayString(day), "следующий день")

		for _, chat := range groupChats {
			if chat.PeerID != 0 {
				chat.ID = chat.PeerID
			}
			own := chat.Group
			if kind == cache.KindTeachers {
				own = chat.Teacher
			}
			header := n.dayHeader(false, phrase, kind, value, own)
			msg := header + "\n" + n.formatDay(kind, value, day, chat)
			if err := n.sender.SendText(chat.ID, msg); err != nil {
				n.log.Error().Err(err).Int64("chatID", chat.ID).Msg("cron day notification failed")
			}
		}
	}
}

func todayDayOf(entry map[string]any) (map[string]any, int) {
	days, _ := entry["days"].([]any)
	todayIdx := utils.DayIndexFromDate(time.Now())

	var today map[string]any
	maxLessons := 0
	for _, d := range days {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if dayIndex(dayString(m)) == todayIdx {
			today = m
		}
		lessons, _ := m["lessons"].([]any)
		if len(lessons) > maxLessons {
			maxLessons = len(lessons)
		}
	}
	return today, maxLessons
}

func futureDays(entry map[string]any) []map[string]any {
	days, _ := entry["days"].([]any)
	todayIdx := utils.DayIndexFromDate(time.Now())

	for i, d := range days {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if dayIndex(dayString(m)) > todayIdx {
			var result []map[string]any
			for _, dd := range days[i:] {
				if dm, ok := dd.(map[string]any); ok {
					result = append(result, dm)
				}
			}
			return result
		}
	}
	return nil
}

func dayIndex(dateStr string) int {
	t, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return 0
	}
	return utils.DayIndexFromDate(t)
}

func (n *EventNotifier) UpdateWeek(kind string, week int) {
	firstWeekDay := utils.WeekIndexFromNumber(week).FirstDayDate()

	message := "🆕 Доступно расписание на следующую неделю"

	var timetable map[string]any
	if kind == cache.KindTeachers {
		timetable = n.cache.GetTeachers()
	} else {
		timetable = n.cache.GetGroups()
	}

	var values []string
	for value, v := range timetable {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		days, _ := entry["days"].([]any)
		for _, d := range days {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			date, _ := m["day"].(string)
			t, err := time.Parse("02.01.2006", date)
			if err != nil {
				continue
			}
			lessons, _ := m["lessons"].([]any)
			if !t.Before(firstWeekDay) && len(lessons) > 0 {
				values = append(values, value)
				break
			}
		}
	}
	if len(values) == 0 {
		return
	}
	sort.Strings(values)

	var base []*EventChat
	var err error
	if kind == cache.KindTeachers {
		base, err = n.chats.FindChatsByTeachers("telegram", values, false)
	} else {
		base, err = n.chats.FindChatsByGroups("telegram", values, false)
	}
	if err != nil {
		return
	}

	baseChats := make([]*EventChat, 0, len(base))
	for _, c := range base {
		if c.NoticeNextWeek {
			baseChats = append(baseChats, c)
		}
	}

	baseIDs := make(map[int64]bool, len(baseChats))
	for _, chat := range baseChats {
		baseIDs[chat.ID] = true

		var btn *KeyboardButton
		own := chat.Group
		if kind == cache.KindTeachers {
			own = chat.Teacher
		}
		if own != "" {
			data := fmt.Sprintf(getWeekTimetableCallback, kindLetter(kind), own, week)
			btn = &KeyboardButton{Text: "📃 Показать", Data: data}
		}
		if chat.PeerID != 0 {
			chat.ID = chat.PeerID
		}
		if btn != nil {
			n.sender.SendTextWithButtons(chat.ID, message, []KeyboardButton{*btn})
		} else {
			n.sender.SendText(chat.ID, message)
		}
	}

	for _, value := range values {
		var subs []*EventChat
		if kind == cache.KindTeachers {
			subs, err = n.chats.FindSubscribedChatsByTeacher("telegram", value, false)
		} else {
			subs, err = n.chats.FindSubscribedChatsByGroup("telegram", value, false)
		}
		if err != nil {
			continue
		}

		scoped := fmt.Sprintf("🆕 %s: доступно расписание на следующую неделю", valueLabel(kind, value))
		data := fmt.Sprintf(getWeekTimetableCallback, kindLetter(kind), value, week)
		btn := KeyboardButton{Text: "📃 Показать", Data: data}

		for _, chat := range subs {
			if !chat.NoticeNextWeek || baseIDs[chat.ID] {
				continue
			}
			if chat.PeerID != 0 {
				chat.ID = chat.PeerID
			}
			n.sender.SendTextWithButtons(chat.ID, scoped, []KeyboardButton{btn})
		}
	}
}

func valueLabel(kind, value string) string {
	if kind == cache.KindTeachers {
		return "Преподаватель " + value
	}
	return "Группа " + value
}

func kindLetter(kind string) string {
	if kind == cache.KindTeachers {
		return "t"
	}
	return "g"
}

func (n *EventNotifier) WithdrawWeek(kind string, week int, entries []string) {
	if len(entries) == 0 {
		return
	}

	infoLine := "Расписание ещё не окончательно и может измениться."
	baseMessage := fmt.Sprintf("❌ Расписание на следующую неделю отозвано.\n%s", infoLine)

	var base []*EventChat
	var err error
	if kind == cache.KindTeachers {
		base, err = n.chats.FindChatsByTeachers("telegram", entries, false)
	} else {
		base, err = n.chats.FindChatsByGroups("telegram", entries, false)
	}
	if err != nil {
		return
	}

	baseChats := make([]*EventChat, 0, len(base))
	for _, c := range base {
		if c.NoticeNextWeek {
			baseChats = append(baseChats, c)
		}
	}

	baseIDs := make(map[int64]bool, len(baseChats))
	for _, chat := range baseChats {
		baseIDs[chat.ID] = true
		if chat.PeerID != 0 {
			chat.ID = chat.PeerID
		}
		n.sender.SendText(chat.ID, baseMessage)
	}

	for _, value := range entries {
		var subs []*EventChat
		if kind == cache.KindTeachers {
			subs, err = n.chats.FindSubscribedChatsByTeacher("telegram", value, false)
		} else {
			subs, err = n.chats.FindSubscribedChatsByGroup("telegram", value, false)
		}
		if err != nil {
			continue
		}

		scoped := fmt.Sprintf("❌ %s: расписание на следующую неделю отозвано.\n%s", valueLabel(kind, value), infoLine)
		for _, chat := range subs {
			if !chat.NoticeNextWeek || baseIDs[chat.ID] {
				continue
			}
			if chat.PeerID != 0 {
				chat.ID = chat.PeerID
			}
			n.sender.SendText(chat.ID, scoped)
		}
	}
}

func (n *EventNotifier) CallsChanged(ev *cache.CallsEvent) {
	if !ev.WeekdaysChanged && !ev.SaturdayChanged {
		return
	}

	chats, err := n.chats.FindChatsWithNotice("telegram", "notice_calls")
	if err != nil {
		return
	}
	chats = filterConfiguredChats(chats)
	if len(chats) == 0 {
		return
	}

	parts := []string{"🔔 Изменено расписание звонков"}
	if ev.Reason != "" {
		parts = append(parts, fmt.Sprintf("Причина: %s", ev.Reason))
	}

	if ev.WeekdaysChanged {
		parts = append(parts, "\n__ Звонки (будни) __")
		parts = append(parts, formatCallsLines(ev.Schedule.Weekdays))
	}
	if ev.SaturdayChanged {
		parts = append(parts, "\n__ Звонки (суббота) __")
		parts = append(parts, formatCallsLines(ev.Schedule.Saturday))
	}
	parts = append(parts, "\nПолное расписание: нажмите Показать")

	message := strings.Join(parts, "\n")
	for _, chat := range chats {
		if chat.PeerID != 0 {
			chat.ID = chat.PeerID
		}
		if err := n.sender.SendTextWithButtons(chat.ID, message, []KeyboardButton{{Text: "📊 Показать", Data: getCallsCallback}}); err != nil {
			n.log.Error().Err(err).Int64("chatID", chat.ID).Msg("calls notification failed")
		}
	}
}

func filterConfiguredChats(chats []*EventChat) []*EventChat {
	var result []*EventChat
	for _, c := range chats {
		if c.Group != "" || c.Teacher != "" {
			result = append(result, c)
		}
	}
	return result
}

func formatCallsLines(calls [][2][2]string) string {
	var lines []string
	for i, lesson := range calls {
		lines = append(lines, fmt.Sprintf("%d. %s - %s | %s - %s", i+1, lesson[0][0], lesson[0][1], lesson[1][0], lesson[1][1]))
	}
	return strings.Join(lines, "\n")
}

func (n *EventNotifier) ParserError(err error) {
	base, e := n.chats.FindChatsWithNotice("telegram", "notice_parser_errors")
	if e != nil {
		return
	}
	admins, e := n.chats.FindAdminChats("telegram")
	if e != nil {
		return
	}
	chats := n.mergeChats(base, admins)
	if len(chats) == 0 {
		return
	}

	message := "Parser error\n" + err.Error()
	for _, chat := range chats {
		if chat.PeerID != 0 {
			chat.ID = chat.PeerID
		}
		n.sender.SendText(chat.ID, message)
	}
}

func (n *EventNotifier) HandleEvents(events []cache.Event) {
	for _, ev := range events {
		if ev.Day != nil {
			if ev.Day.Type == cache.DayEventUpdate {
				n.UpdateDay(ev.Day)
			} else {
				n.AddDay(ev.Day)
			}
		}
		if ev.Week != nil {
			if ev.Week.Withdrawn {
				n.WithdrawWeek(ev.Week.Kind, ev.Week.Week, ev.Week.Entries)
			} else {
				n.UpdateWeek(ev.Week.Kind, ev.Week.Week)
			}
		}
		if ev.Calls != nil {
			n.CallsChanged(ev.Calls)
		}
	}
}
