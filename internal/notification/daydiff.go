package notification

import (
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
)

var defaultLocalizer = i18n.New("ru")

func (n *EventNotifier) SetLocalizer(loc *i18n.Localizer) {
	n.localizer = loc
}

func (n *EventNotifier) loc(key string) string {
	return n.locData(key, nil)
}

func (n *EventNotifier) locData(key string, data map[string]any) string {
	if n.localizer != nil {
		return n.localizer.T("ru", key, data)
	}
	return defaultLocalizer.T("ru", key, data)
}

func (n *EventNotifier) changeSummary(ev *cache.DayEvent) string {
	if ev.Diff == nil || ev.Diff.Empty() {
		return ""
	}
	lines := []string{n.loc("notify_diff_header")}
	for _, change := range ev.Diff.Of(schedulediff.Added) {
		lines = append(lines, n.locData("notify_diff_added", map[string]any{"Subject": change.New.Subject}))
	}
	for _, change := range ev.Diff.Of(schedulediff.Removed) {
		lines = append(lines, n.locData("notify_diff_removed", map[string]any{"Subject": change.Old.Subject}))
	}
	for _, change := range ev.Diff.Of(schedulediff.Moved) {
		lines = append(lines, n.locData("notify_diff_moved", map[string]any{
			"Subject": change.New.Subject,
			"From":    change.From,
			"To":      change.To,
		}))
	}
	for _, change := range ev.Diff.Of(schedulediff.RoomChanged) {
		lines = append(lines, n.locData("notify_diff_room", map[string]any{
			"Subject": change.New.Subject,
			"Room":    change.New.Room,
		}))
	}
	for _, change := range ev.Diff.Of(schedulediff.TeacherChanged) {
		lines = append(lines, n.locData("notify_diff_teacher", map[string]any{
			"Subject": change.New.Subject,
			"Teacher": change.New.Teacher,
		}))
	}
	for _, change := range ev.Diff.Of(schedulediff.TimeChanged) {
		text, _ := change.New.Extra["time"].(string)
		lines = append(lines, n.locData("notify_diff_time", map[string]any{
			"Subject": change.New.Subject,
			"Time":    text,
		}))
	}
	return strings.Join(lines, "\n")
}
