package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"golang.org/x/text/encoding/charmap"
)

type brovkaCmd struct{ bot *Bot }

func (c *brovkaCmd) Name() string        { return "/vychetkaDlyaBrovkiDSOnline" }
func (c *brovkaCmd) Description() string { return "Отображает сколько групп заканчивают к определённой паре" }
func (c *brovkaCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/vychetkadlyabrovkidsonline")
}
func (c *brovkaCmd) Handler(ctx context.Context, u *Update) error {
	if u.Bot == nil {
		return fmt.Errorf("no bot context")
	}

	day := strings.TrimSpace(strings.TrimPrefix(u.Text, "/vychetkaDlyaBrovkiDSOnline"))
	if day == "" {
		return fmt.Errorf("день не указан")
	}

	archiveRepo, archiveOK := c.bot.archive.(*archive.Repository)
	if !archiveOK || archiveRepo == nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	if chat.Mode != ModeTeacher {
		return u.Bot.SendText(u.ChatID, "Доступно только для режима чата \"преподаватель\".")
	}
	if chat.Teacher == "" {
		return u.Bot.SendText(u.ChatID, "Для данного чата учитель не был выбран.")
	}

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Архив недоступен")
	}

	parsed, err := time.Parse("02.01.2006", day)
	if err != nil {
		if parsed, err = time.Parse("02.01.06", day); err != nil {
			return u.Bot.SendText(u.ChatID, "День, с которого необходимо начать не указан")
		}
	}
	fromDay := int64(utils.DayIndexFromDate(parsed))
	if fromDay < bounds.Min {
		fromDay = bounds.Min
	}

	entries, err := archiveRepo.TeacherDays(chat.Teacher, &fromDay)
	if err != nil || len(entries) == 0 {
		return u.Bot.SendText(u.ChatID, "Нет данных за указанный период")
	}

	csv := []string{"День,Подгруппа,Группа,Тип,Предмет"}
	for _, entry := range entries {
		dayTime, err := time.Parse("02.01.2006", entry.Day)
		dayLabel := entry.Day
		if err == nil {
			dayLabel = dayTime.Format("02.01")
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
			csv = append(csv, strings.Join([]string{
				dayLabel,
				subgroup,
				lesson.Group,
				lessonType,
				utils.GetFullSubjectName(lesson.Lesson),
			}, ","))
		}
	}

	encoded, err := charmap.Windows1251.NewEncoder().Bytes([]byte(strings.Join(csv, "\n")))
	if err != nil {
		encoded = []byte(strings.Join(csv, "\n"))
	}

	maxDay, _ := time.Parse("02.01.2006", entries[len(entries)-1].Day)
	filename := fmt.Sprintf("Вычетка c %s по %s (%d).csv", day, maxDay.Format("02.01.2006"), time.Now().UnixMilli())

	return u.Bot.SendDocument(u.ChatID, filename, encoded, "")
}


