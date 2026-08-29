package image

import (
	"fmt"
	"time"
)

func RenderGroupFromCache(group string, data any, outputDir string) (string, error) {
	days := extractDaysForImage(data)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	return r.RenderGroupImage(group, days)
}

func RenderTeacherFromCache(teacher string, data any, outputDir string) (string, error) {
	days := extractDaysForImage(data)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	return r.RenderTeacherImage(teacher, days)
}

func extractDaysForImage(data any) []DayData {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return nil
	}

	now := time.Now()
	today := now.Format("02.01.2006")
	tomorrow := now.AddDate(0, 0, 1).Format("02.01.2006")

	var result []DayData
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dateStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)

		wd := weekdayName(dateStr)

		hint := ""
		if dateStr == today {
			hint = "(сегодня)"
		} else if dateStr == tomorrow {
			hint = "(завтра)"
		}

		var lessonRows []LessonRow
		for i, l := range lessons {
			cells := FormatGroupLesson(l, i+1)
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

func extractTeacherDaysForImage(data any) []DayData {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return nil
	}

	var result []DayData
	for _, d := range daysArr {
		dayMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dateStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)

		wd := weekdayName(dateStr)

		var lessonRows []LessonRow
		for i, l := range lessons {
			cells := FormatTeacherLesson(l, i+1)
			if len(cells) > 0 {
				lessonRows = append(lessonRows, LessonRow{Number: i + 1, Cells: cells})
			}
		}

		result = append(result, DayData{
			Date:    dateStr,
			Weekday: wd,
			Lessons: lessonRows,
		})
	}
	return result
}


