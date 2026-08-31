package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type historyCmd struct{ bot *Bot }

func (c *historyCmd) Name() string        { return "/history" }
func (c *historyCmd) Description() string { return c.bot.loc("cmd_history") }
func (c *historyCmd) MatchText(text string) bool {
	return text == "📚 История"
}
func (c *historyCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if chat.Mode != ModeTeacher || chat.Teacher == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
	}

	c.bot.chatRepo.SetScene(chat, "history_week")
	c.bot.chatRepo.Save(chat)

	return u.Bot.SendText(u.ChatID, c.bot.loc("history_enter_week"))
}

type historyWeekScene struct{ bot *Bot }

func (s *historyWeekScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	archiveRepo, ok := s.bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	weekIndex := s.parseWeekIndex(u.Text)
	if weekIndex == nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_invalid_week"))
	}

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	wi := weekIndex.Value()
	if int64(wi) < bounds.Min || int64(wi) > bounds.Max {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	minIdx, maxIdx := weekIndex.WeekDayIndexRange()
	days, err := archiveRepo.TeacherDaysByRange(int64(minIdx), int64(maxIdx), chat.Teacher)
	if err != nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("history_no_data"))
	}

	var dayMaps []map[string]any
	for _, d := range days {
		dm := map[string]any{"day": d.Day}
		var lessonsAny []any
		for _, l := range d.Lessons {
			if l != nil {
				lessonsAny = append(lessonsAny, l)
			}
		}
		dm["lessons"] = lessonsAny
		dayMaps = append(dayMaps, dm)
	}

	opts := s.bot.getFormatterOpts(chat)
	opts.ShowHeader = true
	opts.WeekLabel = buildWeekLabelFromWeek(*weekIndex)

	text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, dayMaps, opts)
	if text == "" {
		text = s.bot.loc("no_timetable")
	}

	kb := s.bot.weekControlKeyboard("teacher", chat.Teacher, weekIndex.Value(), false)

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	return u.Bot.SendTextWithKeyboard(u.ChatID, text, kb)
}

