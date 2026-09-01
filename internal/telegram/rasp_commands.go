package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type getCabinetCmd struct{ bot *Bot }

func (c *getCabinetCmd) Name() string        { return "/cabinet" }
func (c *getCabinetCmd) Description() string { return "Получить информацию по кабинету" }
func (c *getCabinetCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/cabinet" || strings.HasPrefix(lower, "/cabinet ") || strings.HasPrefix(lower, "/getcabinet ") || lower == "/getcabinet"
}
func (c *getCabinetCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	teachers := rasp.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, "Данные с сервера ещё не загружены, ожидайте...")
	}

	input := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(u.Text, "/cabinet"), "/getcabinet"))
	if input == "" {
		return u.Bot.SendText(u.ChatID, "Номер кабинета не указан")
	}

	type cabinetInfo struct {
		day     string
		lessons []string
	}
	info := make(map[string][]cabinetInfo)

	for teacherName := range teachers {
		_ = teacherName
	}

	if len(info) == 0 {
		return u.Bot.SendText(u.ChatID, "Кабинет не найден.")
	}

	var lines []string
	for cab, days := range info {
		lines = append(lines, fmt.Sprintf("Кабинет: %s", cab))
		for _, d := range days {
			lines = append(lines, fmt.Sprintf("%s", d.day))
			lines = append(lines, d.lessons...)
		}
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type getGroupsCmd struct{ bot *Bot }

func (c *getGroupsCmd) Name() string        { return "/groups" }
func (c *getGroupsCmd) Description() string { return "Получить полный список групп в кэше бота" }
func (c *getGroupsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/groups" || lower == "/getgroups"
}
func (c *getGroupsCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	groups := rasp.GetGroups()

	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	if len(groupNames) == 0 {
		return u.Bot.SendText(u.ChatID, "Группы ещё не загружены")
	}

	ago := time.Since(rasp.GetGroupsUpdateTime()).Truncate(time.Second)
	msg := fmt.Sprintf("Группы в кэше:\n%s\n\nЗагружено: %s назад", strings.Join(groupNames, ", "), ago)
	return u.Bot.SendText(u.ChatID, msg)
}

type getTeachersCmd struct{ bot *Bot }

func (c *getTeachersCmd) Name() string        { return "/teachers" }
func (c *getTeachersCmd) Description() string { return "Получить полный список преподавателей в кэше бота" }
func (c *getTeachersCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/teachers" || lower == "/getteachers"
}
func (c *getTeachersCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	teachers := rasp.GetTeachers()

	teacherNames := make([]string, 0, len(teachers))
	for name := range teachers {
		teacherNames = append(teacherNames, name)
	}
	sort.Strings(teacherNames)

	if len(teacherNames) == 0 {
		return u.Bot.SendText(u.ChatID, "Преподаватели ещё не загружены")
	}

	var lines []string
	lines = append(lines, "Преподаватели в кэше:")
	for i, name := range teacherNames {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, name))
	}

	ago := time.Since(rasp.GetTeachersUpdateTime()).Truncate(time.Second)
	lines = append(lines, fmt.Sprintf("\nЗагружено: %s назад", ago))
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type compareGroupsCmd struct{ bot *Bot }

func (c *compareGroupsCmd) Name() string        { return "/comparegroups" }
func (c *compareGroupsCmd) Description() string { return "Сравнить расписания двух групп" }
func (c *compareGroupsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/comparegroups" || lower == "/comparegroup" || lower == "/groupscompare" || strings.HasPrefix(lower, "сравнить группы")
}
func (c *compareGroupsCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	groups := rasp.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, "Данные с сервера ещё не загружены, ожидайте...")
	}
	return u.Bot.SendText(u.ChatID, "Введите номера двух групп через пробел (например: 101 102)")
}
