package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/calendar"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
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

	parsed := parseCallsInput(u.Text)
	if parsed == nil {
		return u.Bot.SendText(u.ChatID, "Неверный формат ввода. Попробуйте ещё раз.")
	}
	s.bot.cache.SetCallsManual(parsed.Weekdays, parsed.Saturday, u.Text)

	return u.Bot.SendText(u.ChatID, "Расписание звонков обновлено вручную.")
}

type callsParsed struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

func parseCallsInput(text string) *callsParsed {
	var weekdays, saturday [][2][2]string
	var target *[][2][2]string

	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "суббот") || strings.Contains(lower, "saturday") {
			target = &saturday
			continue
		}
		if strings.Contains(lower, "будн") || strings.Contains(lower, "weekday") {
			target = &weekdays
			continue
		}
		if target == nil {
			target = &weekdays
		}
		row := parseCallRow(line)
		if row != nil {
			*target = append(*target, *row)
		}
	}

	if len(weekdays) == 0 && len(saturday) == 0 {
		return nil
	}

	return &callsParsed{Weekdays: weekdays, Saturday: saturday}
}

func parseCallRow(line string) *[2][2]string {
	re := regexp.MustCompile(`\b(\d{1,2})[:.](\d{2})\b`)
	matches := re.FindAllString(line, -1)
	if len(matches) < 4 {
		return nil
	}
	var row [2][2]string
	for i := 0; i < 4; i++ {
		parts := strings.Split(matches[i], ":")
		if parts[0] != matches[i] {
			parts = strings.Split(matches[i], ".")
		}
		hh := parts[0]
		if len(hh) == 1 {
			hh = "0" + hh
		}
		row[i/2][i%2] = hh + ":" + parts[1]
	}
	return &row
}

type pingCmd struct{ bot *Bot }

func (c *pingCmd) Name() string        { return "/ping" }
func (c *pingCmd) Description() string { return "Проверка работоспособности бота" }
func (c *pingCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "pong")
}

type icsCmd struct{ bot *Bot }

func (c *icsCmd) Name() string        { return "/ics" }
func (c *icsCmd) Description() string { return "Экспорт расписания в .ics" }
func (c *icsCmd) MatchText(text string) bool {
	return text == "📅 ICS" || text == "/ics"
}
func (c *icsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.cfg.Calendar.ICS.Enabled {
		return u.Bot.SendText(u.ChatID, "ICS отключен в конфиге.")
	}
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	week := utils.WeekIndexFromDate(time.Now())
	minIdx, maxIdx := week.WeekDayIndexRange()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
		}
		return c.bot.buildAndSendICS(u, chat, "group", chat.Group, minIdx, maxIdx, week)
	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
		}
		return c.bot.buildAndSendICS(u, chat, "teacher", chat.Teacher, minIdx, maxIdx, week)
	}

	return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
}

func (bot *Bot) buildAndSendICS(u *Update, chat *Chat, typeName, value string, minIdx, maxIdx int, week utils.WeekIndex) error {
	archiveRepo, ok := bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	builder := calendar.NewICSBuilder()
	weekNum := week.AcademicWeekNumber()

	switch typeName {
	case "group":
		days, err := archiveRepo.GroupDaysByRange(int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Нет расписания за текущую неделю.")
		}
		for _, d := range days {
			builder.AddGroupDay(d, value)
		}
	case "teacher":
		days, err := archiveRepo.TeacherDaysByRange(int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Нет расписания за текущую неделю.")
		}
		for _, d := range days {
			builder.AddTeacherDay(d, value)
		}
	}

	ics := builder.Build()
	var filename string
	switch typeName {
	case "group":
		filename = fmt.Sprintf("schedule-group-%s-week-%02d.ics", value, weekNum)
	case "teacher":
		filename = fmt.Sprintf("schedule-teacher-%s-week-%02d.ics", value, weekNum)
	}

	path := filepath.Join("./cache", filename)
	if err := os.WriteFile(path, []byte(ics), 0644); err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка создания .ics файла.")
	}

	f, err := os.Open(path)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения .ics файла.")
	}
	defer f.Close()

	_, err = bot.client.SendDocument(context.Background(), &telego.SendDocumentParams{
		ChatID: telego.ChatID{ID: u.ChatID},
		Document: telego.InputFile{File: f},
		Caption: fmt.Sprintf("📅 Расписание %s %s, учебная неделя №%d", typeName, value, weekNum),
	})
	return err
}

type subscriptionsTestCmd struct{ bot *Bot }

