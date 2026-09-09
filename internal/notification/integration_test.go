package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func dayWith(lesson, timeStr string) []any {
	return []any{
		map[string]any{"lesson": lesson, "time": timeStr},
	}
}

func entryWithDays(days ...map[string]any) map[string]any {
	list := make([]any, 0, len(days))
	for _, d := range days {
		list = append(list, d)
	}
	return map[string]any{"days": list}
}

func dayMap(date string, lessons []any) map[string]any {
	return map[string]any{"day": date, "lessons": lessons}
}

func cachedDays(c *cache.RaspCache, value string) []map[string]any {
	entry, ok := c.GetGroups()[value].(map[string]any)
	if !ok {
		return nil
	}
	raw, _ := entry["days"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, d := range raw {
		if m, ok := d.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func TestIntegration_TwoDayParseSequence(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	tomorrow := tomorrowStr()
	tomorrowAfter := time.Now().AddDate(0, 0, 2).Format("02.01.2006")

	firstParse := map[string]any{
		"63": entryWithDays(
			dayMap(tomorrow, dayWith("Математика", "08:00 - 08:45")),
		),
	}
	c.SetGroups(firstParse, "hash1")
	n.HandleEvents(c.DrainEvents())

	if len(sender.sent) != 0 {
		t.Fatalf("brand-new entry must not notify (TS: no events without currentEntry), got %d", len(sender.sent))
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != 0 {
		t.Fatalf("lastNoticedDay must be unset after first parse, got %d", last)
	}

	secondParse := map[string]any{
		"63": entryWithDays(
			dayMap(tomorrow, []any{
				map[string]any{"lesson": "Математика", "time": "08:00 - 08:45"},
				map[string]any{"lesson": "Физика", "time": "09:00 - 09:45"},
			}),
		),
	}
	c.SetGroups(secondParse, "hash2")
	n.HandleEvents(c.DrainEvents())

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 add notification after second parse, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "📢 Расписание на завтра") {
		t.Fatalf("expected add header, got %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[0].text, "Физика") {
		t.Fatalf("expected new lesson in add body: %q", sender.sent[0].text)
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != int64(dayIndex(tomorrow)) {
		t.Fatalf("lastNoticedDay must be set to tomorrow (%d), got %d", dayIndex(tomorrow), last)
	}

	thirdParse := map[string]any{
		"63": entryWithDays(
			dayMap(tomorrow, []any{
				map[string]any{"lesson": "Математика", "time": "08:00 - 08:45"},
				map[string]any{"lesson": "Физика", "time": "09:00 - 09:45"},
				map[string]any{"lesson": "Химия", "time": "10:00 - 10:45"},
			}),
			dayMap(tomorrowAfter, dayWith("История", "08:00 - 08:45")),
		),
	}
	c.SetGroups(thirdParse, "hash3")
	n.HandleEvents(c.DrainEvents())

	if len(sender.sent) != 2 {
		t.Fatalf("expected exactly 1 update notification (day-after-tomorrow add must not fire), got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[1].text, "🆕 Изменено расписание") {
		t.Fatalf("expected update header, got %q", sender.sent[1].text)
	}
	if !strings.Contains(sender.sent[1].text, "Химия") {
		t.Fatalf("expected new lesson in update body: %q", sender.sent[1].text)
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != int64(dayIndex(tomorrow)) {
		t.Fatalf("lastNoticedDay must stay at tomorrow after update, got %d", last)
	}

	c.SetGroups(thirdParse, "hash4")
	if events := c.DrainEvents(); len(events) != 0 {
		t.Fatalf("identical re-parse must produce no events, got %d", len(events))
	}
	if len(sender.sent) != 2 {
		t.Fatalf("identical re-parse must not notify, got %d messages", len(sender.sent))
	}

	days := cachedDays(c, "63")
	if len(days) != 2 {
		t.Fatalf("merged cache must keep both days, got %d", len(days))
	}
	var tomorrowDay map[string]any
	for _, d := range days {
		if d["day"] == tomorrow {
			tomorrowDay = d
		}
	}
	if tomorrowDay == nil {
		t.Fatal("merged cache must contain tomorrow's day")
	}
	lessons, _ := tomorrowDay["lessons"].([]any)
	if len(lessons) != 3 {
		t.Fatalf("merged cache must keep updated lessons (3), got %d", len(lessons))
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != int64(dayIndex(tomorrow)) {
		t.Fatalf("lastNoticedDay must survive merge, got %d", last)
	}
}

func TestIntegration_TwoDayParseSequence_NoChats(t *testing.T) {
	n, sender, _, c := newTestNotifier(t)

	tomorrow := tomorrowStr()
	c.SetGroups(map[string]any{
		"63": entryWithDays(dayMap(tomorrow, dayWith("Математика", "08:00 - 08:45"))),
	}, "hash1")
	n.HandleEvents(c.DrainEvents())

	c.SetGroups(map[string]any{
		"63": entryWithDays(dayMap(tomorrow, []any{
			map[string]any{"lesson": "Математика", "time": "08:00 - 08:45"},
			map[string]any{"lesson": "Физика", "time": "09:00 - 09:45"},
		})),
	}, "hash2")
	n.HandleEvents(c.DrainEvents())

	if len(sender.sent) != 0 {
		t.Fatalf("no matched chats, expected no sends, got %d", len(sender.sent))
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != int64(dayIndex(tomorrow)) {
		t.Fatalf("lastNoticedDay must be set even without recipients (TS controller), got %d", last)
	}
}

func TestIntegration_TwoDayParseSequence_FilteredLessons(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	n.cfg.Parser.AlertableIgnoreFilter.Group = []config.LessonFilter{{Lesson: "Физика"}}

	tomorrow := tomorrowStr()
	c.SetGroups(map[string]any{
		"63": entryWithDays(dayMap(tomorrow, dayWith("Физика", "08:00 - 08:45"))),
	}, "hash1")
	n.HandleEvents(c.DrainEvents())

	c.SetGroups(map[string]any{
		"63": entryWithDays(dayMap(tomorrow, dayWith("Физика", "09:00 - 09:45"))),
	}, "hash2")
	n.HandleEvents(c.DrainEvents())

	if len(sender.sent) != 0 {
		t.Fatalf("all-filtered day must not notify, got %d", len(sender.sent))
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last != int64(dayIndex(tomorrow)) {
		t.Fatalf("lastNoticedDay must be set even when filtered (TS controller), got %d", last)
	}
}
