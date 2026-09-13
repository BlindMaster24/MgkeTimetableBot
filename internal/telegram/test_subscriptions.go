package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type subTestPickScene struct{ bot *Bot }

func (s *subTestPickScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	list, _ := s.bot.chatRepo.GetSubscriptions(u.UserID)
	if len(list) == 0 {
		chat.Scene = ""
		s.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Подписок нет.", s.bot.replySubscriptionsMenu())
	}

	input := strings.TrimSpace(u.Text)

	idx := 0
	for _, c := range input {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		}
	}

	var target Subscription
	if idx >= 1 && idx <= len(list) {
		target = list[idx-1]
	} else {
		found := false
		for _, item := range list {
			if normalizeSubValue(item.Value) == normalizeSubValue(input) {
				target = item
				found = true
				break
			}
		}
		if !found {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Неверный номер подписки.", s.bot.replySubscriptionsMenu())
		}
	}

	chat.Scene = "sub_test_mode:" + target.Type + ":" + target.Value
	s.bot.chatRepo.Save(chat)

	prompt := "Что проверить?\n1. Оповещение об изменении дня\n2. Оповещение о новой неделе\n3. Оба варианта"
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, prompt, s.bot.replySubscriptionsMenu())
}

func normalizeSubValue(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(value, ".", ""), " ", ""))
}

type subTestModeScene struct{ bot *Bot }

func (s *subTestModeScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	payload := strings.TrimPrefix(chat.Scene, "sub_test_mode:")
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		chat.Scene = ""
		s.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Неверный номер подписки.", s.bot.replySubscriptionsMenu())
	}
	subType, subValue := parts[0], parts[1]

	mode := ""
	switch strings.TrimSpace(u.Text) {
	case "1":
		mode = "day"
	case "2":
		mode = "week"
	case "3":
		mode = "both"
	default:
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Введите 1, 2 или 3", s.bot.replySubscriptionsMenu())
	}

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	week := s.bot.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()
	days := s.bot.archiveDaysForWeek(subType, subValue, minIdx, maxIdx)

	label := "Группа"
	if subType == "teacher" {
		label = "Преподаватель"
	}

	if mode == "day" || mode == "both" {
		day := pickFutureDay(days)
		phrase := "день"
		var one []map[string]any
		if day != nil {
			phrase = subscriptionDayPhrase(day["day"].(string))
			one = []map[string]any{day}
		}

		opts := s.bot.fmtOpts(chat, false)
		var text string
		if subType == "teacher" {
			text = formatter.GetByIndex(chat.Formatter).FormatTeacherFull(subValue, one, opts)
		} else {
			text = formatter.GetByIndex(chat.Formatter).FormatGroupFull(subValue, one, opts)
		}

		message := fmt.Sprintf("📢 %s %s: расписание на %s\n%s", label, subValue, phrase, text)
		if err := u.Bot.SendTextWithReplyKeyboard(u.ChatID, message, s.bot.replySubscriptionsMenu()); err != nil {
			return err
		}
		if mode == "day" {
			return nil
		}
	}

	return u.Bot.SendTextWithKeyboard(u.ChatID,
		fmt.Sprintf("🆕 %s %s: доступно расписание на следующую неделю", label, subValue),
		weekTimetableButton("📃 Показать", subType, subValue, week.Value(), false))
}

func pickFutureDay(days []map[string]any) map[string]any {
	todayIdx := utils.DayIndexFromDate(time.Now())
	for _, day := range days {
		dateStr, _ := day["day"].(string)
		t, err := time.Parse("02.01.2006", dateStr)
		if err != nil {
			continue
		}
		if utils.DayIndexFromDate(t) > todayIdx {
			return day
		}
	}
	if len(days) > 0 {
		return days[0]
	}
	return nil
}

func subscriptionDayPhrase(day string) string {
	t, err := time.Parse("02.01.2006", day)
	if err != nil {
		return "день"
	}

	dayIdx := utils.DayIndexFromDate(t)
	todayIdx := utils.DayIndexFromDate(time.Now())
	if dayIdx == todayIdx {
		return "сегодня"
	}
	if dayIdx == todayIdx+1 {
		return "завтра"
	}
	if utils.WeekIndexFromDate(t).IsFutureWeek() {
		return "следующую неделю"
	}
	return "день"
}

type subscriptionsTestCmd struct{ bot *Bot }

func (c *subscriptionsTestCmd) Name() string { return "/subscriptions_test" }
func (c *subscriptionsTestCmd) Description() string {
	return "Тестовое уведомление по подпискам"
}
func (c *subscriptionsTestCmd) MatchText(text string) bool {
	return text == "🧪 Проверить" || text == "/subscriptions_test"
}
func (c *subscriptionsTestCmd) Handler(ctx context.Context, u *Update) error {
	return c.bot.subTestPrompt(u)
}