func (c *subscriptionsTestCmd) Name() string        { return "/subscriptions_test" }
func (c *subscriptionsTestCmd) Description() string { return "Тестовое уведомление по подпискам" }
func (c *subscriptionsTestCmd) MatchText(text string) bool {
	return text == "🧪 Проверить" || text == "/subscriptions_test"
}
func (c *subscriptionsTestCmd) Handler(ctx context.Context, u *Update) error {
	list, _ := c.bot.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		return u.Bot.SendText(u.ChatID, "Подписок нет.")
	}

	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	chat.Scene = "sub_test_pick"
	c.bot.chatRepo.Save(chat)

	prompt := "Что проверить?\n1. Оповещение об изменении дня\n2. Оповещение о новой неделе\n3. Оба варианта\n\n" + c.bot.formatSubscriptionsList(list)
	return u.Bot.SendText(u.ChatID, prompt)
}

type compareGroupsInputScene struct{ bot *Bot }

func (s *compareGroupsInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	input := strings.TrimSpace(u.Text)
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return u.Bot.SendText(u.ChatID, "Введите два номера групп через пробел.")
	}

	group1, group2 := parts[0], parts[1]
	groups := s.bot.cache.GetGroups()

	if _, ok := groups[group1]; !ok {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("Группа %s не найдена.", group1))
	}
	if _, ok := groups[group2]; !ok {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("Группа %s не найдена.", group2))
	}

	data1, _ := groups[group1].(map[string]any)
	data2, _ := groups[group2].(map[string]any)
	days1 := extractDays(data1)
	days2 := extractDays(data2)

	dateSet1 := make(map[string]bool)
	for _, d := range days1 {
		dateSet1[fmt.Sprint(d["day"])] = true
	}
	dateSet2 := make(map[string]bool)
	for _, d := range days2 {
		dateSet2[fmt.Sprint(d["day"])] = true
	}

	var common []string
	for date := range dateSet1 {
		if dateSet2[date] {
			common = append(common, date)
		}
	}

	msg := fmt.Sprintf("Расписание %s: %d дней\nРасписание %s: %d дней\nСовпадающих дней: %d", group1, len(days1), group2, len(days2), len(common))
	if len(common) > 0 {
		msg += "\n\nСовпадающие дни: " + strings.Join(common, ", ")
	}

	return u.Bot.SendText(u.ChatID, msg)
}

type archiveCmd struct{ bot *Bot }

func (c *archiveCmd) Name() string        { return "/archive" }
func (c *archiveCmd) Description() string { return "Архив расписания за прошедшие дни" }
func (c *archiveCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	raw := strings.TrimSpace(strings.TrimPrefix(u.Text, "/archive"))
	if raw == "" {
		return u.Bot.SendText(u.ChatID, "День не указан. Пример: /archive 12.02 или /archive week 5")
	}

	archiveRepo, ok := c.bot.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	weekMatch := regexp.MustCompile(`(?i)^(week|неделя)\s+(\d+)$`).FindStringSubmatch(raw)
	if weekMatch != nil {
		weekNum, _ := strconv.Atoi(weekMatch[2])
		if weekNum < 1 {
			return u.Bot.SendText(u.ChatID, "Неверный номер недели")
		}
		week := utils.WeekIndexFromNumber(weekNum)
		minIdx, maxIdx := week.WeekDayIndexRange()

		switch chat.Mode {
		case ModeStudent, ModeParent:
			if chat.Group == "" {
				return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
			}
			days, err := archiveRepo.GroupDaysByRange(int64(minIdx), int64(maxIdx), chat.Group)
			if err != nil || len(days) == 0 {
				return u.Bot.SendText(u.ChatID, "Нет данных за указанную неделю")
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
			opts := c.bot.getFormatterOpts(chat)
			opts.ShowHeader = true
			opts.WeekLabel = buildWeekLabelFromWeek(week)
			text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, dayMaps, opts)
			return u.Bot.SendText(u.ChatID, text)

		case ModeTeacher:
			if chat.Teacher == "" {
				return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
			}
			days, err := archiveRepo.TeacherDaysByRange(int64(minIdx), int64(maxIdx), chat.Teacher)
			if err != nil || len(days) == 0 {
				return u.Bot.SendText(u.ChatID, "Нет данных за указанную неделю")
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
			opts := c.bot.getFormatterOpts(chat)
			opts.ShowHeader = true
			opts.WeekLabel = buildWeekLabelFromWeek(week)
			text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, dayMaps, opts)
			return u.Bot.SendText(u.ChatID, text)
		}

		return u.Bot.SendText(u.ChatID, "Выберите группу или учителя")
	}

	parts := strings.Split(raw, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return u.Bot.SendText(u.ChatID, "Неверный формат. Пример: /archive 12.02 или /archive 12.02.2026")
	}

	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year := time.Now().Year()
	if len(parts) == 3 {
		year, _ = strconv.Atoi(parts[2])
	}

	if day < 1 || day > 31 || month < 1 || month > 12 {
		return u.Bot.SendText(u.ChatID, "Неверная дата")
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	dayIdx := utils.WeekIndexFromDate(date)
	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения архива")
	}

	wi := dayIdx.Value()
	if int64(wi) < bounds.Min || int64(wi) > bounds.Max {
		return u.Bot.SendText(u.ChatID, "Дата вне периода сохранённых данных")
	}

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_group"))
		}
		days, err := archiveRepo.GroupDaysByRange(int64(wi), int64(wi), chat.Group)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Ничего не найдено на данный день")
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
		opts := c.bot.getFormatterOpts(chat)
		opts.ShowHeader = true
		text := formatter.GetByIndex(chat.Formatter).FormatGroupFull(chat.Group, dayMaps, opts)
		return u.Bot.SendText(u.ChatID, text)

	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendText(u.ChatID, c.bot.loc("need_teacher"))
		}
		days, err := archiveRepo.TeacherDaysByRange(int64(wi), int64(wi), chat.Teacher)
		if err != nil || len(days) == 0 {
			return u.Bot.SendText(u.ChatID, "Ничего не найдено на данный день")
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
		opts := c.bot.getFormatterOpts(chat)
		opts.ShowHeader = true
		text := formatter.GetByIndex(chat.Formatter).FormatTeacherFull(chat.Teacher, dayMaps, opts)
		return u.Bot.SendText(u.ChatID, text)
	}

	return u.Bot.SendText(u.ChatID, "Выберите группу или учителя")
}

