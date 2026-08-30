package telegram

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

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

func (c *debugCmd) Name() string        { return "/debug" }
func (c *debugCmd) Description() string { return c.bot.loc("cmd_debug") }

func (c *debugCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	var lines []string

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
	lines = append(lines, fmt.Sprintf("Uptime: %s", formatUptime(c.bot.startTime)))
	lines = append(lines, fmt.Sprintf("Команд: %d", len(c.bot.commands)))
	lines = append(lines, fmt.Sprintf("Callback'ов: %d", len(c.bot.callbacks)))

	lines = append(lines, "")
	lines = append(lines, "-- Кеш --")
	stats := c.bot.cache.Stats()
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
	total, err := c.bot.chatRepo.CountAll()
	if err == nil {
		lines = append(lines, fmt.Sprintf("Всего чатов: %d", total))
	}
	notifyCount := 0
	notifyChats, err := c.bot.chatRepo.FindAllWithNotifications("telegram")
	if err == nil {
		notifyCount = len(notifyChats)
	}
	lines = append(lines, fmt.Sprintf("С уведомлениями: %d", notifyCount))
	modes, err := c.bot.chatRepo.CountByMode()
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
	lines = append(lines, fmt.Sprintf("Parser v2: %v", c.bot.cfg.Parser.V2 != nil && c.bot.cfg.Parser.V2.Enabled))
	lines = append(lines, fmt.Sprintf("Telegram noticer: %v", c.bot.cfg.Telegram.Noticer))

	return u.Bot.SendText(u.ChatID, "<pre>"+strings.Join(lines, "\n")+"</pre>")
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

func (c *sendCmd) Name() string        { return "/send" }
func (c *sendCmd) Description() string { return c.bot.loc("cmd_send") }

func (c *sendCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	text := strings.TrimPrefix(u.Text, "/send")
	text = strings.TrimSpace(text)
	if text == "" {
		return u.Bot.SendText(u.ChatID, "Введите текст сообщения после /send")
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
		Text:      fmt.Sprintf("📤 Рассылка: 0/%d (0.00%%)", len(chats)),
		ParseMode: "HTML",
	})
	if err != nil {
		return err
	}

	sent := 0
	failed := 0
	total := len(chats)
	rateLimiter := time.NewTicker(2400 * time.Millisecond)
	defer rateLimiter.Stop()

	lastEdit := time.Now()

	for i, chat := range chats {
		if err := c.bot.SendText(chat.PeerID, text); err != nil {
			failed++
		} else {
			sent++
		}

		if time.Since(lastEdit) >= 1*time.Second || i == total-1 {
			pct := float64(i+1) / float64(total) * 100
			editText := fmt.Sprintf("📤 tg: %d/%d (%.2f%%)", i+1, total, pct)
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

		if i < total-1 {
			<-rateLimiter.C
		}
	}

	result := fmt.Sprintf("✅ Успешно отправлено %d из %d", sent, total)
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

func (c *triggerCmd) Name() string        { return "/trigger" }
func (c *triggerCmd) Description() string { return c.bot.loc("cmd_trigger") }

func (c *triggerCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	if c.bot.parseFunc == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("parse_not_available"))
	}

	go func() {
		if err := c.bot.parseFunc(); err != nil {
			c.bot.log.Error().Err(err).Msg("trigger parse error")
			c.bot.SendText(u.ChatID, c.bot.loc("force_parse_error"))
			return
		}
		c.bot.SendText(u.ChatID, c.bot.loc("force_parse_done"))
	}()

	return u.Bot.SendText(u.ChatID, c.bot.loc("force_parse_started"))
}
