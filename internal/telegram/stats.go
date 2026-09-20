package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type statsCmd struct{ bot *Bot }

func (c *statsCmd) Name() string        { return "/stats" }
func (c *statsCmd) Description() string { return c.bot.loc("cmd_stats") }
func (c *statsCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if c.bot.archive == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	if (chat.Mode == ModeStudent || chat.Mode == ModeParent) && chat.Group != "" {
		msg := c.getGroupStats(c.bot.archive, chat.Group)
		return u.Bot.SendText(u.ChatID, msg)
	}

	if chat.Mode == ModeTeacher && chat.Teacher != "" {
		msg := c.getTeacherStats(c.bot.archive, chat.Teacher)
		return u.Bot.SendText(u.ChatID, msg)
	}

	return u.Bot.SendText(u.ChatID, c.bot.loc("stats_no_group"))
}

func (c *statsCmd) getGroupStats(repo archiveStore, group string) string {
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
	sort.SliceStable(sorted, func(i, j int) bool {
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

func (c *statsCmd) getTeacherStats(repo archiveStore, teacher string) string {
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
	sort.SliceStable(sorted, func(i, j int) bool {
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
			if subgroup, ok := m["subgroup"].(float64); ok && subgroup > 0 {
				parts = append(parts, fmt.Sprintf("%d-%s.", int(subgroup), group))
			} else {
				parts = append(parts, group+".")
			}
		}
	} else {
		if subgroup, ok := m["subgroup"].(float64); ok && subgroup > 0 {
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
