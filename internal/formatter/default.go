package formatter

import (
	"fmt"
	"strings"
	"time"
)

type DefaultFormatter struct{}

func (f *DefaultFormatter) Name() string  { return "default" }
func (f *DefaultFormatter) Label() string { return "📝 Структурированный" }

func (f *DefaultFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	return f.formatFull(group, "", parseDaysFromSlice(days), true, opts)
}

func (f *DefaultFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	return f.formatFull("", teacher, parseDaysFromSlice(days), false, opts)
}

func (f *DefaultFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			text = append(text, fmt.Sprintf("- Группа '%s' -", name))
		} else {
			text = append(text, fmt.Sprintf("- Преподаватель '%s' -", opts.getFullTeacherName(teacher)))
		}
	}

	if opts.WeekLabel != "" {
		text = append(text, opts.WeekLabel)
	}

	if len(days) > 0 {
		for _, day := range days {
			dayText := f.formatDayHeader(day, opts)
			lessonsText := f.formatLessons(day.Lessons, opts)
			text = append(text, dayText+"\n"+lessonsText)
		}
	} else {
		text = append(text, notTimetable())
	}

	footer := formatFooter(opts)
	if strings.TrimSpace(footer) != "" {
		text = append(text, footer)
	}

	return strings.Join(text, "\n\n")
}

func (f *DefaultFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return "__ " + opts.b(w) + ", " + day.Date + " __"
}

func (f *DefaultFormatter) formatLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return notLessons()
	}

	var text []string
	for i, lesson := range lessons {
		if lesson == nil {
			continue
		}

		subs := getSubgroups(lesson)
		if len(subs) == 0 {
			continue
		}

		lessonHeader := fmt.Sprintf("%d. ", i+1)

		withSubgroups := len(subs) > 1
		lessonsEqual := allEqual(func(p LessonPart) string { return p.Lesson }, subs)
		typeEqual := lessonsEqual && allEqual(func(p LessonPart) string { return p.Type }, subs)
		teacherEqual := typeEqual && allEqual(func(p LessonPart) string { return p.Teacher }, subs)
		cabinetEqual := teacherEqual && allEqual(func(p LessonPart) string { return p.Cabinet }, subs)
		commentEqual := allEqual(func(p LessonPart) string { return p.Comment }, subs)

		showOpts := map[string]bool{
			"subgroup": !withSubgroups,
			"lesson":   !withSubgroups || lessonsEqual,
			"type":     !withSubgroups || typeEqual,
			"teacher":  !withSubgroups || teacherEqual,
			"cabinet":  !withSubgroups || cabinetEqual,
			"comment":  !withSubgroups || commentEqual,
		}

		mainLesson := f.formatLessonLine(subs[0], showOpts, opts)
		text = append(text, lessonHeader+mainLesson)

		if withSubgroups {
			reverseOpts := map[string]bool{}
			for k, v := range showOpts {
				reverseOpts[k] = !v
			}
			var subLines []string
			for j, sub := range subs {
				value := f.formatLessonLine(sub, reverseOpts, opts)
				prefix := "├── "
				if j == len(subs)-1 {
					prefix = "└── "
				}
				subLines = append(subLines, prefix+value)
			}
			text = append(text, strings.Join(subLines, "\n"))
		}
	}

	return strings.Join(text, "\n")
}

func (f *DefaultFormatter) formatLessonLine(p LessonPart, show map[string]bool, opts FormatOptions) string {
	var parts []string

	if show["subgroup"] && p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}
	if show["lesson"] && p.Lesson != "" {
		parts = append(parts, p.Lesson)
	}
	if show["type"] && p.Type != "" {
		parts = append(parts, "("+p.Type+")")
	}
	if show["teacher"] && p.Teacher != "" {
		parts = append(parts, p.Teacher)
	}
	if show["cabinet"] && p.Cabinet != "" {
		parts = append(parts, "{"+p.Cabinet+"}")
	}
	if show["comment"] && p.Comment != "" {
		parts = append(parts, "// "+p.Comment)
	}

	return strings.Join(parts, " ")
}

func parseDaysFromSlice(days []map[string]any) []DayInfo {
	if len(days) == 0 {
		return nil
	}

	now := time.Now()
	today := now.Format("02.01.2006")
	tomorrow := now.AddDate(0, 0, 1).Format("02.01.2006")

	var result []DayInfo
	for _, dayMap := range days {
		dateStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)

		wd := ""
		if t, err := time.Parse("02.01.2006", dateStr); err == nil {
			wd = weekdayNames[t.Weekday()]
		}

		hint := ""
		if dateStr == today {
			hint = "(сегодня)"
		} else if dateStr == tomorrow {
			hint = "(завтра)"
		}

		result = append(result, DayInfo{
			Date:    dateStr,
			Weekday: wd,
			Hint:    hint,
			Lessons: lessons,
		})
	}
	return result
}
