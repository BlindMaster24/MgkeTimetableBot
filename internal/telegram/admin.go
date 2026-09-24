package telegram

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/mymmrac/telego"
)

type adminChecker interface {
	isAdmin(userID int64) bool
}

func (b *Bot) isAdmin(userID int64) bool {
	for _, id := range b.cfg.Telegram.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

type debugCmd struct{ bot *Bot }

func (c *debugCmd) AdminOnly() bool { return true }

func (c *debugCmd) Name() string        { return "/debug" }
func (c *debugCmd) Description() string { return c.bot.loc("cmd_debug") }

func (c *debugCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	return u.Bot.SendText(u.ChatID, "<pre>"+strings.Join(c.bot.debugLines(), "\n")+"</pre>")
}

func (b *Bot) debugLines() []string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	var lines []string

	info := b.buildInfo
	lines = append(lines, "-- Сборка --")
	lines = append(lines, fmt.Sprintf("Версия: %s", info.Version))
	lines = append(lines, fmt.Sprintf("Коммит: %s", info.ShortCommit()))
	lines = append(lines, fmt.Sprintf("Собрано: %s", info.Date))
	if info.Go != "" {
		lines = append(lines, fmt.Sprintf("Toolchain: %s %s/%s", info.Go, info.OS, info.Arch))
	}

	lines = append(lines, "")
	lines = append(lines, "-- Система --")
	lines = append(lines, fmt.Sprintf("ОС: %s (%s)", runtime.GOOS, runtime.GOARCH))
	lines = append(lines, fmt.Sprintf("Go: %s", runtime.Version()))
	lines = append(lines, fmt.Sprintf("CPU: %d", runtime.NumCPU()))
	lines = append(lines, fmt.Sprintf("GOMAXPROCS: %d", runtime.GOMAXPROCS(0)))

	lines = append(lines, "")
	lines = append(lines, "-- Runtime --")
	lines = append(lines, fmt.Sprintf("Goroutines: %d", runtime.NumGoroutine()))
	lines = append(lines, fmt.Sprintf("Heap Alloc: %s", formatBytes(m.HeapAlloc)))
	lines = append(lines, fmt.Sprintf("Heap Sys: %s", formatBytes(m.HeapSys)))
	lines = append(lines, fmt.Sprintf("Heap Inuse: %s", formatBytes(m.HeapInuse)))
	lines = append(lines, fmt.Sprintf("Stack Inuse: %s", formatBytes(m.StackInuse)))
	lines = append(lines, fmt.Sprintf("Sys: %s", formatBytes(m.Sys)))
	lines = append(lines, fmt.Sprintf("Total Alloc: %s", formatBytes(m.TotalAlloc)))
	lines = append(lines, fmt.Sprintf("NumGC: %d", m.NumGC))
	lines = append(lines, fmt.Sprintf("GCCPUFraction: %.4f", m.GCCPUFraction))
	if gc.PauseTotal > 0 {
		lines = append(lines, fmt.Sprintf("GC Pause Total: %s", gc.PauseTotal))
	}

	lines = append(lines, "")
	lines = append(lines, "-- Бот --")
	lines = append(lines, fmt.Sprintf("PID: %d", os.Getpid()))
	lines = append(lines, fmt.Sprintf("Uptime: %s", formatUptime(b.startTime)))
	lines = append(lines, fmt.Sprintf("Команд: %d", len(b.commands)))
	lines = append(lines, fmt.Sprintf("Callback'ов: %d", len(b.callbacks)))

	lines = append(lines, "")
	lines = append(lines, "-- Кеш --")
	stats := b.cache.Stats()
	lines = append(lines, fmt.Sprintf("Групп: %d", stats.GroupsCount))
	lines = append(lines, fmt.Sprintf("Преподавателей: %d", stats.TeachersCount))
	lines = append(lines, fmt.Sprintf("Хиты/Промахи: %d/%d", stats.Hits, stats.Misses))
	lines = append(lines, fmt.Sprintf("SuccessUpdate: %v", stats.SuccessUpdate))
	if stats.GroupsUpdate > 0 {
		lines = append(lines, fmt.Sprintf("Группы обновлены: %s", time.UnixMilli(stats.GroupsUpdate).Format("02.01.2006 15:04")))
	}
	if stats.TeachersUpdate > 0 {
		lines = append(lines, fmt.Sprintf("Преподаватели обновлены: %s", time.UnixMilli(stats.TeachersUpdate).Format("02.01.2006 15:04")))
	}

	lines = append(lines, "")
	lines = append(lines, "-- База данных --")
	total, err := b.chatRepo.CountAll()
	if err == nil {
		lines = append(lines, fmt.Sprintf("Всего чатов: %d", total))
	}
	tgTotal, tgAllowed, err := b.chatRepo.CountTGChats()
	if err == nil {
		lines = append(lines, fmt.Sprintf("Чатов бота Telegram: %d (allow: %d)", tgTotal, tgAllowed))
	}
	if b.keys != nil {
		if keys, active, err := b.keys.Counts(); err == nil {
			lines = append(lines, fmt.Sprintf("API ключей: %d (active: %d)", keys, active))
		}
	}
	notifyCount := 0
	notifyChats, err := b.chatRepo.FindAllWithNotifications("telegram")
	if err == nil {
		notifyCount = len(notifyChats)
	}
	lines = append(lines, fmt.Sprintf("С уведомлениями: %d", notifyCount))
	modes, err := b.chatRepo.CountByMode()
	if err == nil {
		modeOrder := []string{"student", "teacher", "parent", "guest", "none"}
		for _, mode := range modeOrder {
			if count, ok := modes[mode]; ok && count > 0 {
				lines = append(lines, fmt.Sprintf("  %s: %d", mode, count))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, "-- Конфиг --")
	lines = append(lines, fmt.Sprintf("Telegram noticer: %v", b.cfg.Telegram.Noticer))

	return lines
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatUptime(start time.Time) string {
	d := time.Since(start)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

type sendCmd struct{ bot *Bot }

func (c *sendCmd) AdminOnly() bool { return true }

func (c *sendCmd) Name() string        { return "/send" }
func (c *sendCmd) Description() string { return c.bot.loc("cmd_send") }

func (c *sendCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	text := strings.TrimPrefix(u.Text, "/send")
	text = strings.TrimSpace(text)
	if text == "" {
		return u.Bot.SendText(u.ChatID, "Сообщение не введено")
	}

	chats, err := c.bot.chatRepo.FindAllTGChats()
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка получения чатов: "+err.Error())
	}

	if len(chats) == 0 {
		return u.Bot.SendText(u.ChatID, "Нет активных чатов для рассылки")
	}

	progressMsg, err := c.bot.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: u.ChatID},
		Text:      "Начинаю отправку сообщений.",
		ParseMode: "HTML",
	})
	if err != nil {
		return err
	}

	sent := 0
	failed := 0
	total := len(chats)
	sentTimestamps := make([]time.Time, 0, 25)
	lastEdit := time.Now()

	for i, chat := range chats {
		now := time.Now()
		if len(sentTimestamps) >= 25 {
			oldest := sentTimestamps[0]
			waitUntil := oldest.Add(60 * time.Second)
			if waitUntil.After(now) {
				time.Sleep(waitUntil.Sub(now))
			}
			sentTimestamps = sentTimestamps[1:]
		}

		if err := c.bot.SendText(chat.PeerID, text); err != nil {
			failed++
		} else {
			sent++
			sentTimestamps = append(sentTimestamps, time.Now())
		}

		if time.Since(lastEdit) >= 1*time.Second || i == total-1 {
			pct := float64(i+1) / float64(total) * 100
			editText := fmt.Sprintf("tg: %d/%d (%.2f%%)", i+1, total, pct)
			if failed > 0 {
				editText += fmt.Sprintf("\n❌ Ошибок: %d", failed)
			}
			c.bot.client.EditMessageText(context.Background(), &telego.EditMessageTextParams{
				ChatID:    telego.ChatID{ID: u.ChatID},
				MessageID: progressMsg.MessageID,
				Text:      editText,
			})
			lastEdit = time.Now()
		}
	}

	result := "Успешно отправлено!"
	if failed > 0 {
		result += fmt.Sprintf("\n❌ Ошибок: %d", failed)
	}

	c.bot.client.EditMessageText(context.Background(), &telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: u.ChatID},
		MessageID: progressMsg.MessageID,
		Text:      result,
	})

	return nil
}

