package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

type compareGroupsStepA struct{ bot *Bot }

func (s *compareGroupsStepA) Handle(ctx context.Context, u *Update, chat *Chat) error {
	group, ok := s.bot.findGroup(u, chat, strings.TrimSpace(u.Text))
	if !ok {
		return nil
	}

	chat.Scene = sceneCompareInput + ":" + group
	s.bot.chatRepo.Save(chat)

	prompt := fmt.Sprintf("Введите номер второй группы (например, %s)", randomKey(s.bot.cache.GetGroups()))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(groupHistoryKeyboard(chat)))
}

type compareGroupsInputScene struct{ bot *Bot }

func (s *compareGroupsInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	groupA := strings.TrimPrefix(chat.Scene, sceneCompareInput+":")

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	groupB, ok := s.bot.findGroup(u, chat, strings.TrimSpace(u.Text))
	if !ok {
		return nil
	}

	if groupA == groupB {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Выберите две разные группы", replyMainMenu(s.bot, chat))
	}

	chat.AppendGroupHistory(groupA)
	chat.AppendGroupHistory(groupB)
	s.bot.chatRepo.Save(chat)

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, s.bot.compareGroupsMessage(groupA, groupB), replyMainMenu(s.bot, chat))
}

func (b *Bot) findGroup(u *Update, chat *Chat, input string) (string, bool) {
	return b.findGroupWithKeyboard(u, chat, input, replyMainMenu(b, chat))
}

func (b *Bot) findGroupWithKeyboard(u *Update, chat *Chat, input string, keyboard *telego.ReplyKeyboardMarkup) (string, bool) {
	normalized := strings.TrimRight(strings.TrimSpace(input), "*")

	if normalized == "" {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Это не число", keyboard)
		return "", false
	}
	for _, ch := range normalized {
		if ch < '0' || ch > '9' {
			u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Это не число", keyboard)
			return "", false
		}
	}
	if len(normalized) > 3 {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Номер группы введён неверно", keyboard)
		return "", false
	}
	if _, ok := b.cache.GetGroups()[normalized]; !ok {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Данной учебной группы не существует", keyboard)
		return "", false
	}

	return normalized, true
}

func (b *Bot) findTeacherWithKeyboard(u *Update, chat *Chat, input string, keyboard *telego.ReplyKeyboardMarkup) (string, bool, bool) {
	if len(input) < 3 {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Фамилия введена некорректно", keyboard)
		return "", false, false
	}

	matched, tooMany := matchTeacherList(input, b.cache.GetTeachers(), b.cache.GetTeamNames())
	if len(matched) == 0 {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Данный преподаватель не найден", keyboard)
		return "", false, false
	}
	if tooMany {
		u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Слишком много результатов для выборки.", keyboard)
		return "", false, false
	}
	if len(matched) > 1 {
		msg := "Найдено несколько преподавателей.\nКакой именно нужен?\n\n" + strings.Join(matched, "\n")
		u.Bot.SendTextWithKeyboard(u.ChatID, msg, withCancelButton(verticalValuesKeyboard(matched)))
		return "", false, true
	}

	return matched[0], true, false
}

func (b *Bot) compareGroupsMessage(groupA, groupB string) string {
	week := b.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()

	daysA := b.weekDayLessons(minIdx, maxIdx, groupA)
	daysB := b.weekDayLessons(minIdx, maxIdx, groupB)

	lines := []string{
		fmt.Sprintf("Сравнение групп %s и %s", groupA, groupB),
		buildWeekLabelFromWeek(week),
	}

	for dayIndex := minIdx; dayIndex <= maxIdx; dayIndex++ {
		date := utils.DayIndexToDate(dayIndex)
		if date.Weekday() == time.Sunday {
			continue
		}

		dayStr := date.Format("02.01.2006")
		lessonsA := daysA[dayStr]
		lessonsB := daysB[dayStr]

		calls := b.cfg.Timetable.Weekdays
		if date.Weekday() == time.Saturday {
			calls = b.cfg.Timetable.Saturday
		}

		maxPairs := len(lessonsA)
		if len(lessonsB) > maxPairs {
			maxPairs = len(lessonsB)
		}
		if len(calls) > maxPairs {
			maxPairs = len(calls)
		}

		var overlap []string
		var overlapDetails []string
		var sameSubjects []string
		var free []string

		for i := 0; i < maxPairs; i++ {
			a := hasLesson(lessonsAt(lessonsA, i))
			c := hasLesson(lessonsAt(lessonsB, i))

			if a && c {
				overlap = append(overlap, strconv.Itoa(i+1))

				aText := lessonText(lessonsAt(lessonsA, i))
				if aText == "" {
					aText = "—"
				}
				bText := lessonText(lessonsAt(lessonsB, i))
				if bText == "" {
					bText = "—"
				}

				same := sameSubjectsOf(lessonsAt(lessonsA, i), lessonsAt(lessonsB, i))
				if len(same) > 0 {
					sameSubjects = append(sameSubjects, fmt.Sprintf("%d) %s", i+1, strings.Join(same, " / ")))
				}

				detail := fmt.Sprintf("%d) %s | %s", i+1, aText, bText)
				if len(same) > 0 {
					detail += " — совпадает"
				}
				overlapDetails = append(overlapDetails, detail)
			}

			if !a && !c && i < len(calls) {
				free = append(free, strconv.Itoa(i+1))
			}
		}

		lines = append(lines, fmt.Sprintf("\n__ %s, %s __", weekdayFromDate(dayStr), dayStr))
		lines = append(lines, "Совпадающие пары: "+orNone(strings.Join(overlap, ", ")))
		if len(overlapDetails) > 0 {
			lines = append(lines, "Пары:")
			lines = append(lines, strings.Join(overlapDetails, "\n"))
		}
		lines = append(lines, "Общие предметы: "+orNone(strings.Join(sameSubjects, ", ")))

		formattedFree := make([]string, 0, len(free))
		for _, pair := range free {
			formattedFree = append(formattedFree, formatPair(pair, calls))
		}
		lines = append(lines, "Общие окна: "+orNone(strings.Join(formattedFree, ", ")))
	}

	return strings.Join(lines, "\n")
}

