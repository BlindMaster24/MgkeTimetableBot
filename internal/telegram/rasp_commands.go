package telegram

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
)

func (b *Bot) weekdayFromDate(date string) string {
	t, err := time.Parse("02.01.2006", date)
	if err != nil {
		return ""
	}
	return []string{b.loc("rasptext_1"), b.loc("rasptext_2"), b.loc("rasptext_3"), b.loc("rasptext_4"), b.loc("rasptext_5"), b.loc("rasptext_6"), b.loc("rasptext_7")}[t.Weekday()]
}

type getCabinetCmd struct{ bot *Bot }

func (c *getCabinetCmd) Name() string { return "/cabinet" }
func (c *getCabinetCmd) Description() string {
	return c.bot.loc("rasptext_8")
}
func (c *getCabinetCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/cabinet" || strings.HasPrefix(lower, "/cabinet ") || strings.HasPrefix(lower, "/getcabinet ") || lower == "/getcabinet"
}

var cabinetRegexp = regexp.MustCompile(`^(!|\/)(get)?cabinet(\b|\s|$)`)
var cabinetNumberRegexp = regexp.MustCompile(`(\d+-)?\d+`)

func cabinetMatches(cabinet, query string) bool {
	want := cabinetNumberRegexp.FindString(query)
	if want == "" {
		return strings.EqualFold(strings.TrimSpace(cabinet), strings.TrimSpace(query))
	}
	return cabinetNumberRegexp.FindString(cabinet) == want
}

func (c *getCabinetCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	teachers := rasp.GetTeachers()
	if len(teachers) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_9"))
	}

	input := strings.TrimSpace(cabinetRegexp.ReplaceAllString(strings.ToLower(u.Text), ""))
	if input == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_10"))
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
				if !cabinetMatches(cab, input) {
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
					info[cab][dayDate] = &dayInfo{Date: dayDate, Weekday: c.bot.weekdayFromDate(dayDate)}
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
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_11"))
	}

	cabinets := make([]string, 0, len(info))
	for cab := range info {
		cabinets = append(cabinets, cab)
	}
	sort.Strings(cabinets)

	var blocks []string
	for _, cab := range cabinets {
		dates := make([]string, 0, len(info[cab]))
		for date := range info[cab] {
			dates = append(dates, date)
		}
		sort.Strings(dates)

		lines := []string{fmt.Sprintf(c.bot.loc("rasptext_12"), cab)}
		for _, date := range dates {
			d := info[cab][date]
			sort.SliceStable(d.Lessons, func(i, j int) bool { return d.Lessons[i].Index < d.Lessons[j].Index })

			lines = append(lines, fmt.Sprintf("%s, %s", d.Weekday, d.Date))
			for _, l := range d.Lessons {
				subgroup := ""
				if l.Subgroup > 0 {
					subgroup = fmt.Sprintf("%d. ", l.Subgroup)
				}
				lines = append(lines, fmt.Sprintf("%d. %s (%s), %s%s, %s", l.Index+1, l.Lesson, l.Type, subgroup, l.Group, l.Teacher))
			}
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	return u.Bot.SendText(u.ChatID, strings.Join(blocks, "\n\n"))
}

type getGroupsCmd struct{ bot *Bot }

func (c *getGroupsCmd) Name() string { return "/groups" }
func (c *getGroupsCmd) Description() string {
	return c.bot.loc("rasptext_13")
}
func (c *getGroupsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/groups" || lower == "/getgroups"
}
func (c *getGroupsCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()

	groupNames := c.bot.archiveValues(ctx, "group")
	if len(groupNames) == 0 {
		groups := rasp.GetGroups()
		for name := range groups {
			groupNames = append(groupNames, name)
		}
		sort.Strings(groupNames)
	}

	if len(groupNames) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_14"))
	}

	msg := fmt.Sprintf(c.bot.loc("rasptext_15"),
		strings.Join(groupNames, ", "),
		formatAgo(c.bot.nowTime(), rasp.GetGroupsUpdateTime()),
		formatAgo(c.bot.nowTime(), rasp.GetGroupsChangedTime()))
	return u.Bot.SendText(u.ChatID, msg)
}

func (b *Bot) archiveValues(ctx context.Context, kind string) []string {
	if b.archive == nil {
		return nil
	}

	var (
		values []string
		err    error
	)
	if kind == "teacher" {
		values, err = b.archive.Teachers(ctx)
	} else {
		values, err = b.archive.Groups(ctx)
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
	return c.bot.loc("rasptext_16")
}
func (c *getTeachersCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/teachers" || lower == "/getteachers"
}
func (c *getTeachersCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()

	teacherNames := c.bot.archiveValues(ctx, "teacher")
	if len(teacherNames) == 0 {
		for name := range rasp.GetTeachers() {
			teacherNames = append(teacherNames, name)
		}
		sort.Strings(teacherNames)
	}

	if len(teacherNames) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_17"))
	}

	fullNames := rasp.GetTeamNames()
	lines := []string{c.bot.loc("rasptext_18")}
	for i, name := range teacherNames {
		label := name
		if full, ok := fullNames[name]; ok && full != "" {
			label = full
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, label))
	}

	lines = append(lines,
		fmt.Sprintf(c.bot.loc("rasptext_19"), formatAgo(c.bot.nowTime(), rasp.GetTeachersUpdateTime())),
		fmt.Sprintf(c.bot.loc("rasptext_20"), formatAgo(c.bot.nowTime(), rasp.GetTeachersChangedTime())),
		c.bot.loc("rasptext_21"),
		fmt.Sprintf(c.bot.loc("rasptext_22"), formatAgo(c.bot.nowTime(), rasp.GetTeamUpdateTime())),
		fmt.Sprintf(c.bot.loc("rasptext_23"), formatAgo(c.bot.nowTime(), rasp.GetTeamChangedTime())))

	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type compareGroupsCmd struct{ bot *Bot }

func (c *compareGroupsCmd) Name() string { return "/comparegroups" }
func (c *compareGroupsCmd) Description() string {
	return c.bot.loc("rasptext_24")
}
func (c *compareGroupsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/comparegroups" || lower == "/comparegroup" || lower == "/groupscompare" || strings.HasPrefix(lower, c.bot.loc("rasptext_25"))
}
func (c *compareGroupsCmd) Handler(ctx context.Context, u *Update) error {
	rasp := c.bot.GetRaspCache()
	groups := rasp.GetGroups()
	if len(groups) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("rasptext_26"))
	}
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	chat.Scene = sceneCompareStepA
	c.bot.saveChat(chat)

	prompt := fmt.Sprintf(c.bot.loc("rasptext_27"), randomKey(groups))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, c.bot.withCancelButton(groupHistoryKeyboard(chat)))
}
