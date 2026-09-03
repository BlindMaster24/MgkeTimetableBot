package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

func weekdayFromDate(date string) string {
	t, err := time.Parse("02.01.2006", date)
	if err != nil {
		return ""
	}
	return []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}[t.Weekday()]
}

type getCabinetCmd struct{ bot *Bot }

func (c *getCabinetCmd) Name() string { return "/cabinet" }
func (c *getCabinetCmd) Description() string {
	return "Получить информацию по кабинету"
}
func (c *getCabinetCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/cabinet" || strings.HasPrefix(lower, "/cabinet ") || strings.HasPrefix(lower, "/getcabinet ") || lower == "/getcabinet"
}

var cabinetRegexp = regexp.MustCompile(`^(!|\/)(get)?cabinet(\b|\s|$)`)

func (c *getCabinetCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	teachers := rasp.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, "Данные с сервера ещё не загружены, ожидайте...")
	}

	input := strings.TrimSpace(cabinetRegexp.ReplaceAllString(strings.ToLower(u.Text), ""))
	if input == "" {
		return u.Bot.SendText(u.ChatID, "Номер кабинета не указан")
	}

	type lessonInfo struct {
		Index    int
		Lesson   string
		Type     string
		Group    string
		Teacher  string
		Subgroup int
	}
	type dayInfo struct {
		Date    string
		Weekday string
		Lessons []lessonInfo
	}
	info := make(map[string]map[string]*dayInfo)

	for teacherName, teacherData := range teachers {
		teacherMap, ok := teacherData.(map[string]any)
		if !ok {
			continue
		}
		daysRaw, _ := teacherMap["days"].([]any)
		for _, dayRaw := range daysRaw {
			dayMap, ok := dayRaw.(map[string]any)
			if !ok {
				continue
			}
			dayDate, _ := dayMap["day"].(string)
			lessonsRaw, _ := dayMap["lessons"].([]any)
			for lessonIdx, lessonRaw := range lessonsRaw {
				lessonMap, ok := lessonRaw.(map[string]any)
				if !ok {
					continue
				}
				cab, _ := lessonMap["cabinet"].(string)
				if cab == "" {
					continue
				}
				if !strings.EqualFold(cab, input) {
					continue
				}
				lessonName, _ := lessonMap["lesson"].(string)
				lessonType, _ := lessonMap["type"].(string)
				lessonGroup, _ := lessonMap["group"].(string)
				subgroup := 0
				if s, ok := lessonMap["subgroup"].(float64); ok {
					subgroup = int(s)
				}
				if info[cab] == nil {
					info[cab] = make(map[string]*dayInfo)
				}
				if info[cab][dayDate] == nil {
					info[cab][dayDate] = &dayInfo{Date: dayDate, Weekday: weekdayFromDate(dayDate)}
				}
				info[cab][dayDate].Lessons = append(info[cab][dayDate].Lessons, lessonInfo{
					Index:    lessonIdx,
					Lesson:   lessonName,
					Type:     lessonType,
					Group:    lessonGroup,
					Teacher:  teacherName,
					Subgroup: subgroup})
			}
		}
	}

	if len(info) == 0 {
		return u.Bot.SendText(u.ChatID, "Кабинет не найден.")
	}

	var lines []string
	for cab, days := range info {
		lines = append(lines, fmt.Sprintf("Кабинет: %s", cab))
		for _, d := range days {
			lines = append(lines, fmt.Sprintf("%s, %s", d.Weekday, d.Date))
			for _, l := range d.Lessons {
				subgroup := ""
				if l.Subgroup > 0 {
					subgroup = fmt.Sprintf("%d. ", l.Subgroup)
				}
				lines = append(lines, fmt.Sprintf("%d. %s (%s), %s%s, %s", l.Index+1, l.Lesson, l.Type, subgroup, l.Group, l.Teacher))
			}
			lines = append(lines, "")
		}
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type getGroupsCmd struct{ bot *Bot }

func (c *getGroupsCmd) Name() string { return "/groups" }
func (c *getGroupsCmd) Description() string {
	return "Получить полный список групп в кэше бота"
}
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

func (c *getTeachersCmd) Name() string { return "/teachers" }
func (c *getTeachersCmd) Description() string {
	return "Получить полный список преподавателей в кэше бота"
}
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

func (c *compareGroupsCmd) Name() string { return "/comparegroups" }
func (c *compareGroupsCmd) Description() string {
	return "Сравнить расписания двух групп"
}
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err == nil {
		chat.Scene = "compare_groups_input"
		c.bot.chatRepo.Save(chat)
	}
	return u.Bot.SendText(u.ChatID, "Введите номера двух групп через пробел (например: 101 102)")
}
