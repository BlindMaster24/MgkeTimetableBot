package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

const (
	KindGroups   = "groups"
	KindTeachers = "teachers"

	DayEventAdd    = "add"
	DayEventUpdate = "update"
)

type DayEvent struct {
	Kind  string
	Value string
	Day   map[string]any
	Type  string
	Diff  *schedulediff.Result
}

type DayChange struct {
	Kind  string
	Value string
	Date  string
}

type CallsEvent struct {
	WeekdaysChanged bool
	SaturdayChanged bool
	Reason          string
	Schedule        CallsSchedule
}

type WeekEvent struct {
	Kind      string
	Week      int
	Withdrawn bool
	Entries   []string
}

type Event struct {
	Day   *DayEvent
	Calls *CallsEvent
	Week  *WeekEvent
}

func entryDays(v any) []any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	days, _ := m["days"].([]any)
	return days
}

func dayIndex(dateStr string) int {
	t, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return 0
	}
	return utils.DayIndexFromDate(t)
}

func lessonsJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func mergeDays(newDays, oldDays []any) (merged, added, changed []any) {
	days := make(map[string]any, len(oldDays)+len(newDays))
	order := make([]string, 0, len(oldDays)+len(newDays))

	for _, d := range oldDays {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		date, _ := m["day"].(string)
		if _, seen := days[date]; !seen {
			order = append(order, date)
		}
		days[date] = d
	}

	for _, nd := range newDays {
		nm, ok := nd.(map[string]any)
		if !ok {
			continue
		}
		date, _ := nm["day"].(string)
		od, existed := days[date]
		if !existed {
			added = append(added, nd)
		} else if dayLessonsChanged(od.(map[string]any), nm) {
			changed = append(changed, nd)
		}
		if _, seen := days[date]; !seen {
			order = append(order, date)
		}
		days[date] = nd
	}

	for _, date := range order {
		if d, ok := days[date]; ok {
			merged = append(merged, d)
		}
	}
	return merged, added, changed
}

type dayEventOut struct {
	ev     Event
	evType string
}

func collectEntryDayEvents(kind, value string, oldEntry, newEntry map[string]any, todayIdx int) ([]dayEventOut, []DayChange) {
	oldDays := entryDays(oldEntry)
	newDays := entryDays(newEntry)

	added, _, changed := mergeDays(newDays, oldDays)
	if len(added) == 0 && len(changed) == 0 {
		return nil, nil
	}

	changes := make([]DayChange, 0, len(added)+len(changed))
	for _, day := range append(append([]any{}, added...), changed...) {
		m, ok := day.(map[string]any)
		if !ok {
			continue
		}
		date, _ := m["day"].(string)
		if date == "" {
			continue
		}
		changes = append(changes, DayChange{Kind: kind, Value: value, Date: date})
	}

	lastNoticed := entryLastNoticedDayFromMap(oldEntry)

	oldByDate := make(map[string]map[string]any, len(oldDays))
	for _, d := range oldDays {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if date, _ := m["day"].(string); date != "" {
			oldByDate[date] = m
		}
	}

	var out []dayEventOut
	for _, cd := range changed {
		cm, _ := cd.(map[string]any)
		if cm == nil {
			continue
		}
		date, _ := cm["day"].(string)
		idx := dayIndex(date)

		var evType string
		if idx == todayIdx {
			evType = DayEventUpdate
		} else if idx == todayIdx+1 {
			if lastNoticed == int64(idx) {
				evType = DayEventUpdate
			} else {
				evType = DayEventAdd
			}
		}
		if evType != "" {
			var diff *schedulediff.Result
			if old, ok := oldByDate[date]; ok {
				if result, comparable := diffDayLessons(old, cm); comparable && !result.Empty() {
					diff = &result
				}
			}
			out = append(out, dayEventOut{ev: Event{Day: &DayEvent{Kind: kind, Value: value, Day: cm, Type: evType, Diff: diff}}, evType: evType})
		}
	}
	return out, changes
}