type triggerCmd struct{ bot *Bot }

func (c *triggerCmd) AdminOnly() bool { return true }

func (c *triggerCmd) Name() string        { return "/trigger" }
func (c *triggerCmd) Description() string { return c.bot.loc("cmd_trigger") }

func (c *triggerCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(u.Text, "/trigger")))
	if len(args) == 0 {
		return u.Bot.SendText(u.ChatID, "not found")
	}

	if args[0] != "NextDayUpdater" {
		return u.Bot.SendText(u.ChatID, "not found")
	}

	if len(args) < 2 {
		return u.Bot.SendText(u.ChatID, "index is not a number")
	}

	index, err := strconv.Atoi(args[1])
	if err != nil {
		return u.Bot.SendText(u.ChatID, "index is not a number")
	}

	if c.bot.noticeDay == nil {
		return u.Bot.SendText(u.ChatID, "not found")
	}

	c.bot.noticeDay(index - 1)
	return u.Bot.SendText(u.ChatID, "ok")
}

type noticeDebugCmd struct{ bot *Bot }

func (c *noticeDebugCmd) Name() string { return "/noticedebug" }
func (c *noticeDebugCmd) Description() string {
	return "Кому придут уведомления каждого типа"
}
func (c *noticeDebugCmd) AdminOnly() bool { return true }

