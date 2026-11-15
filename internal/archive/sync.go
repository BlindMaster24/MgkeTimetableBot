package archive

import (
	"context"
	"database/sql"
	"strconv"
)

func (r *Repository) FlushCache(ctx context.Context, groups, teachers map[string]any) (int, error) {
	entries := cacheEntries(groups, teachers)
	if len(entries) == 0 {
		return 0, nil
	}

	return len(entries), r.AppendDays(ctx, entries)
}

type pendingDay struct {
	entry   AppendDay
	day     int64
	payload string
}

func (r *Repository) SyncFromCache(ctx context.Context, groups, teachers map[string]any) error {
	pending := pendingDays(cacheEntries(groups, teachers))
	if len(pending) == 0 {
		return nil
	}

	minDay, maxDay := pending[0].day, pending[0].day
	for _, day := range pending[1:] {
		if day.day < minDay {
			minDay = day.day
		}
		if day.day > maxDay {
			maxDay = day.day
		}
	}

	stored, err := r.storedDays(ctx, minDay, maxDay)
	if err != nil {
		return err
	}

	changed := make([]AppendDay, 0, len(pending))
	for _, day := range pending {
		if payload, ok := stored[dayKey(day.entry.Type, day.entry.Value, day.day)]; ok && payload == day.payload {
			continue
		}
		changed = append(changed, day.entry)
	}

	if len(changed) == 0 {
		return nil
	}
	return r.AppendDays(ctx, changed)
}

func pendingDays(entries []AppendDay) []pendingDay {
	pending := make([]pendingDay, 0, len(entries))
	for _, entry := range entries {
		day, payload := extractDayData(entry.Day)
		if day <= 0 {
			continue
		}
		pending = append(pending, pendingDay{entry: entry, day: day, payload: payload})
	}
	return pending
}

func dayKey(kind, value string, day int64) string {
	return kind + "\x00" + value + "\x00" + strconv.FormatInt(day, 10)
}

func (r *Repository) storedDays(ctx context.Context, minDay, maxDay int64) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT day, "group", teacher, data FROM timetable_archive WHERE day BETWEEN ? AND ?`, minDay, maxDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stored := make(map[string]string)
	for rows.Next() {
		var day int64
		var group, teacher sql.NullString
		var data string
		if err := rows.Scan(&day, &group, &teacher, &data); err != nil {
			return nil, err
		}
		if group.Valid {
			stored[dayKey("group", group.String, day)] = data
		}
		if teacher.Valid {
			stored[dayKey("teacher", teacher.String, day)] = data
		}
	}
	return stored, rows.Err()
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
