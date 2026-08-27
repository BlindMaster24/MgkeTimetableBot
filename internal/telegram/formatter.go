package telegram

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func formatGroupDay(data any) string {
	return formatScheduleDay(data, false)
}

func formatTeacherDay(data any) string {
	return formatScheduleDay(data, true)
}

func formatScheduleDay(data any, isTeacher bool) string {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return ""
	}

	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return ""
	}

	now := time.Now()
	today := now.Format("02.01.2006")
	yesterday := now.AddDate(0, 0, -1).Format("02.01.2006")

	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		if dayStr == today || dayStr == yesterday {
			continue
		}
		if dayStr != today {
			continue
		}
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

	if targetDay == "" && len(daysArr) > 0 {
		dayMap, _ := daysArr[0].(map[string]any)
		targetDay, _ = dayMap["day"].(string)
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
		var sb strings.Builder
		dayName := getDayName(dayStr)
		sb.WriteString(fmt.Sprintf("📅 %s (%s)\n", dayStr, dayName))
		if len(lessons) == 0 {
			sb.WriteString("Пар нет")
		} else {
			for i, l := range lessons {
				text := formatLesson(l, i+1)
				if text != "" {
					sb.WriteString(text)
					sb.WriteString("\n")
				}
			}
		}
		return sb.String()
	}

	return ""
}

func formatGroupWeek(data any) string {
	return formatScheduleWeek(data, false)
}

func formatTeacherWeek(data any) string {
	return formatScheduleWeek(data, true)
}

func formatScheduleWeek(data any, isTeacher bool) string {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return ""
	}

	var sb strings.Builder
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dayStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)
		dayName := getDayName(dayStr)
		sb.WriteString(fmt.Sprintf("📅 %s (%s)\n", dayStr, dayName))
		if len(lessons) == 0 {
			sb.WriteString("Пар нет\n")
		} else {
			for i, l := range lessons {
				text := formatLesson(l, i+1)
				if text != "" {
					sb.WriteString(text)
					sb.WriteString("\n")
				}
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
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

func anyToMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	data, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(data, &m)
	return m
}
