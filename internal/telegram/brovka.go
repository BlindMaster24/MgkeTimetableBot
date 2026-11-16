package telegram

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"golang.org/x/text/encoding/charmap"
)

func brovkaCSV(header []string, entries []model.TeacherDay) []string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	_ = writer.Write(header)

	for _, entry := range entries {
		dayLabel := entry.Day
		if parsed, err := time.Parse("02.01.2006", entry.Day); err == nil {
			dayLabel = parsed.Format("02.01")
		}

		for _, lesson := range entry.Lessons {
			if lesson == nil {
				continue
			}

			lessonType := "-"
			if lesson.Type != nil && *lesson.Type != "" {
				lessonType = strings.ToUpper(*lesson.Type)
			}

			subgroup := ""
			if lesson.Subgroup != nil {
				subgroup = fmt.Sprint(*lesson.Subgroup)
			}

			_ = writer.Write([]string{
				dayLabel,
				subgroup,
				lesson.Group,
				lessonType,
				utils.GetFullSubjectName(lesson.Lesson),
			})
		}
	}

	writer.Flush()
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

type brovkaCmd struct{ bot *Bot }

func (c *brovkaCmd) Name() string { return "/vychetkaDlyaBrovkiDSOnline" }
func (c *brovkaCmd) Description() string {
	return c.bot.loc("brovkatext_6")
}
func (c *brovkaCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/vychetkadlyabrovkidsonline")
}
func (c *brovkaCmd) Handler(ctx context.Context, u *Update) error {
	if u.Bot == nil {
		return fmt.Errorf("no bot context")
	}

	day := strings.TrimSpace(strings.TrimPrefix(u.Text, "/vychetkaDlyaBrovkiDSOnline"))
	if day == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_7"))
	}

	if c.bot.archive == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_8"))
	}

	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if chat.Mode != ModeTeacher {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_9"))
	}
	if chat.Teacher == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_10"))
	}

	bounds, err := c.bot.archive.DayIndexBounds(ctx)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_11"))
	}

	parsed, err := time.Parse("02.01.2006", day)
	if err != nil {
		if parsed, err = time.Parse("02.01.06", day); err != nil {
			return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_12"))
		}
	}
	fromDay := int64(utils.DayIndexFromDate(parsed))
	if fromDay < bounds.Min {
		fromDay = bounds.Min
	}

	entries, err := c.bot.archive.TeacherDays(ctx, chat.Teacher, &fromDay)
	if err != nil || len(entries) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("brovkatext_13"))
	}

	csv := brovkaCSV([]string{
		c.bot.loc("brovkatext_1"),
		c.bot.loc("brovkatext_2"),
		c.bot.loc("brovkatext_3"),
		c.bot.loc("brovkatext_4"),
		c.bot.loc("brovkatext_5"),
	}, entries)

	encoded, err := charmap.Windows1251.NewEncoder().Bytes([]byte(strings.Join(csv, "\n")))
	if err != nil {
		encoded = []byte(strings.Join(csv, "\n"))
	}

	maxDay, _ := time.Parse("02.01.2006", entries[len(entries)-1].Day)
	filename := fmt.Sprintf(c.bot.loc("brovkatext_14"), day, maxDay.Format("02.01.2006"), time.Now().UnixMilli())

	return u.Bot.SendDocument(u.ChatID, filename, encoded, "")
}
