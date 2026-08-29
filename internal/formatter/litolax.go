package formatter

import (
	"fmt"
	"strings"
)

type LitolaxFormatter struct{}

func (f *LitolaxFormatter) Name() string  { return "litolax" }
func (f *LitolaxFormatter) Label() string { return "💩 LitolaxStyle" }

func (f *LitolaxFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *LitolaxFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *LitolaxFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			text = append(text, "Группа: "+opts.b(name))
		} else {
			text = append(text, "Преподаватель: "+opts.b(teacher))
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

func (f *LitolaxFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return "\nДень - " + w + ", " + day.Date + "\n"
}

func (f *LitolaxFormatter) formatLessons(lessons []any, opts FormatOptions) string {
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

		lessonHeader := "\n" + opts.b(fmt.Sprintf("Пара: №%d", i+1))
		text = append(text, lessonHeader)

		withSubgroups := len(subs) > 1

		if !withSubgroups {
			mainLesson := f.formatLessonLine(subs[0], opts)
			text = append(text, mainLesson)

			if subs[0].Cabinet != "" {
				text = append(text, "Каб: "+subs[0].Cabinet)
			}
		} else {
			for _, sub := range subs {
				value := f.formatLessonLine(sub, opts)
				text = append(text, value)

				cab := sub.Cabinet
				if cab == "" {
					cab = "-"
				}
				text = append(text, "Каб: "+cab)
			}
		}
	}

	return strings.Join(text, "\n")
}

func (f *LitolaxFormatter) formatLessonLine(p LessonPart, opts FormatOptions) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	if p.Lesson != "" {
		parts = append(parts, p.Lesson)
	}

	if p.Type != "" {
		parts = append(parts, "("+p.Type+")")
	}

	if p.Teacher != "" {
		parts = append(parts, p.Teacher)
	}

	if p.Comment != "" {
		parts = append(parts, "// "+p.Comment)
	}

	return strings.Join(parts, " ")
}
