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
	return "Проверка работоспособности бота"
}
func (c *pingCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "pong")
}

type icsCmd struct{ bot *Bot }

func (c *icsCmd) Name() string        { return "/ics" }
func (c *icsCmd) Description() string { return "Экспорт расписания в .ics" }
func (c *icsCmd) MatchText(text string) bool {
	return text == "📅 ICS" || text == "/ics"
}
func (c *icsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.cfg.Calendar.ICS.Enabled {
		return u.Bot.SendText(u.ChatID, "ICS отключен в конфиге.")
	}
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	week := c.bot.relevantWeekIndex()
	minIdx, maxIdx := week.WeekDayIndexRange()

	switch chat.Mode {
	case ModeStudent, ModeParent:
		if chat.Group == "" {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Группа не выбрана. Используйте /setup.", replyMainMenu(c.bot, chat))
		}
		return c.bot.buildAndSendICS(u, chat, "group", chat.Group, minIdx, maxIdx, week)
	case ModeTeacher:
		if chat.Teacher == "" {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Преподаватель не выбран. Используйте /setup.", replyMainMenu(c.bot, chat))
		}
		return c.bot.buildAndSendICS(u, chat, "teacher", chat.Teacher, minIdx, maxIdx, week)
	}

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Режим чата не поддерживает экспорт расписания.", replyMainMenu(c.bot, chat))
}

func (bot *Bot) buildAndSendICS(u *Update, chat *Chat, typeName, value string, minIdx, maxIdx int, week utils.WeekIndex) error {
	if bot.archive == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	builder := calendar.NewICSBuilder()
	weekNum := week.AcademicWeekNumber()

	switch typeName {
	case "group":
		days, err := bot.archive.GroupDaysByRange(int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Нет расписания за текущую неделю.", replyMainMenu(bot, chat))
		}
		for _, d := range days {
			builder.AddGroupDay(d, value)
		}
	case "teacher":
		days, err := bot.archive.TeacherDaysByRange(int64(minIdx), int64(maxIdx), value)
		if err != nil || len(days) == 0 {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, "Нет расписания за текущую неделю.", replyMainMenu(bot, chat))
		}
		for _, d := range days {
			builder.AddTeacherDay(d, value)
		}
	}

	ics := builder.Build()
	var filename string
	switch typeName {
	case "group":
		filename = fmt.Sprintf("schedule-group-%s-week-%02d.ics", value, weekNum)
	case "teacher":
		filename = fmt.Sprintf("schedule-teacher-%s-week-%02d.ics", value, weekNum)
	}

	_, err := bot.client.SendDocument(context.Background(), &telego.SendDocumentParams{
		ChatID:   telego.ChatID{ID: u.ChatID},
		Document: telego.InputFile{File: namedReader{name: filename, reader: strings.NewReader(ics)}},
		Caption:  fmt.Sprintf("📅 Расписание %s %s, учебная неделя №%d", typeName, value, weekNum),
	})
	return err
}
