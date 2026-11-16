package image

import (
	"fmt"
	"time"
)

const renderRetention = 24 * time.Hour

func RenderGroupDays(group string, dayMaps []map[string]any, outputDir string, texts Texts, stampFormat string) (string, error) {
	days := daysFromMaps(dayMaps, true, texts)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	r.SetTexts(texts)
	r.SetStampFormat(stampFormat)
	path, err := r.RenderGroupImage(group, days)
	if err != nil {
		return "", err
	}
	r.Cleanup(renderRetention)
	return path, nil
}

func RenderTeacherDays(teacher string, dayMaps []map[string]any, outputDir string, texts Texts, stampFormat string) (string, error) {
	days := daysFromMaps(dayMaps, false, texts)
	if len(days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	r := NewRenderer(outputDir)
	r.SetTexts(texts)
	r.SetStampFormat(stampFormat)
	path, err := r.RenderTeacherImage(teacher, days)
	if err != nil {
		return "", err
	}
	r.Cleanup(renderRetention)
	return path, nil
}

func daysFromMaps(dayMaps []map[string]any, isGroup bool, texts Texts) []DayData {
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
			if dateStr == today && texts.Today != "" {
				hint = "(" + texts.Today + ")"
			} else if dateStr == tomorrow && texts.Tomorrow != "" {
				hint = "(" + texts.Tomorrow + ")"
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
