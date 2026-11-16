package formatter

import (
	"fmt"
	"strings"
)

type CompactFormatter struct{}

func (f *CompactFormatter) Name() string { return "compact" }
func (f *CompactFormatter) Label(opts FormatOptions) string {
	return opts.formatLabel("formatter_label_compact")
}
func (f *CompactFormatter) NoTimetable(opts FormatOptions) string {
	return opts.formatLabel("no_timetable")
}

func (f *CompactFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *CompactFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *CompactFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			if header := opts.formatLabel("formatter_group_compact", name); header != "" {
				text = append(text, header)
			}
		} else {
			if header := opts.formatLabel("formatter_teacher_compact", opts.getFullTeacherName(teacher)); header != "" {
				text = append(text, header)
			}
		}
	}

	if opts.WeekLabel != "" {
		text = append(text, opts.WeekLabel)
	}

	if len(days) > 0 {
		for _, day := range days {
			dayText := f.formatDayHeader(day, opts)
			lessonsText := f.formatTeacherLessons(day.Lessons, opts)
			if isGroup {
				lessonsText = f.formatGroupLessons(day.Lessons, opts)
			}
			text = append(text, dayText+"\n"+lessonsText)
		}
	} else {
		if empty := f.NoTimetable(opts); empty != "" {
			text = append(text, empty)
		}
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

func (f *CompactFormatter) formatGroupLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return opts.italicLabel("no_lessons")
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

		withSubgroups := isSubgroupList(lesson)
		showOpts := groupLessonOptions(subs, withSubgroups)

		mainLesson := f.formatGroupLessonLine(subs[0], showOpts)
		text = append(text, lessonHeader+mainLesson)

		if withSubgroups {
			reverseOpts := reverseGroupLessonOptions(showOpts)
			for _, sub := range subs {
				text = append(text, "- "+f.formatGroupLessonLine(sub, reverseOpts))
			}
		}
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *CompactFormatter) formatTeacherLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return opts.italicLabel("no_lessons")
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

		text = append(text, fmt.Sprintf("%d. ", i+1)+f.formatTeacherLesson(subs[0]))
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *CompactFormatter) formatTeacherLesson(p LessonPart) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	parts = append(parts, p.Group+"-"+p.Lesson)

	if p.Cabinet != "" {
		parts = append(parts, "{"+p.Cabinet+"}")
	}

	return strings.Join(parts, " ")
}

func (f *CompactFormatter) formatGroupLessonLine(p LessonPart, show map[string]bool) string {
	var parts []string

	if show["subgroup"] && p.Subgroup > 0 {
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
