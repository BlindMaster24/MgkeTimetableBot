package telegram

import (
	"context"
	"fmt"
	"strings"
)

type compareGroupsStepA struct{ bot *Bot }

func (s *compareGroupsStepA) Handle(ctx context.Context, u *Update, chat *Chat) error {
	groups := s.bot.cache.GetGroups()
	input := strings.TrimSpace(u.Text)

	if _, ok := groups[input]; !ok {
		return u.Bot.SendText(u.ChatID, "Данной учебной группы не существует")
	}

	chat.Scene = sceneCompareInput + ":" + input
	s.bot.chatRepo.Save(chat)
	prompt := fmt.Sprintf("Введите номер второй группы (например, %s)", randomKey(groups))
	return u.Bot.SendTextWithKeyboard(u.ChatID, prompt, withCancelButton(groupHistoryKeyboard(chat)))
}

type compareGroupsInputScene struct{ bot *Bot }

func (s *compareGroupsInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	groupA := strings.TrimPrefix(chat.Scene, "compare_groups_input:")
	groupB := strings.TrimSpace(u.Text)

	chat.Scene = ""
	s.bot.chatRepo.Save(chat)

	groups := s.bot.cache.GetGroups()
	if _, ok := groups[groupB]; !ok {
		return u.Bot.SendText(u.ChatID, "Данной учебной группы не существует")
	}
	if groupA == groupB {
		return u.Bot.SendText(u.ChatID, "Выберите две разные группы")
	}

	chat.AppendGroupHistory(groupA)
	chat.AppendGroupHistory(groupB)
	s.bot.chatRepo.Save(chat)

	data1, _ := groups[groupA].(map[string]any)
	data2, _ := groups[groupB].(map[string]any)
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

	msg := fmt.Sprintf("Сравнение групп %s и %s\nРасписание %s: %d дней\nРасписание %s: %d дней\nСовпадающих дней: %d", groupA, groupB, groupA, len(days1), groupB, len(days2), len(common))
	if len(common) > 0 {
		msg += "\n\nСовпадающие дни: " + strings.Join(common, ", ")
	}

	return u.Bot.SendText(u.ChatID, msg)
}
