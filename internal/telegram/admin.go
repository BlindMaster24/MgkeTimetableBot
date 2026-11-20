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
	"unicode/utf8"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
)

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
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_1"))
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
	lines = append(lines, b.loc("admintext_2"))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_3"), info.Version))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_4"), info.ShortCommit()))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_5"), info.Date))
	if info.Go != "" {
		lines = append(lines, fmt.Sprintf("Toolchain: %s %s/%s", info.Go, info.OS, info.Arch))
	}

	lines = append(lines, "")
	lines = append(lines, b.loc("admintext_6"))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_7"), runtime.GOOS, runtime.GOARCH))
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
	lines = append(lines, b.loc("admintext_8"))
	lines = append(lines, fmt.Sprintf("PID: %d", os.Getpid()))
	lines = append(lines, fmt.Sprintf("Uptime: %s", formatUptime(b.startTime)))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_9"), len(b.commands)))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_10"), len(b.callbacks)))

	lines = append(lines, "")
	lines = append(lines, b.loc("admintext_11"))
	stats := b.cache.Stats()
	lines = append(lines, fmt.Sprintf(b.loc("admintext_12"), stats.GroupsCount))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_13"), stats.TeachersCount))
	lines = append(lines, fmt.Sprintf(b.loc("admintext_14"), stats.Hits, stats.Misses))
	lines = append(lines, fmt.Sprintf("SuccessUpdate: %v", stats.SuccessUpdate))
	if stats.GroupsUpdate > 0 {
		lines = append(lines, fmt.Sprintf(b.loc("admintext_15"), time.UnixMilli(stats.GroupsUpdate).Format("02.01.2006 15:04")))
	}
	if stats.TeachersUpdate > 0 {
		lines = append(lines, fmt.Sprintf(b.loc("admintext_16"), time.UnixMilli(stats.TeachersUpdate).Format("02.01.2006 15:04")))
	}

	lines = append(lines, "")
	lines = append(lines, b.loc("admintext_17"))
	total, err := b.chatRepo.CountAll()
	if err == nil {
		lines = append(lines, fmt.Sprintf(b.loc("admintext_18"), total))
	}
	tgTotal, tgAllowed, err := b.chatRepo.CountTGChats()
	if err == nil {
		lines = append(lines, fmt.Sprintf(b.loc("admintext_19"), tgTotal, tgAllowed))
	}
	if b.keys != nil {
		if keys, active, err := b.keys.Counts(); err == nil {
			lines = append(lines, fmt.Sprintf(b.loc("admintext_20"), keys, active))
		}
	}
	notifyCount := 0
	notifyChats, err := b.chatRepo.FindAllWithNotifications("telegram")
	if err == nil {
		notifyCount = len(notifyChats)
	}
	lines = append(lines, fmt.Sprintf(b.loc("admintext_21"), notifyCount))
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
	lines = append(lines, b.loc("admintext_22"))
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
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_23"))
	}

	text := strings.TrimPrefix(u.Text, "/send")
	text = strings.TrimSpace(text)
	if text == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_24"))
	}
	// One over-long text would be rejected for every chat in the broadcast, so
	// the admin hears about it before anything is sent.
	if utf8.RuneCountInString(text) > telegramTextLimit {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_too_long"))
	}

	if !c.bot.broadcast.TryLock() {
		return u.Bot.SendText(u.ChatID, c.bot.loc("send_busy"))
	}

	chats, err := c.bot.chatRepo.FindAllTGChats()
	if err != nil {
		c.bot.broadcast.Unlock()
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_25")+err.Error())
	}

	if len(chats) == 0 {
		c.bot.broadcast.Unlock()
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_26"))
	}

	progressID, err := c.bot.sendMessageForID(u.ChatID, c.bot.loc("admintext_27"))
	if err != nil {
		c.bot.broadcast.Unlock()
		return err
	}

	go c.runBroadcast(u.ChatID, progressID, chats, text)
	return nil
}

func (c *sendCmd) runBroadcast(chatID int64, progressID int, chats []*Chat, text string) {
	defer c.bot.broadcast.Unlock()

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
			editText := fmt.Sprintf("telegram: %d/%d (%.2f%%)", i+1, total, pct)
			if failed > 0 {
				editText += fmt.Sprintf(c.bot.loc("admintext_28"), failed)
			}
			c.bot.EditMessageText(chatID, progressID, editText, nil)
			lastEdit = time.Now()
		}
	}

	result := c.bot.loc("admintext_29")
	if failed > 0 {
		result += fmt.Sprintf(c.bot.loc("admintext_30"), failed)
	}

	c.bot.EditMessageText(chatID, progressID, result, nil)
}

