package telegram

import (
	"context"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
)

type callsEditInputScene struct{ bot *Bot }

func (s *callsEditInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	parsed := parseCallsInput(u.Text)
	if parsed == nil {
		return u.Bot.SendText(u.ChatID, "Не удалось распознать расписание. Проверьте формат.")
	}

	chat.CallsEditInput = u.Text
	chat.Scene = sceneCallsEditReason
	s.bot.chatRepo.Save(chat)

	reasonKb := withCancelButton(&telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Пропустить", CallbackData: "answer:Пропустить"}},
		},
	})

	return u.Bot.SendTextWithKeyboard(u.ChatID, "Укажите причину изменения или нажмите Пропустить", reasonKb)
}

type callsEditReasonScene struct{ bot *Bot }

func (s *callsEditReasonScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	reasonText := strings.TrimSpace(u.Text)
	chat.CallsEditReason = reasonText
	chat.Scene = sceneCallsEditConfirm
	s.bot.chatRepo.Save(chat)

	confirmKb := withCancelButton(&telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "Отправить", CallbackData: "answer:Отправить"},
				{Text: "Не отправлять", CallbackData: "answer:Не отправлять"},
			},
		},
	})

	return u.Bot.SendTextWithKeyboard(u.ChatID, "Отправить уведомление всем?", confirmKb)
}

type callsEditConfirmScene struct{ bot *Bot }

func (s *callsEditConfirmScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	confirmText := strings.ToLower(u.Text)
	notifyNow := strings.Contains(confirmText, "отправить")

	reason := strings.TrimSpace(chat.CallsEditReason)
	if reason == "" || strings.EqualFold(reason, "пропустить") || strings.EqualFold(reason, "скип") {
		reason = ""
	}

	parsed := parseCallsInput(chat.CallsEditInput)
	if parsed == nil {
		chat.Scene = ""
		chat.CallsEditInput = ""
		chat.CallsEditReason = ""
		s.bot.chatRepo.Save(chat)
		return u.Bot.SendText(u.ChatID, "Не удалось распознать расписание. Проверьте формат.")
	}

	s.bot.cacheMu.Lock()
	s.bot.cache.SetCallsManualNotify(parsed.Weekdays, parsed.Saturday, reason, notifyNow)
	s.bot.cacheMu.Unlock()

	chat.Scene = ""
	chat.CallsEditInput = ""
	chat.CallsEditReason = ""
	s.bot.chatRepo.Save(chat)

	if notifyNow {
		return u.Bot.SendText(u.ChatID, "Расписание звонков обновлено и уведомление отправлено.")
	}
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
	matches := re.FindAllStringSubmatch(line, -1)
	if len(matches) < 4 {
		return nil
	}
	var row [2][2]string
	for i := 0; i < 4; i++ {
		hh := matches[i][1]
		mm := matches[i][2]
		if len(hh) == 1 {
			hh = "0" + hh
		}
		row[i/2][i%2] = hh + ":" + mm
	}
	return &row
}
