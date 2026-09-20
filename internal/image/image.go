package image

import (
	"fmt"
	"time"
)

func RenderGroupDays(group string, dayMaps []map[string]any, outputDir string) (string, error) {
	days := daysFromMaps(dayMaps, true)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	return r.RenderGroupImage(group, days)
}

func RenderTeacherDays(teacher string, dayMaps []map[string]any, outputDir string) (string, error) {
	days := daysFromMaps(dayMaps, false)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	return r.RenderTeacherImage(teacher, days)
}

func daysFromMaps(dayMaps []map[string]any, isGroup bool) []DayData {
	now := time.Now()
	today := now.Format("02.01.2006")
	tomorrow := now.AddDate(0, 0, 1).Format("02.01.2006")

	var result []DayData
	for _, dayMap := range dayMaps {
		dateStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)

		wd := weekdayName(dateStr)

		hint := ""
		if isGroup {
			if dateStr == today {
				hint = "(сегодня)"
			} else if dateStr == tomorrow {
				hint = "(завтра)"
			}
		}

		var lessonRows []LessonRow
		for i, l := range lessons {
			var cells []string
			if isGroup {
				cells = FormatGroupLesson(l, i+1)
			} else {
				cells = FormatTeacherLesson(l, i+1)
			}
			if len(cells) > 0 {
				lessonRows = append(lessonRows, LessonRow{Number: i + 1, Cells: cells})
			}
		}

		result = append(result, DayData{
			Date:    dateStr,
			Weekday: wd + " " + hint,
			Lessons: lessonRows,
		})
	}
	return result
}
