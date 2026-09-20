package archive

import (
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func (r *Repository) FlushCache(groups, teachers map[string]any) (int, error) {
	entries := cacheEntries(groups, teachers)
	if len(entries) == 0 {
		return 0, nil
	}

	return len(entries), r.AppendDays(entries)
}

func (r *Repository) SyncFromCache(groups, teachers map[string]any) error {
	entries := cacheEntries(groups, teachers)

	if len(entries) == 0 {
		return nil
	}

	cacheMaxDay := int64(0)
	for _, e := range entries {
		if idx := dayIndexOf(e.Day); idx > cacheMaxDay {
			cacheMaxDay = idx
		}
	}

	dbMaxDay := int64(0)
	if bounds, err := r.DayIndexBounds(); err == nil {
		dbMaxDay = bounds.Max
	}

	if cacheMaxDay <= dbMaxDay {
		return nil
	}

	return r.AppendDays(entries)
}

func cacheEntries(groups, teachers map[string]any) []AppendDay {
	entries := make([]AppendDay, 0, 512)

	for group, v := range groups {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		daysArr, ok := m["days"].([]any)
		if !ok {
			continue
		}
		for _, d := range daysArr {
			entries = append(entries, AppendDay{Type: "group", Value: group, Day: d})
		}
	}

	for teacher, v := range teachers {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		daysArr, ok := m["days"].([]any)
		if !ok {
			continue
		}
		for _, d := range daysArr {
			entries = append(entries, AppendDay{Type: "teacher", Value: teacher, Day: d})
		}
	}

	return entries
}

func dayIndexOf(v interface{}) int64 {
	switch d := v.(type) {
	case map[string]any:
		dateStr, _ := d["day"].(string)
		return DateToDayIndex(dateStr)
	case *model.GroupDay:
		return DateToDayIndex(d.Day)
	case *model.TeacherDay:
		return DateToDayIndex(d.Day)
	}
	return 0
}