var regexp3 = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?$`)

func (s *historyWeekScene) parseWeekIndex(text string) *utils.WeekIndex {
	value := strings.TrimSpace(strings.ToLower(text))
	if value == "" {
		wi := utils.WeekIndexFromDate(time.Now())
		return &wi
	}

	if matched, _ := strconv.ParseInt(value, 10, 64); matched > 0 {
		wi := utils.WeekIndexFromNumber(int(matched))
		return &wi
	}

	dateMatch := regexp3.FindStringSubmatch(value)
	if dateMatch != nil {
		day, _ := strconv.Atoi(dateMatch[1])
		month, _ := strconv.Atoi(dateMatch[2])
		year := time.Now().Year()
		if dateMatch[3] != "" {
			year, _ = strconv.Atoi(dateMatch[3])
			if year < 100 {
				year += 2000
			}
		}
		if day > 0 && month > 0 && year > 0 {
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
			wi := utils.WeekIndexFromDate(date)
			return &wi
		}
	}

	return nil
}

type statsCmd struct{ bot *Bot }

func (c *statsCmd) Name() string        { return "/stats" }
func (c *statsCmd) Description() string { return c.bot.loc("cmd_stats") }
func (c *statsCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	archiveRepo, ok := c.bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	if (chat.Mode == ModeStudent || chat.Mode == ModeParent) && chat.Group != "" {
		msg := c.getGroupStats(archiveRepo, chat.Group)
		return u.Bot.SendText(u.ChatID, msg)
	}

	if chat.Mode == ModeTeacher && chat.Teacher != "" {
		msg := c.getTeacherStats(archiveRepo, chat.Teacher)
		return u.Bot.SendText(u.ChatID, msg)
	}

	return u.Bot.SendText(u.ChatID, c.bot.loc("stats_no_group"))
}

func (c *statsCmd) getGroupStats(repo *archive.Repository, group string) string {
	days, err := repo.GroupDays(group, nil)
	if err != nil || len(days) == 0 {
		return c.bot.loc("no_timetable")
	}

	type statEntry struct {
		key   string
		count int
	}
	total := make(map[string]int)

	for _, day := range days {
		for _, lesson := range day.Lessons {
			if lesson == nil {
				continue
			}
			entries := formatGroupLessonExplain(lesson)
			for _, entry := range entries {
				total[entry]++
			}
		}
	}

	var sorted []statEntry
	for k, v := range total {
		sorted = append(sorted, statEntry{key: k, count: v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var lines []string
	lines = append(lines, c.bot.loc("stats_header"))
	totalCount := 0
	for _, e := range sorted {
		lines = append(lines, fmt.Sprintf("%s - %d пар", e.key, e.count))
		totalCount += e.count
	}

	lines = append(lines, fmt.Sprintf("\nИтого всего пар (%d предметов): %d", len(sorted), totalCount))
	return strings.Join(lines, "\n")
}

func (c *statsCmd) getTeacherStats(repo *archive.Repository, teacher string) string {
	days, err := repo.TeacherDays(teacher, nil)
	if err != nil || len(days) == 0 {
		return c.bot.loc("no_timetable")
	}

	type statEntry struct {
		key   string
		count int
	}
	total := make(map[string]int)

	for _, day := range days {
		for _, lesson := range day.Lessons {
			if lesson == nil {
				continue
			}
			entries := formatTeacherLessonExplain(lesson)
			for _, entry := range entries {
				total[entry]++
			}
		}
	}

	var sorted []statEntry
	for k, v := range total {
		sorted = append(sorted, statEntry{key: k, count: v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var lines []string
	lines = append(lines, c.bot.loc("stats_header"))
	totalCount := 0
	for _, e := range sorted {
		lines = append(lines, fmt.Sprintf("%s - %d пар", e.key, e.count))
		totalCount += e.count
	}

	lines = append(lines, fmt.Sprintf("\nИтого всего пар (%d предметов): %d", len(sorted), totalCount))
	return strings.Join(lines, "\n")
}

func formatGroupLessonExplain(lesson any) []string {
	switch l := lesson.(type) {
	case map[string]any:
		return []string{formatExplainMap(l, "")}
	case []any:
		var result []string
		for _, item := range l {
			result = append(result, formatGroupLessonExplain(item)...)
		}
		return result
	}
	return nil
}

func formatTeacherLessonExplain(lesson any) []string {
	switch l := lesson.(type) {
	case map[string]any:
		return []string{formatExplainMap(l, "group")}
	case []any:
		var result []string
		for _, item := range l {
			result = append(result, formatTeacherLessonExplain(item)...)
		}
		return result
	}
	return nil
}

func formatExplainMap(m map[string]any, groupKey string) string {
	var parts []string

	if groupKey != "" {
		if group, ok := m[groupKey].(string); ok && group != "" {
			if subgroup, ok := m["subgroup"].(float64); ok {
				parts = append(parts, fmt.Sprintf("%d-%s.", int(subgroup), group))
			} else {
				parts = append(parts, group+".")
			}
		}
	} else {
		if subgroup, ok := m["subgroup"].(float64); ok {
			parts = append(parts, fmt.Sprintf("%d.", int(subgroup)))
		}
	}

	if lesson, ok := m["lesson"].(string); ok {
		parts = append(parts, lesson)
	}

	if typ, ok := m["type"].(string); ok && typ != "" {
		parts = append(parts, fmt.Sprintf("(%s)", typ))
	}

	if comment, ok := m["comment"].(string); ok && comment != "" {
		parts = append(parts, fmt.Sprintf("// %s", comment))
	}

	return strings.Join(parts, " ")
}

type historyCb struct{ bot *Bot }

func (cb *historyCb) Prefix() string { return "history" }
func (cb *historyCb) Handler(ctx context.Context, u *Update) error {
	cmd := &historyCmd{bot: cb.bot}
	return cmd.Handler(ctx, u)
}

type subsCheckFullCb struct{ bot *Bot }

func (cb *subsCheckFullCb) Prefix() string { return "subs_check_full" }
func (cb *subsCheckFullCb) Handler(ctx context.Context, u *Update) error {
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	list, _ := cb.bot.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendText(u.ChatID, "Подписок нет.")
	}

	chat.Scene = "sub_test_pick"
	cb.bot.chatRepo.Save(chat)

	prompt := "Что проверить?\n1. Оповещение об изменении дня\n2. Оповещение о новой неделе\n3. Оба варианта\n\n" + cb.bot.formatSubscriptionsList(list)
	return u.Bot.SendText(u.ChatID, prompt)
}

type subTestPickScene struct{ bot *Bot }

func (s *subTestPickScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	input := strings.TrimSpace(u.Text)

	list, _ := s.bot.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		chat.Scene = ""
		s.bot.chatRepo.Save(chat)
		return u.Bot.SendText(u.ChatID, "Подписок нет.")
	}

	mode := ""
	switch input {
	case "1":
		mode = "day"
	case "2":
		mode = "week"
	case "3":
		mode = "both"
	default:
		return u.Bot.SendText(u.ChatID, "Введите 1, 2 или 3")
	}

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	target := list[0]
	text := fmt.Sprintf("📢 Тестовое уведомление для %s %s:\n\n(%s)", target.Type, target.Value, mode)
	return u.Bot.SendTextWithKeyboard(u.ChatID, text, s.bot.subscriptionsKeyboard())
}

type callsEditCb struct{ bot *Bot }

func (cb *callsEditCb) Prefix() string { return "calls_edit" }
func (cb *callsEditCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}
	chat.Scene = "calls_edit_input"
	cb.bot.chatRepo.Save(chat)

	return u.Bot.SendText(u.ChatID, "Введите расписание звонков.\nПример\nБудни\n1 08:30 09:15 09:25 10:10\n2 10:20 11:05 11:15 12:00\nСуббота\n1 09:00 09:45 09:55 10:40")
}

type callsEditInputScene struct{ bot *Bot }

func (s *callsEditInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	return u.Bot.SendText(u.ChatID, "Расписание звонков обновлено вручную.")
}
