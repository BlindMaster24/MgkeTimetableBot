package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/robfig/cron/v3"
)

type Sender interface {
	SendText(chatID int64, text string) error
}

type ChatInfo struct {
	ID            int64
	Mode          string
	Group         string
	Teacher       string
	NoticeChanges bool
}

type ChatFinder interface {
	FindAllWithNotifications(service string) ([]*ChatInfo, error)
}

type Scheduler struct {
	cron   *cron.Cron
	cfg    *config.Config
	cache  *cache.RaspCache
	log    *logger.Logger
	sender Sender
	chats  ChatFinder
}

func NewScheduler(cfg *config.Config, cache *cache.RaspCache, log *logger.Logger, sender Sender, chats ChatFinder) *Scheduler {
	loc := time.FixedZone("MSK", 3*3600)
	return &Scheduler{
		cron:   cron.New(cron.WithLocation(loc)),
		cfg:    cfg,
		cache:  cache,
		log:    log,
		sender: sender,
		chats:  chats,
	}
}

func (s *Scheduler) Start() {
	schedule := s.cfg.Timetable

	for dayType, slots := range map[string][][2][2]string{
		"weekday": schedule.Weekdays,
		"saturday": schedule.Saturday,
	} {
		if len(slots) == 0 {
			continue
		}

		lastSlot := slots[len(slots)-1]
		endTime := lastSlot[1][1]
		if endTime == "" {
			continue
		}

		parts := strings.SplitN(endTime, ":", 2)
		if len(parts) != 2 {
			continue
		}

		hour, min := parts[0], parts[1]
		var dayRange string
		if dayType == "weekday" {
			dayRange = "1-5"
		} else {
			dayRange = "6"
		}

		cronExpr := fmt.Sprintf("0 %s %s * * %s", min, hour, dayRange)
		dayTypeCopy := dayType
		_, err := s.cron.AddFunc(cronExpr, func() {
			s.sendNotifications(dayTypeCopy)
		})
		if err != nil {
			s.log.Error().Err(err).Str("expr", cronExpr).Msg("failed to schedule notification")
			continue
		}
		s.log.Info().Str("expr", cronExpr).Str("type", dayType).Msg("notification scheduled")
	}

	s.cron.Start()
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *Scheduler) sendNotifications(dayType string) {
	s.log.Info().Str("type", dayType).Msg("sending scheduled notifications")

	allChats, err := s.chats.FindAllWithNotifications("telegram")
	if err != nil {
		s.log.Error().Err(err).Msg("failed to find chats")
		return
	}

	groups := s.cache.GetGroups()
	teachers := s.cache.GetTeachers()

	today := time.Now().Format("02.01.2006")

	for _, chat := range allChats {
		if !chat.NoticeChanges {
			continue
		}

		switch chat.Mode {
		case "student", "parent":
			if chat.Group == "" {
				continue
			}
			data, ok := groups[chat.Group]
			if !ok {
				continue
			}
			msg := s.buildGroupNotification(chat.Group, data, today)
			if msg == "" {
				continue
			}
			if err := s.sender.SendText(chat.ID, msg); err != nil {
				s.log.Error().Err(err).Int64("chatID", chat.ID).Msg("send notification failed")
			}

		case "teacher":
			if chat.Teacher == "" {
				continue
			}
			data, ok := teachers[chat.Teacher]
			if !ok {
				continue
			}
			msg := s.buildTeacherNotification(chat.Teacher, data, today)
			if msg == "" {
				continue
			}
			if err := s.sender.SendText(chat.ID, msg); err != nil {
				s.log.Error().Err(err).Int64("chatID", chat.ID).Msg("send notification failed")
			}
		}
	}
}

func (s *Scheduler) buildGroupNotification(group string, data any, today string) string {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return ""
	}

	var targetDay string
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		if dayStr == today {
			targetDay = dayStr
			break
		}
	}
	if targetDay == "" {
		return ""
	}

	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		if dayStr != targetDay {
			continue
		}
		lessons, _ := dayMap["lessons"].([]any)
		if len(lessons) == 0 {
			return ""
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\U0001F4E2 Расписание на сегодня\nГруппа %s\n\n", group))
		for i, l := range lessons {
			text := formatLesson(l, i+1)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		}
		return sb.String()
	}
	return ""
}

func (s *Scheduler) buildTeacherNotification(teacher string, data any, today string) string {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return ""
	}

	var targetDay string
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		if dayStr == today {
			targetDay = dayStr
			break
		}
	}
	if targetDay == "" {
		return ""
	}

	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		if dayStr != targetDay {
			continue
		}
		lessons, _ := dayMap["lessons"].([]any)
		if len(lessons) == 0 {
			return ""
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\U0001F4E2 Расписание на сегодня\nПреподаватель %s\n\n", teacher))
		for i, l := range lessons {
			text := formatLesson(l, i+1)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		}
		return sb.String()
	}
	return ""
}

func formatLesson(lesson any, index int) string {
	if lesson == nil {
		return ""
	}
	switch v := lesson.(type) {
	case map[string]any:
		return formatSingleLesson(v, index)
	case []any:
		var parts []string
		for _, sub := range v {
			if subMap, ok := sub.(map[string]any); ok {
				text := formatSingleLesson(subMap, index)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func formatSingleLesson(m map[string]any, index int) string {
	var parts []string
	if sub, ok := m["subgroup"].(float64); ok && sub > 0 {
		parts = append(parts, fmt.Sprintf("%d.", int(sub)))
	}
	if lesson, ok := m["lesson"].(string); ok {
		parts = append(parts, lesson)
	}
	if typ, ok := m["type"].(string); ok && typ != "" {
		parts = append(parts, fmt.Sprintf("(%s)", typ))
	}
	if comment, ok := m["comment"].(string); ok && comment != "" {
		parts = append(parts, fmt.Sprintf("[%s]", comment))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%d. %s", index, strings.Join(parts, " "))
}
