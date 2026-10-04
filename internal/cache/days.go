package cache

import (
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func (c *RaspCache) TimetableDays(kind, date string) map[string]timetable.Day {
	out := make(map[string]timetable.Day)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if kind == KindTeachers {
		if c.Teachers == nil {
			return out
		}
		for name, schedule := range c.Teachers.Timetable {
			for _, day := range schedule.Days {
				if day.Date == date {
					out[name] = day
					break
				}
			}
		}
		return out
	}
	if kind != KindGroups || c.Groups == nil {
		return out
	}
	for name, schedule := range c.Groups.Timetable {
		for _, day := range schedule.Days {
			if day.Date == date {
				out[name] = day
				break
			}
		}
	}
	return out
}