func (c *noticeDebugCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	lines := []string{"-- Оповещения: получатели --"}

	addSection := func(title string, chats []*Chat, format func(*Chat) string) {
		lines = append(lines, "")
		lines = append(lines, title)
		if len(chats) == 0 {
			lines = append(lines, "  (нет получателей)")
			return
		}
		for _, chat := range chats {
			lines = append(lines, "  "+format(chat))
		}
	}

	describe := func(chat *Chat) string {
		parts := []string{fmt.Sprintf("%d", chat.PeerID)}
		if chat.Mode != "" {
			parts = append(parts, string(chat.Mode))
		}
		if chat.Group != "" {
			parts = append(parts, "группа "+chat.Group)
		}
		if chat.Teacher != "" {
			parts = append(parts, "преподаватель "+chat.Teacher)
		}
		return strings.Join(parts, ", ")
	}

	groups, err := c.bot.chatRepo.FindChatsByGroups("telegram", c.bot.cache.GroupKeys(), true)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	addSection("Новое расписание (day add, notice_changes):", groups, describe)

	teachers, err := c.bot.chatRepo.FindChatsByTeachers("telegram", c.bot.cache.TeacherKeys(), true)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	addSection("Новое расписание преподавателей (notice_changes):", teachers, describe)

	subsG, err := c.bot.chatRepo.CountSubscriptionsByType("telegram", "group")
	if err == nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Подписки на группы: %d чат(ов)", subsG))
	}
	subsT, err := c.bot.chatRepo.CountSubscriptionsByType("telegram", "teacher")
	if err == nil {
		lines = append(lines, fmt.Sprintf("Подписки на преподавателей: %d чат(ов)", subsT))
	}

	nextWeek, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_next_week")
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	addSection("Новая неделя (notice_next_week):", nextWeek, describe)

	calls, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_calls")
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	configured := make([]*Chat, 0, len(calls))
	for _, chat := range calls {
		if chat.Group != "" || chat.Teacher != "" {
			configured = append(configured, chat)
		}
	}
	addSection("Изменение звонков (notice_calls, настроен режим):", configured, describe)

	errs, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_parser_errors")
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	addSection("Ошибки парсера (notice_parser_errors):", errs, describe)

	admins, err := c.bot.chatRepo.FindAdminChats("telegram", c.bot.cfg.Telegram.AdminIDs)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения чатов: "+err.Error())
	}
	addSection("Админы (всегда получают ошибки парсера):", admins, describe)

	cronChats, err := c.bot.chatRepo.FindAllWithNotifications("telegram")
	if err == nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Всего чатов с notice_changes: %d", len(cronChats)))
	}

	return u.Bot.SendText(u.ChatID, "<pre>"+strings.Join(lines, "\n")+"</pre>")
}

type archiveStatsCmd struct{ bot *Bot }

func (c *archiveStatsCmd) Name() string { return "/archivestats" }
func (c *archiveStatsCmd) Description() string {
	return "Статистика архива расписания"
}
func (c *archiveStatsCmd) AdminOnly() bool { return true }

func (c *archiveStatsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	if c.bot.archive == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	var total, withGroup, withTeacher int
	if err := c.bot.archive.DB().QueryRow("SELECT COUNT(*), COUNT(\"group\"), COUNT(teacher) FROM timetable_archive").Scan(&total, &withGroup, &withTeacher); err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения архива: "+err.Error())
	}

	var lines []string
	lines = append(lines, "-- Архив --")
	if bounds, err := c.bot.archive.DayIndexBounds(); err == nil && bounds.Max > 0 {
		lines = append(lines, fmt.Sprintf("Дни: %s — %s", archive.DayIndexToDate(bounds.Min), archive.DayIndexToDate(bounds.Max)))
		minWeek := int(bounds.Min) / 7
		maxWeek := int(bounds.Max) / 7
		lines = append(lines, fmt.Sprintf("Недели: %d — %d", minWeek, maxWeek))
	} else {
		lines = append(lines, "Дни: архив пуст")
	}
	lines = append(lines, fmt.Sprintf("Записей: %d (группы: %d, преподаватели: %d)", total, withGroup, withTeacher))

	if groups, err := c.bot.archive.Groups(); err == nil && len(groups) > 0 {
		lines = append(lines, fmt.Sprintf("Групп в архиве: %d", len(groups)))
	}
	if teachers, err := c.bot.archive.Teachers(); err == nil && len(teachers) > 0 {
		lines = append(lines, fmt.Sprintf("Преподавателей в архиве: %d", len(teachers)))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}
