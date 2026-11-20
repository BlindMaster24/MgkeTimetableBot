package telegram

import (
	"context"
	"regexp"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/mymmrac/telego"
)

type callsEditInputScene struct{ bot *Bot }

func (s *callsEditInputScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	parsed := parseCallsInput(u.Text, s.bot.loc)
	if parsed == nil {
		return u.Bot.SendText(u.ChatID, s.bot.loc("calls_edit_unrecognized"))
	}

	chat.CallsEditInput = u.Text
	chat.Scene = sceneCallsEditReason
	s.bot.chatRepo.Save(chat)

	reasonKb := s.bot.withCancelButton(&telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: s.bot.loc("calls_edit_skip"), CallbackData: "answer:" + s.bot.loc("calls_edit_skip")}},
		},
	})

	return u.Bot.SendTextWithKeyboard(u.ChatID, s.bot.loc("calls_edit_reason_prompt"), reasonKb)
}

type callsEditReasonScene struct{ bot *Bot }

func (s *callsEditReasonScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	reasonText := strings.TrimSpace(u.Text)
	chat.CallsEditReason = reasonText
	chat.Scene = sceneCallsEditConfirm
	s.bot.chatRepo.Save(chat)

	confirmKb := s.bot.withCancelButton(&telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: s.bot.loc("calls_edit_send"), CallbackData: "answer:" + s.bot.loc("calls_edit_send")},
				{Text: s.bot.loc("calls_edit_dont_send"), CallbackData: "answer:" + s.bot.loc("calls_edit_dont_send")},
			},
		},
	})

	return u.Bot.SendTextWithKeyboard(u.ChatID, s.bot.loc("calls_edit_confirm"), confirmKb)
}

type callsEditConfirmScene struct{ bot *Bot }

func (s *callsEditConfirmScene) Handle(ctx context.Context, u *Update, chat *Chat) error {
	notifyNow := strings.EqualFold(strings.TrimSpace(u.Text), s.bot.loc("calls_edit_send"))

	reason := strings.TrimSpace(chat.CallsEditReason)
	if reason == "" || strings.EqualFold(reason, s.bot.loc("calls_edit_reason_skip")) || strings.EqualFold(reason, s.bot.loc("calls_edit_reason_skip_short")) {
		reason = ""
	}

	parsed := parseCallsInput(chat.CallsEditInput, s.bot.loc)
	if parsed == nil {
		chat.Scene = ""
		chat.CallsEditInput = ""
		chat.CallsEditReason = ""
		s.bot.chatRepo.Save(chat)
		return u.Bot.SendText(u.ChatID, s.bot.loc("calls_edit_unrecognized"))
	}

	s.bot.cache.SetCallsManualNotify(parsed.Weekdays, parsed.Saturday, reason, notifyNow)

	chat.Scene = ""
	chat.CallsEditInput = ""
	chat.CallsEditReason = ""
	s.bot.chatRepo.Save(chat)

	if notifyNow {
		return u.Bot.SendText(u.ChatID, s.bot.loc("calls_edit_done_notified"))
	}
	return u.Bot.SendText(u.ChatID, s.bot.loc("calls_edit_done_manual"))
}

var callsRowRe = regexp.MustCompile(`\b(\d{1,2})[:.](\d{2})\b`)

type callsParsed struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

func parseCallsInput(text string, loc func(string) string) *callsParsed {
	var weekdays, saturday [][2][2]string
	var target *[][2][2]string

	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, loc("calls_parse_saturday")) || strings.Contains(lower, "saturday") {
			target = &saturday
			continue
		}
		if strings.Contains(lower, loc("calls_parse_weekday")) || strings.Contains(lower, "weekday") {
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

	if err := (cache.CallsSchedule{Weekdays: weekdays, Saturday: saturday}).Validate(); err != nil {
		return nil
	}

	return &callsParsed{Weekdays: weekdays, Saturday: saturday}
}

func parseCallRow(line string) *[2][2]string {
	matches := callsRowRe.FindAllStringSubmatch(line, -1)
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
