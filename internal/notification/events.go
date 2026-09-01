package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

type ChangeNotifier struct {
	cache  *cache.RaspCache
	log    *logger.Logger
	sender Sender
	chats  ChatFinder
}

func NewChangeNotifier(cache *cache.RaspCache, log *logger.Logger, sender Sender, chats ChatFinder) *ChangeNotifier {
	return &ChangeNotifier{
		cache:  cache,
		log:    log,
		sender: sender,
		chats:  chats,
	}
}

func (n *ChangeNotifier) NotifyChanges(oldGroupsHash, oldTeachersHash string) {
	newGroupsHash := n.cache.GetGroupsHash()
	newTeachersHash := n.cache.GetTeachersHash()

	if oldGroupsHash == newGroupsHash && oldTeachersHash == newTeachersHash {
		return
	}

	n.log.Info().
		Str("old_groups", oldGroupsHash).
		Str("new_groups", newGroupsHash).
		Str("old_teachers", oldTeachersHash).
		Str("new_teachers", newTeachersHash).
		Msg("detected schedule changes, sending notifications")

	chats, err := n.chats.FindAllWithNotifications("telegram")
	if err != nil {
		n.log.Error().Err(err).Msg("failed to find chats for change notifications")
		return
	}

	for _, chat := range chats {
		if !chat.NoticeChanges {
			continue
		}

		switch chat.Mode {
		case "student", "parent":
			if chat.Group == "" {
				continue
			}
			if oldGroupsHash != newGroupsHash {
				n.notifyGroupChange(chat, chat.Group)
			}
		case "teacher":
			if chat.Teacher == "" {
				continue
			}
			if oldTeachersHash != newTeachersHash {
				n.notifyTeacherChange(chat, chat.Teacher)
			}
		}
	}
}

func (n *ChangeNotifier) NotifyWeekChange(oldWeekIndex int) {
	newWeek := time.Now().Format("02.01.2006")

	chats, err := n.chats.FindAllWithNotifications("telegram")
	if err != nil {
		return
	}

	for _, chat := range chats {
		if !chat.NoticeChanges {
			continue
		}

		switch chat.Mode {
		case "student", "parent":
			if chat.Group == "" {
				continue
			}
			groups := n.cache.GetGroups()
			if data, ok := groups[chat.Group]; ok {
				msg := n.buildWeekChangeNotification("group", chat.Group, data)
				if msg != "" {
					n.sender.SendText(chat.ID, msg)
				}
			}
		case "teacher":
			if chat.Teacher == "" {
				continue
			}
			teachers := n.cache.GetTeachers()
			if data, ok := teachers[chat.Teacher]; ok {
				msg := n.buildWeekChangeNotification("teacher", chat.Teacher, data)
				if msg != "" {
					n.sender.SendText(chat.ID, msg)
				}
			}
		}
		_ = newWeek
	}
}

func (n *ChangeNotifier) notifyGroupChange(chat *ChatInfo, group string) {
	groups := n.cache.GetGroups()
	data, ok := groups[group]
	if !ok {
		return
	}

	today := time.Now().Format("02.01.2006")
	msg := n.buildDayChangeNotification("group", group, data, today)
	if msg != "" {
		if err := n.sender.SendText(chat.ID, msg); err != nil {
			n.log.Error().Err(err).Int64("chatID", chat.ID).Msg("send group change notification failed")
		}
	}
}

func (n *ChangeNotifier) notifyTeacherChange(chat *ChatInfo, teacher string) {
	teachers := n.cache.GetTeachers()
	data, ok := teachers[teacher]
	if !ok {
		return
	}

	today := time.Now().Format("02.01.2006")
	msg := n.buildDayChangeNotification("teacher", teacher, data, today)
	if msg != "" {
		if err := n.sender.SendText(chat.ID, msg); err != nil {
			n.log.Error().Err(err).Int64("chatID", chat.ID).Msg("send teacher change notification failed")
		}
	}
}

func (n *ChangeNotifier) buildDayChangeNotification(typeName, value string, data any, today string) string {
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

		var label string
		if typeName == "group" {
			label = fmt.Sprintf("Группа %s", value)
		} else {
			label = fmt.Sprintf("Преподаватель %s", value)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\U0001F514 Расписание обновлено\n%s\n\n", label))
		opts := formatter.FormatOptions{ShowHeader: false, IsTelegram: true}
		if typeName == "group" {
			text := formatter.GetByIndex(0).FormatGroupFull(value, []map[string]any{dayMap}, opts)
			sb.WriteString(text)
		} else {
			text := formatter.GetByIndex(0).FormatTeacherFull(value, []map[string]any{dayMap}, opts)
			sb.WriteString(text)
		}
		return sb.String()
	}
	return ""
}

func (n *ChangeNotifier) buildWeekChangeNotification(typeName, value string, data any) string {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok || len(daysArr) == 0 {
		return ""
	}

	var label string
	if typeName == "group" {
		label = fmt.Sprintf("Группа %s", value)
	} else {
		label = fmt.Sprintf("Преподаватель %s", value)
	}

	return fmt.Sprintf("\U0001F4C5 Расписание обновлено\n%s\n\nПоявилось новое расписание на следующую неделю.", label)
}
