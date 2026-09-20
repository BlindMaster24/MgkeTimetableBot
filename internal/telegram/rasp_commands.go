package telegram

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
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

	groupNames := c.bot.archiveValues("group")
	if len(groupNames) == 0 {
		groups := rasp.GetGroups()
		for name := range groups {
			groupNames = append(groupNames, name)
		}
		sort.Strings(groupNames)
	}

	if len(groupNames) == 0 {
		return u.Bot.SendText(u.ChatID, "Группы ещё не загружены")
	}

	msg := fmt.Sprintf("__ Группы в кэше __\n\n%s\n\nЗагружено: %s назад\nИзменено: %s назад",
		strings.Join(groupNames, ", "),
		formatAgo(c.bot.nowTime(), rasp.GetGroupsUpdateTime()),
		formatAgo(c.bot.nowTime(), rasp.GetGroupsChangedTime()))
	return u.Bot.SendText(u.ChatID, msg)
}

func (b *Bot) archiveValues(kind string) []string {
	archiveRepo, ok := b.archive.(*archive.Repository)
	if !ok || archiveRepo == nil {
		return nil
	}

	var (
		values []string
		err    error
	)
	if kind == "teacher" {
		values, err = archiveRepo.Teachers()
	} else {
		values, err = archiveRepo.Groups()
	}
	if err != nil {
		return nil
	}

	sort.Strings(values)
	return values
}

func formatAgo(now, then time.Time) string {
	seconds := int64(math.Ceil(now.Sub(then).Seconds()))
	if seconds < 0 {
		seconds = 0
	}
	return formatter.FormatSeconds(seconds)
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

	teacherNames := c.bot.archiveValues("teacher")
	if len(teacherNames) == 0 {
		for name := range rasp.GetTeachers() {
			teacherNames = append(teacherNames, name)
		}
		sort.Strings(teacherNames)
	}

	if len(teacherNames) == 0 {
		return u.Bot.SendText(u.ChatID, "Преподаватели ещё не загружены")
	}

	fullNames := rasp.GetTeamNames()
	lines := []string{"__ Преподаватели в кэше __"}
	for i, name := range teacherNames {
		label := name
		if full, ok := fullNames[name]; ok && full != "" {
			label = full
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, label))
	}

	lines = append(lines,
		fmt.Sprintf("\nЗагружено: %s назад", formatAgo(c.bot.nowTime(), rasp.GetTeachersUpdateTime())),
		fmt.Sprintf("Изменено: %s назад", formatAgo(c.bot.nowTime(), rasp.GetTeachersChangedTime())),
		"\n__ Страницы с учителями/администрацией __",
		fmt.Sprintf("Загружено: %s назад", formatAgo(c.bot.nowTime(), rasp.GetTeamUpdateTime())),
		fmt.Sprintf("Изменено: %s назад", formatAgo(c.bot.nowTime(), rasp.GetTeamChangedTime())))

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
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	chat.Scene = sceneCompareStepA
	c.bot.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите номер первой группы (например, %s)", randomKey(groups))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(groupHistoryKeyboard(chat)))
}
