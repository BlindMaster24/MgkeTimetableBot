package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

type callsFullCb struct{ bot *Bot }

func (cb *callsFullCb) Prefix() string { return "calls_full" }
func (cb *callsFullCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return withChat(cb.bot, u, func(chat *Chat) error {
		cb.bot.showCallsFullFull(u, chat)
		return nil
	})
}

type imageCb struct{ bot *Bot }

func (cb *imageCb) Prefix() string { return "image" }
func (cb *imageCb) Handler(ctx context.Context, u *Update) error {
	payload := strings.TrimPrefix(u.Data, "image")
	payload = strings.TrimPrefix(payload, "_")
	if idx := strings.IndexByte(payload, ':'); idx >= 0 {
		typePart := payload[:idx]
		value := payload[idx+1:]
		switch typePart {
		case "g", "group":
			cb.bot.AnswerCallback(u.Callback.ID, "")
			err := cb.bot.handleImagePayload(u, "g", value)
			if err == nil {
				cb.bot.AnswerCallback(u.Callback.ID, "Изображение было отправлено")
			}
			return err
		case "t", "teacher":
			cb.bot.AnswerCallback(u.Callback.ID, "")
			err := cb.bot.handleImagePayload(u, "t", value)
			if err == nil {
				cb.bot.AnswerCallback(u.Callback.ID, "Изображение было отправлено")
			}
			return err
		}
	}

	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	if chat.Mode == ModeStudent || chat.Mode == ModeParent {
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
		}
		data, ok := cb.bot.cache.GetGroups()[chat.Group]
		if !ok {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("group_not_exists"))
		}
		path, err := imagepkg.RenderGroupFromCache(chat.Group, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}
	if chat.Mode == "teacher" {
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("need_teacher"))
		}
		data, ok := cb.bot.cache.GetTeachers()[chat.Teacher]
		if !ok {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("teacher_not_exists"))
		}
		path, err := imagepkg.RenderTeacherFromCache(chat.Teacher, data, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, cb.bot.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type imageGroupCb struct{ bot *Bot }

func (b *Bot) handleImagePayload(u *Update, typeLetter, value string) error {
	weekIndex := b.relevantWeekIndex().Value()
	if idx := strings.LastIndexByte(value, ':'); idx >= 0 {
		if w, err := strconv.Atoi(value[idx+1:]); err == nil {
			weekIndex = w
			value = value[:idx]
		}
	}

	minIdx, maxIdx := utils.WeekIndexFromNumber(weekIndex).WeekDayIndexRange()

	var days []map[string]any
	switch typeLetter {
	case "g":
		if _, ok := b.cache.GetGroups()[value]; !ok {
			return u.Bot.SendText(u.ChatID, b.loc("group_not_exists"))
		}
		days = b.archiveDaysForWeek("group", value, minIdx, maxIdx)
		if len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Нет расписания для отображения")
		}
		path, err := imagepkg.RenderGroupDays(value, days, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	case "t":
		if _, ok := b.cache.GetTeachers()[value]; !ok {
			return u.Bot.SendText(u.ChatID, b.loc("teacher_not_exists"))
		}
		days = b.archiveDaysForWeek("teacher", value, minIdx, maxIdx)
		if len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Нет расписания для отображения")
		}
		path, err := imagepkg.RenderTeacherDays(value, days, "./cache/images")
		if err != nil {
			return u.Bot.SendText(u.ChatID, b.loc("image_failed"))
		}
		return u.Bot.SendPhoto(u.ChatID, path, "")
	}
	return u.Bot.SendText(u.ChatID, b.loc("no_timetable"))
}

type cancelCb struct{ bot *Bot }

func (cb *cancelCb) Prefix() string { return "cancel" }
func (cb *cancelCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		wasSetup := chat.Scene == sceneSetup
		chat.Scene = ""
		cb.bot.chatRepo.Save(chat)
		if wasSetup {
			return cb.bot.SendTextWithReplyKeyboard(u.ChatID, cb.bot.loc("about_bot"), replyMainMenu(cb.bot, chat))
		}
	}
	return u.Bot.SendText(u.ChatID, cb.bot.loc("input_cancelled"))
}

type answerCb struct{ bot *Bot }

func (cb *answerCb) Prefix() string { return "answer:" }
func (cb *answerCb) Handler(ctx context.Context, u *Update) error {
	answer := strings.TrimPrefix(u.Data, "answer:")
	cb.bot.AnswerCallback(u.Callback.ID, fmt.Sprintf("Выбрано: \"%s\"", answer))
	if _, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID); err != nil {
		return nil
	}
	u.Text = answer
	cb.bot.handleMessageText(ctx, u)
	return nil
}

func callsFullKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Показать полностью", CallbackData: "calls_full"}},
		},
	}
}

func (b *Bot) showCallsFull(u *Update, chat *Chat) {
	b.showCalls(u, chat, false)
}

func (b *Bot) showCallsFullFull(u *Update, chat *Chat) {
	b.showCalls(u, chat, true)
}

func (b *Bot) showCalls(u *Update, chat *Chat, full bool) {
	schedule := b.callsScheduleFor(chat)
	activeWeekdays := schedule.Weekdays
	activeSaturday := schedule.Saturday

	maxLessons := len(activeWeekdays)
	if len(activeSaturday) > maxLessons {
		maxLessons = len(activeSaturday)
	}

	userMax := maxLessons
	current := countCurrentLessons(chat, b.cache)
	if !full && current > 0 {
		userMax = current
	}
	if current > 0 && current >= maxLessons {
		full = true
	}

	var msg []string
	calls := b.cache.GetCalls()
	if calls.Active.Source == "manual" && calls.ManualReason != "" {
		msg = append(msg, fmt.Sprintf("Причина: %s\n", calls.ManualReason))
	}
	if line := b.callsCampusLine(chat); line != "" {
		msg = append(msg, line)
	}
	msg = append(msg, "__ <b>Звонки (будни)</b> __")
	msg = append(msg, b.callsLines(activeWeekdays, userMax, full, []int{1, 2, 3, 4, 5}))
	msg = append(msg, "\n__ <b>Звонки (суббота)</b> __")
	msg = append(msg, b.callsLines(activeSaturday, userMax, full, []int{6}))

	text := strings.Join(msg, "\n")
	if !full {
		b.sendOrEdit(u, text, callsFullKeyboard())
		return
	}

	b.sendOrEdit(u, text, nil)
}

func countCurrentLessons(chat *Chat, c *cache.RaspCache) int {
	var data any
	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return 0
		}
		data, _ = c.GetGroups()[chat.Group]
	case ModeTeacher:
		if chat.Teacher == "" {
			return 0
		}
		data, _ = c.GetTeachers()[chat.Teacher]
	}
	if data == nil {
		return 0
	}
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return 0
	}
	maxLessons := 0
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		lessons, ok := dayMap["lessons"].([]any)
		if !ok {
			continue
		}
		if len(lessons) > maxLessons {
			maxLessons = len(lessons)
		}
	}
	if maxLessons == 0 {
		return 0
	}
	return maxLessons
}

func (b *Bot) callsLines(slots [][2][2]string, maxLessons int, showFull bool, includedDays []int) string {
	now := time.Now()

	var lines []string
	for i := 0; i < maxLessons; i++ {
		if i >= len(slots) {
			break
		}
		slot := slots[i]
		line := fmt.Sprintf("%d. %s - %s | %s - %s", i+1, slot[0][0], slot[0][1], slot[1][0], slot[1][1])

		if !showFull && isNowInSlot(now, slot, includedDays) {
			line = "👉 " + line + " 👈"
		}

		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

type reparseCb struct{ bot *Bot }

func (cb *reparseCb) Prefix() string { return notification.ParserReparseCallback }

func (cb *reparseCb) Handler(ctx context.Context, u *Update) error {
	if u.Callback != nil {
		cb.bot.AnswerCallback(u.Callback.ID, "")
	}
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if cb.bot.parseFunc == nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("parse_not_available"))
	}

	chatID := u.ChatID
	if err := u.Bot.SendText(chatID, cb.bot.loc("force_parse_started")); err != nil {
		return err
	}

	go func() {
		if err := cb.bot.parseFunc(); err != nil {
			cb.bot.log.Error().Err(err).Msg("manual reparse failed")
			cb.bot.SendText(chatID, cb.bot.loc("force_parse_error"))
			return
		}
		cb.bot.markIncidentFix(health.ScopeParser, cb.bot.loc("incident_fix_reparse"))
		cb.bot.SendText(chatID, cb.bot.loc("force_parse_done"))
	}()

	return nil
}

type calendarSyncCb struct{ bot *Bot }

func (cb *calendarSyncCb) Prefix() string { return notification.CalendarSyncCallback }

func (cb *calendarSyncCb) Handler(ctx context.Context, u *Update) error {
	if u.Callback != nil {
		cb.bot.AnswerCallback(u.Callback.ID, "")
	}
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if cb.bot.calendarSync == nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("calendar_sync_unavailable"))
	}

	chatID := u.ChatID
	if err := u.Bot.SendText(chatID, cb.bot.loc("calendar_sync_started")); err != nil {
		return err
	}

	go func() {
		days, err := cb.bot.calendarSync(context.Background())
		if err != nil {
			cb.bot.log.Error().Err(err).Msg("manual calendar sync failed")
			cb.bot.SendText(chatID, cb.bot.locData("calendar_sync_error", map[string]interface{}{"Error": err.Error()}))
			return
		}
		cb.bot.markIncidentFix(health.ScopeCalendar, cb.bot.loc("incident_fix_calendar"))
		cb.bot.SendText(chatID, cb.bot.locData("calendar_sync_done", map[string]interface{}{"Days": days}))
	}()

	return nil
}

