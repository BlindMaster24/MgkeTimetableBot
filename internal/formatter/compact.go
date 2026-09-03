package formatter

import (
	"fmt"
	"strings"
)

type CompactFormatter struct{}

func (f *CompactFormatter) Name() string  { return "compact" }
func (f *CompactFormatter) Label() string { return "Компактный" }

func (f *CompactFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *CompactFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *CompactFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			text = append(text, "Группа '"+name+"'")
		} else {
			text = append(text, "Преподаватель '"+opts.getFullTeacherName(teacher)+"'")
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

func (f *CompactFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return "__ " + opts.b(w) + ", " + day.Date + " __"
}

func (f *CompactFormatter) formatLessons(lessons []any, opts FormatOptions) string {
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

		lessonsEqual := allEqual(func(p LessonPart) string { return p.Lesson }, subs)

		showOpts := map[string]bool{
			"lesson":  true,
			"cabinet": true,
		}
		if len(subs) == 1 || lessonsEqual {
			showOpts["lesson"] = true
		}

		mainLesson := f.formatLessonLine(subs[0], showOpts, opts)
		text = append(text, lessonHeader+mainLesson)

		if len(subs) > 1 {
			for _, sub := range subs {
				value := f.formatLessonLine(sub, showOpts, opts)
				text = append(text, "- "+value)
			}
		}
	}

	return strings.Join(text, "\n")
}

func (f *CompactFormatter) formatLessonLine(p LessonPart, show map[string]bool, opts FormatOptions) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	if show["lesson"] && p.Lesson != "" {
		parts = append(parts, p.Lesson)
	}

	if show["cabinet"] && p.Cabinet != "" {
		parts = append(parts, "{"+p.Cabinet+"}")
	}

	return strings.Join(parts, " ")
}