func (b *Bot) weekDayLessons(minIdx, maxIdx int, group string) map[string][]any {
	result := make(map[string][]any)

	if b.archive != nil {
		days, err := b.archive.GroupDaysByRange(int64(minIdx), int64(maxIdx), group)
		if err == nil {
			for _, day := range daysToMaps(days) {
				date, _ := day["day"].(string)
				lessons, _ := day["lessons"].([]any)
				result[date] = lessons
			}
			return result
		}
	}

	for _, day := range extractDays(b.cache.GetGroups()[group]) {
		date, _ := day["day"].(string)
		lessons, _ := day["lessons"].([]any)
		result[date] = lessons
	}
	return result
}

func lessonsAt(lessons []any, index int) any {
	if index < 0 || index >= len(lessons) {
		return nil
	}
	return lessons[index]
}

func hasLesson(lesson any) bool {
	switch value := lesson.(type) {
	case []any:
		for _, item := range value {
			if hasLesson(item) {
				return true
			}
		}
		return false
	case map[string]any:
		name, _ := value["lesson"].(string)
		return name != ""
	}
	return false
}

func lessonText(lesson any) string {
	switch value := lesson.(type) {
	case []any:
		var parts []string
		for _, item := range value {
			if text := lessonText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " / ")
	case map[string]any:
		return lessonExplainText(value)
	}
	return ""
}

func lessonExplainText(lesson map[string]any) string {
	name, _ := lesson["lesson"].(string)
	if name == "" {
		return ""
	}

	var sb strings.Builder
	if subgroup, ok := lesson["subgroup"].(float64); ok && subgroup > 0 {
		sb.WriteString(fmt.Sprintf("[%d] ", int(subgroup)))
	}
	sb.WriteString(name)

	if typ, ok := lesson["type"].(string); ok && typ != "" {
		sb.WriteString(" " + typ)
	}
	if cabinet, ok := lesson["cabinet"].(string); ok && cabinet != "" {
		sb.WriteString(" {" + cabinet + "}")
	}
	if teacher, ok := lesson["teacher"].(string); ok && teacher != "" {
		sb.WriteString(" - " + teacher)
	}
	return sb.String()
}

func lessonSubjects(lesson any) []string {
	switch value := lesson.(type) {
	case []any:
		var subjects []string
		for _, item := range value {
			if m, ok := item.(map[string]any); ok {
				if name, ok := m["lesson"].(string); ok && name != "" {
					subjects = append(subjects, name)
				}
			}
		}
		return subjects
	case map[string]any:
		if name, ok := value["lesson"].(string); ok && name != "" {
			return []string{name}
		}
	}
	return nil
}

func sameSubjectsOf(a, b any) []string {
	left := lessonSubjects(a)
	right := lessonSubjects(b)
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	index := make(map[string]bool, len(right))
	for _, value := range right {
		index[normalizeSubjectKey(value)] = true
	}

	var same []string
	for _, value := range left {
		if index[normalizeSubjectKey(value)] {
			same = append(same, value)
		}
	}
	return same
}

func normalizeSubjectKey(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func formatPair(pair string, calls [][2][2]string) string {
	index, err := strconv.Atoi(pair)
	if err != nil || index < 1 || index > len(calls) {
		return pair
	}

	return fmt.Sprintf("%s (%s-%s)", pair, calls[index-1][0][0], calls[index-1][1][1])
}
