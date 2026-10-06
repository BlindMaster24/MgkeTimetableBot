package notification

import (
	"math/rand"
	"sort"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/reminder"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type reminderKey struct {
	kind  string
	value string
	chat  int64
	start int64
}

func (n *EventNotifier) RemindUpcoming(now time.Time, lead, window time.Duration, r *rand.Rand) {
	if lead <= 0 {
		return
	}
	n.pruneReminders(now)
	schedule := reminder.Schedule{Weekdays: n.cache.GetCallsWeekdays(), Saturday: n.cache.GetCallsSaturday()}
	date := timetable.FormatDate(now)
	for _, kind := range []string{cache.KindGroups, cache.KindTeachers} {
		days := n.cache.TimetableDays(kind, date)
		values := make([]string, 0, len(days))
		for value := range days {
			values = append(values, value)
		}
		sort.Strings(values)
		for _, value := range values {
			n.remindValue(kind, value, days[value], now, schedule, lead, window, r)
		}
	}
}

func (n *EventNotifier) remindValue(kind, value string, day timetable.Day, now time.Time, schedule reminder.Schedule, lead, window time.Duration, r *rand.Rand) {
	next, ok := reminder.NextLesson(day, now, schedule)
	if !ok {
		return
	}
	chats := n.reminderChats(kind, value)
	if len(chats) == 0 {
		return
	}
	text := n.locData("notify_lesson_reminder", map[string]any{
		"Subject": next.Lesson.Subject,
		"Time":    next.Start.Format("15:04"),
		"Room":    next.Lesson.Room,
	})
	for _, chat := range chats {
		if !n.reminderDue(chat, kind, value, next.Start, lead, window, now, r) {
			continue
		}
		target := chat.ID
		if chat.PeerID != 0 {
			target = chat.PeerID
		}
		if err := n.sender.SendText(target, text); err != nil {
			n.log.Error().Err(err).Int64("chatID", target).Msg("lesson reminder send failed")
		}
	}
}

func (n *EventNotifier) reminderDue(chat *EventChat, kind, value string, start time.Time, lead, window time.Duration, now time.Time, r *rand.Rand) bool {
	chatLead := n.chatLead(chat, lead)
	key := reminderKey{kind: kind, value: value, chat: chat.ID, start: start.Unix()}

	n.remMu.Lock()
	defer n.remMu.Unlock()

	if n.remSent[key] {
		return false
	}
	if n.remDue == nil {
		n.remDue = make(map[reminderKey]time.Time)
	}
	fireAt, known := n.remDue[key]
	if !known {
		fireAt = reminder.ReminderTime(start, chatLead, window, r)
		n.remDue[key] = fireAt
	}
	if now.Before(fireAt) {
		return false
	}
	if n.remSent == nil {
		n.remSent = make(map[reminderKey]bool)
	}
	n.remSent[key] = true
	delete(n.remDue, key)
	return true
}

func (n *EventNotifier) chatLead(chat *EventChat, fallback time.Duration) time.Duration {
	if chat != nil && chat.ReminderLeadMinutes > 0 {
		return time.Duration(chat.ReminderLeadMinutes) * time.Minute
	}
	return fallback
}

func (n *EventNotifier) reminderChats(kind, value string) []*EventChat {
	var base, subs []*EventChat
	var err error
	if kind == cache.KindTeachers {
		base, err = n.chats.FindChatsByTeachers("telegram", []string{value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByTeacher("telegram", value, true)
		}
	} else {
		base, err = n.chats.FindChatsByGroups("telegram", []string{value}, true)
		if err == nil {
			subs, err = n.chats.FindSubscribedChatsByGroup("telegram", value, true)
		}
	}
	if err != nil {
		n.log.Error().Err(err).Msg("failed to find chats for lesson reminder")
		return nil
	}
	return n.mergeChats(base, subs)
}

func (n *EventNotifier) TrackedReminders() int {
	n.remMu.Lock()
	defer n.remMu.Unlock()
	return len(n.remSent) + len(n.remDue)
}

func (n *EventNotifier) pruneReminders(now time.Time) {
	cutoff := now.Add(-time.Hour).Unix()
	n.remMu.Lock()
	defer n.remMu.Unlock()
	for key := range n.remDue {
		if key.start < cutoff {
			delete(n.remDue, key)
		}
	}
	for key := range n.remSent {
		if key.start < cutoff {
			delete(n.remSent, key)
		}
	}
}