type apiProbeCb struct{ bot *Bot }

func (cb *apiProbeCb) Prefix() string { return notification.APIProbeCallback }

func (cb *apiProbeCb) Handler(ctx context.Context, u *Update) error {
	if u.Callback != nil {
		cb.bot.AnswerCallback(u.Callback.ID, "")
	}
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if cb.bot.apiProbe == nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("api_probe_unavailable"))
	}

	chatID := u.ChatID
	if err := u.Bot.SendText(chatID, cb.bot.loc("api_probe_started")); err != nil {
		return err
	}

	go func() {
		results := cb.bot.apiProbe(context.Background())
		cb.bot.SendText(chatID, cb.bot.apiProbeText(results))
		if apiprobe.Healthy(results) {
			cb.bot.markIncidentFix(health.ScopeAPI, cb.bot.locData("incident_fix_api", map[string]interface{}{
				"Healthy": apiprobe.HealthyCount(results),
				"Total":   len(results),
			}))
		}
	}()

	return nil
}

func (b *Bot) apiProbeText(results []apiprobe.Result) string {
	if len(results) == 0 {
		return b.loc("api_probe_empty")
	}

	width := 0
	for _, result := range results {
		if label := result.Label(); len(label) > width {
			width = len(label)
		}
	}

	slow := b.slowThreshold()
	rows := make([]string, 0, len(results))
	for _, result := range results {
		status := "—"
		if result.Err == "" {
			status = strconv.Itoa(result.Status)
		}
		row := fmt.Sprintf("%-*s %3s %8s", width, result.Label(), status, apiDuration(result.Duration))
		rows = append(rows, row+" "+apiProbeMark(result, slow))
	}

	lines := []string{b.loc("api_probe_header"), "<code>" + strings.Join(rows, "\n") + "</code>"}
	lines = append(lines, b.locData("api_probe_summary", map[string]interface{}{
		"Healthy": apiprobe.HealthyCount(results),
		"Total":   len(results),
		"Slowest": apiProbeSlowest(results),
	}))

	if failed := apiprobe.Failed(results); len(failed) > 0 {
		labels := make([]string, 0, len(failed))
		for _, result := range failed {
			labels = append(labels, apiprobe.Describe(result))
		}
		lines = append(lines, b.locData("api_probe_failed", map[string]interface{}{"Failed": strings.Join(labels, "; ")}))
	}

	if slow > 0 {
		lines = append(lines, b.locData("api_probe_slow_note", map[string]interface{}{"Threshold": apiDuration(slow)}))
	}
	return strings.Join(lines, "\n")
}

func apiProbeMark(result apiprobe.Result, slow time.Duration) string {
	if !result.Healthy() {
		return "⚠️"
	}
	if slow > 0 && result.Duration >= slow {
		return "🐌"
	}
	return "✅"
}

func apiProbeSlowest(results []apiprobe.Result) string {
	slowest, ok := apiprobe.Slowest(results)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s — %s", slowest.Label(), apiDuration(slowest.Duration))
}

func apiDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%d мс", value.Milliseconds())
	}
	return fmt.Sprintf("%.2f с", value.Seconds())
}

func (b *Bot) slowThreshold() time.Duration {
	if b.health == nil {
		return 0
	}
	return b.health.SlowThreshold()
}

func isNowInSlot(now time.Time, slot [2][2]string, includedDays []int) bool {
	dayIncluded := false
	for _, d := range includedDays {
		if int(now.Weekday()) == d {
			dayIncluded = true
			break
		}
	}
	if !dayIncluded {
		return false
	}
	startParts := strings.Split(slot[0][0], ":")
	endParts := strings.Split(slot[1][1], ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		return false
	}

	startH, startM := 0, 0
	fmt.Sscanf(startParts[0], "%d", &startH)
	fmt.Sscanf(startParts[1], "%d", &startM)
	endH, endM := 0, 0
	fmt.Sscanf(endParts[0], "%d", &endH)
	fmt.Sscanf(endParts[1], "%d", &endM)

	startMin := startH*60 + startM
	endMin := endH*60 + endM
	nowMin := now.Hour()*60 + now.Minute()

	return nowMin >= startMin && nowMin <= endMin
}
