package telegram

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/calendar"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

type namedReader struct {
	name   string
	reader io.Reader
}

func (r namedReader) Name() string               { return r.name }
func (r namedReader) Read(p []byte) (int, error) { return r.reader.Read(p) }

type pingCmd struct{ bot *Bot }

func (c *pingCmd) Name() string { return "/ping" }
func (c *pingCmd) Description() string {
	return c.bot.loc("icstext_1")
}
func (c *pingCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "pong")
}

type icsCmd struct{ bot *Bot }

func (c *icsCmd) Name() string        { return "/ics" }
func (c *icsCmd) Description() string { return c.bot.loc("icstext_2") }
func (c *icsCmd) MatchText(text string) bool {
	return text == "📅 ICS" || text == "/ics"
}
func (c *icsCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if !c.bot.cfg.Calendar.ICS.Enabled {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("icstext_3"), replyMainMenu(c.bot, chat))
	}

	week := c.bot.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("icstext_4"), replyMainMenu(c.bot, chat))
		}
		return c.bot.buildAndSendICS(ctx, u, chat, "group", chat.Group, minIdx, maxIdx, week)
	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("icstext_5"), replyMainMenu(c.bot, chat))
		}
		return c.bot.buildAndSendICS(ctx, u, chat, "teacher", chat.Teacher, minIdx, maxIdx, week)
	}

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("icstext_6"), replyMainMenu(c.bot, chat))
}

func (bot *Bot) buildAndSendICS(ctx context.Context, u *Update, chat *Chat, typeName, value string, minIdx, maxIdx int, week utils.WeekIndex) error {
	if bot.archive == nil {
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, bot.loc("icstext_7"), replyMainMenu(bot, chat))
	}

	weekNum := week.AcademicWeekNumber()
	schedule := bot.callsScheduleFor(chat)
	builder := calendar.NewICSBuilder(schedule.Weekdays, schedule.Saturday, weekNum, calendar.Texts{
		Teacher:  bot.loc("ics_teacher"),
		Group:    bot.loc("ics_group"),
		Cabinet:  bot.loc("ics_cabinet"),
		Subgroup: bot.loc("ics_subgroup"),
	})

	switch typeName {
	case "group":
		days, err := bot.archive.GroupDaysByRange(ctx, int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, bot.loc("icstext_8"), replyMainMenu(bot, chat))
		}
		for _, d := range days {
			builder.AddGroupDay(d, value)
		}
	case "teacher":
		days, err := bot.archive.TeacherDaysByRange(ctx, int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, bot.loc("icstext_9"), replyMainMenu(bot, chat))
		}
		for _, d := range days {
			builder.AddTeacherDay(d, value)
		}
	}

	ics := builder.Build()
	filename := fmt.Sprintf("schedule-%s-%s-week-%02d.ics", typeName, value, weekNum)

	_, err := bot.client.SendDocument(context.Background(), &telego.SendDocumentParams{
		ChatID:      telego.ChatID{ID: u.ChatID},
		Document:    telego.InputFile{File: namedReader{name: filename, reader: strings.NewReader(ics)}},
		ReplyMarkup: replyMainMenu(bot, chat),
	})
	return err
}