func entryLastNoticedDayFromMap(entry map[string]any) int64 {
	if entry == nil {
		return 0
	}
	last, _ := entry["lastNoticedDay"].(float64)
	return int64(last)
}

func (c *RaspCache) LastNoticedDay(kind, value string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	view := c.groupsAny
	if kind == KindTeachers {
		view = c.teachersAny
	}
	entry, _ := view[value].(map[string]any)
	return entryLastNoticedDayFromMap(entry)
}

func (c *RaspCache) SetLastNoticedDay(kind, value string, dayIdx int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if kind == KindTeachers {
		if m, ok := c.teachersAny[value].(map[string]any); ok {
			cp := make(map[string]any, len(m)+1)
			for k, v := range m {
				cp[k] = v
			}
			cp["lastNoticedDay"] = float64(dayIdx)
			c.teachersAny[value] = cp
		}
		if schedule, ok := c.Teachers.Timetable[value]; ok {
			schedule.LastNoticed = dayIdx
			c.Teachers.Timetable[value] = schedule
		}
		return
	}
	if m, ok := c.groupsAny[value].(map[string]any); ok {
		cp := make(map[string]any, len(m)+1)
		for k, v := range m {
			cp[k] = v
		}
		cp["lastNoticedDay"] = float64(dayIdx)
		c.groupsAny[value] = cp
	}
	if schedule, ok := c.Groups.Timetable[value]; ok {
		schedule.LastNoticed = dayIdx
		c.Groups.Timetable[value] = schedule
	}
}

func (c *RaspCache) DrainEvents(ctx context.Context) []Event {
	if err := ctx.Err(); err != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.events) == 0 {
		return nil
	}
	evs := c.events
	c.events = nil
	return evs
}

func (c *RaspCache) DrainDayChanges(ctx context.Context) []DayChange {
	if err := ctx.Err(); err != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.dayChanges) == 0 {
		return nil
	}
	changes := c.dayChanges
	c.dayChanges = nil
	return dedupeDayChanges(changes)
}

func dedupeDayChanges(changes []DayChange) []DayChange {
	if len(changes) == 0 {
		return nil
	}

	seen := make(map[DayChange]bool, len(changes))
	result := make([]DayChange, 0, len(changes))
	for _, change := range changes {
		if change.Date == "" || change.Value == "" {
			continue
		}
		if seen[change] {
			continue
		}
		seen[change] = true
		result = append(result, change)
	}
	return result
}

func (c *RaspCache) GroupKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.Groups.Timetable))
	for k := range c.Groups.Timetable {
		keys = append(keys, k)
	}
	return keys
}

func (c *RaspCache) TeacherKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.Teachers.Timetable))
	for k := range c.Teachers.Timetable {
		keys = append(keys, k)
	}
	return keys
}

func collectWeekEvents(kind string, previousWeekIndex int, data map[string]any) (events []Event, maxWeek int) {
	maxWeek = weekOfTimetable(data)

	if previousWeekIndex > 0 && maxWeek > previousWeekIndex {
		events = append(events, Event{Week: &WeekEvent{Kind: kind, Week: maxWeek}})
	}
	return events, maxWeek
}

func collectWeekEntries(tt map[string]any, week int) []string {
	var result []string
	for value, v := range tt {
		for _, d := range entryDays(v) {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			date, _ := m["day"].(string)
			lessons, _ := m["lessons"].([]any)
			t, err := time.Parse("02.01.2006", date)
			if err != nil {
				continue
			}
			if utils.WeekIndexFromDate(t).Value() == week && len(lessons) > 0 {
				result = append(result, value)
				break
			}
		}
	}
	return result
}

func weekOfTimetable(tt map[string]any) int {
	maxWeek := 0
	for _, v := range tt {
		for _, d := range entryDays(v) {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			date, _ := m["day"].(string)
			t, err := time.Parse("02.01.2006", date)
			if err != nil {
				continue
			}
			if w := utils.WeekIndexFromDate(t).Value(); w > maxWeek {
				maxWeek = w
			}
		}
	}
	return maxWeek
}