type triggerCmd struct{ bot *Bot }

func (c *triggerCmd) AdminOnly() bool { return true }

func (c *triggerCmd) Name() string        { return "/trigger" }
func (c *triggerCmd) Description() string { return c.bot.loc("cmd_trigger") }

func (c *triggerCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_31"))
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
	return c.bot.loc("admintext_32")
}
func (c *noticeDebugCmd) AdminOnly() bool { return true }

func (c *noticeDebugCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_33"))
	}

	lines := []string{c.bot.loc("admintext_34")}

	addSection := func(title string, chats []*Chat, format func(*Chat) string) {
		lines = append(lines, "")
		lines = append(lines, title)
		if len(chats) == 0 {
			lines = append(lines, c.bot.loc("admintext_35"))
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
			parts = append(parts, c.bot.loc("admintext_36")+chat.Group)
		}
		if chat.Teacher != "" {
			parts = append(parts, c.bot.loc("admintext_37")+chat.Teacher)
		}
		return strings.Join(parts, ", ")
	}

	groups, err := c.bot.chatRepo.FindChatsByGroups("telegram", c.bot.cache.GroupKeys(), true)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_38")+err.Error())
	}
	addSection(c.bot.loc("admintext_39"), groups, describe)

	teachers, err := c.bot.chatRepo.FindChatsByTeachers("telegram", c.bot.cache.TeacherKeys(), true)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_40")+err.Error())
	}
	addSection(c.bot.loc("admintext_41"), teachers, describe)

	subsG, err := c.bot.chatRepo.CountSubscriptionsByType("telegram", "group")
	if err == nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_42"), subsG))
	}
	subsT, err := c.bot.chatRepo.CountSubscriptionsByType("telegram", "teacher")
	if err == nil {
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_43"), subsT))
	}

	nextWeek, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_next_week")
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_44")+err.Error())
	}
	addSection(c.bot.loc("admintext_45"), nextWeek, describe)

	calls, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_calls")
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_46")+err.Error())
	}
	configured := make([]*Chat, 0, len(calls))
	for _, chat := range calls {
		if chat.Group != "" || chat.Teacher != "" {
			configured = append(configured, chat)
		}
	}
	addSection(c.bot.loc("admintext_47"), configured, describe)

	errs, err := c.bot.chatRepo.FindChatsWithNotice("telegram", "notice_parser_errors")
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_48")+err.Error())
	}
	addSection(c.bot.loc("admintext_49"), errs, describe)

	admins, err := c.bot.chatRepo.FindAdminChats("telegram", c.bot.cfg.Telegram.AdminIDs)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_50")+err.Error())
	}
	addSection(c.bot.loc("admintext_51"), admins, describe)

	cronChats, err := c.bot.chatRepo.FindAllWithNotifications("telegram")
	if err == nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_52"), len(cronChats)))
	}

	return u.Bot.SendText(u.ChatID, "<pre>"+strings.Join(lines, "\n")+"</pre>")
}

type archiveStatsCmd struct{ bot *Bot }

func (c *archiveStatsCmd) Name() string { return "/archivestats" }
func (c *archiveStatsCmd) Description() string {
	return c.bot.loc("admintext_53")
}
func (c *archiveStatsCmd) AdminOnly() bool { return true }

func (c *archiveStatsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_54"))
	}

	if c.bot.archive == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_55"))
	}

	stats, err := c.bot.archive.Stats(ctx)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admintext_56")+err.Error())
	}

	var lines []string
	lines = append(lines, c.bot.loc("admintext_57"))
	if bounds, err := c.bot.archive.DayIndexBounds(ctx); err == nil && bounds.Max > 0 {
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_58"), archive.DayIndexToDate(bounds.Min), archive.DayIndexToDate(bounds.Max)))
		minWeek := int(bounds.Min) / 7
		maxWeek := int(bounds.Max) / 7
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_59"), minWeek, maxWeek))
	} else {
		lines = append(lines, c.bot.loc("admintext_60"))
	}
	lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_61"), stats.Total, stats.WithGroup, stats.WithTeacher))

	if groups, err := c.bot.archive.Groups(ctx); err == nil && len(groups) > 0 {
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_62"), len(groups)))
	}
	if teachers, err := c.bot.archive.Teachers(ctx); err == nil && len(teachers) > 0 {
		lines = append(lines, fmt.Sprintf(c.bot.loc("admintext_63"), len(teachers)))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}
