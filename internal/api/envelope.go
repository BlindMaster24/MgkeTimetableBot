package api

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

var weekdayNames = [...]string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

func weekdayName(date time.Time) string {
	return weekdayNames[date.Weekday()]
}

func nameKey(value string) (int, float64, string) {
	trimmed := strings.TrimSpace(value)
	if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return 0, parsed, value
	}
	return 1, 0, value
}

func sortNames(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ki, ni, si := nameKey(sorted[i])
		kj, nj, sj := nameKey(sorted[j])
		if ki != kj {
			return ki < kj
		}
		if ki == 0 && ni != nj {
			return ni < nj
		}
		return si < sj
	})
	return sorted
}

func decorateDays(entry map[string]any) []any {
	raw, ok := entry["days"].([]any)
	if !ok {
		return nil
	}
	days := make([]any, 0, len(raw))
	for _, item := range raw {
		day, ok := item.(map[string]any)
		if !ok {
			days = append(days, item)
			continue
		}
		decorated := map[string]any{}
		if date, ok := day["day"].(string); ok {
			if parsed, err := time.Parse("02.01.2006", date); err == nil {
				decorated["weekday"] = weekdayName(parsed)
			}
		}
		for key, value := range day {
			decorated[key] = value
		}
		days = append(days, decorated)
	}
	return days
}