type endingsCmd struct{ bot *Bot }

func (c *endingsCmd) Name() string        { return "/endings" }
func (c *endingsCmd) Description() string { return "Сколько групп заканчивают к определённой паре" }
func (c *endingsCmd) Handler(ctx context.Context, u *Update) error {
	groups := c.bot.cache.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, "Данные ещё не загружены")
	}

	type dayStat map[int]int
	stat := make(map[string]dayStat)

	for _, raw := range groups {
		dataMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		daysArr, ok := dataMap["days"].([]any)
		if !ok {
			continue
		}
		for _, d := range daysArr {
			dayMap, ok := d.(map[string]any)
			if !ok {
				continue
			}
			dayStr, _ := dayMap["day"].(string)
			lessons, _ := dayMap["lessons"].([]any)
			lastLesson := -1
			for i, l := range lessons {
				if l == nil {
					continue
				}
				switch v := l.(type) {
					case map[string]any:
						if _, ok := v["lesson"]; ok {
							lastLesson = i
						}
					case []any:
						for _, sub := range v {
							if subMap, ok := sub.(map[string]any); ok {
								if _, ok := subMap["lesson"]; ok {
									lastLesson = i
								}
							}
						}
				}
			}
			if lastLesson == -1 {
				continue
			}
			if stat[dayStr] == nil {
				stat[dayStr] = make(dayStat)
			}
			stat[dayStr][lastLesson+1]++
		}
	}

	if len(stat) == 0 {
		return u.Bot.SendText(u.ChatID, "Нет данных для отображения")
	}

	var msg []string
	for day, counts := range stat {
		part := []string{"__ " + day + " __"}
		for lesson, count := range counts {
			part = append(part, fmt.Sprintf("%d групп заканчивают к %d паре", count, lesson))
		}
		msg = append(msg, strings.Join(part, "\n"))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(msg, "\n\n"))
}

type chatCmd struct{ bot *Bot }

func (c *chatCmd) Name() string        { return "/chat" }
func (c *chatCmd) Description() string { return "Просмотр информации о чате" }
func (c *chatCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка")
	}
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("<pre>%+v</pre>", chat))
}

type idCmd struct{ bot *Bot }

func (c *idCmd) Name() string        { return "/id" }
func (c *idCmd) Description() string { return "ID чата и пользователя" }
func (c *idCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("chat_id: %d\nuser_id: %d", u.ChatID, u.UserID))
}

type errorCmd struct{ bot *Bot }

func (c *errorCmd) Name() string        { return "/error" }
func (c *errorCmd) Description() string { return "Тестовая ошибка" }
func (c *errorCmd) Handler(ctx context.Context, u *Update) error {
	return fmt.Errorf("test error")
}

type testCmd struct{ bot *Bot }

func (c *testCmd) Name() string        { return "/test" }
func (c *testCmd) Description() string { return "Тестовая команда" }
func (c *testCmd) Handler(ctx context.Context, u *Update) error {
	return nil
}
