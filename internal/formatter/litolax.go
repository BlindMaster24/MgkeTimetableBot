package formatter

import (
	"fmt"
	"strings"
)

type LitolaxFormatter struct{}

func (f *LitolaxFormatter) Name() string { return "litolax" }
func (f *LitolaxFormatter) Label(opts FormatOptions) string {
	return opts.formatLabel("formatter_label_litolax")
}
func (f *LitolaxFormatter) NoTimetable(opts FormatOptions) string {
	return opts.formatLabel("no_timetable")
}

func (f *LitolaxFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *LitolaxFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *LitolaxFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			if header := opts.formatLabel("formatter_group_litolax", name); header != "" {
				text = append(text, opts.b(header))
			}
		} else {
			if header := opts.formatLabel("formatter_teacher_litolax", opts.getFullTeacherName(teacher)); header != "" {
				text = append(text, opts.b(header))
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

func (f *LitolaxFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return "\n" + opts.formatLabel("formatter_day_header_litolax", w, day.Date) + "\n"
}

func (f *LitolaxFormatter) formatGroupLessons(lessons []any, opts FormatOptions) string {
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

		header := "\n" + opts.b(opts.formatLabel("formatter_pair_litolax", i+1))

		if !isSubgroupList(lesson) {
			text = append(text, header+"\n"+f.formatGroupLessonLine(subs[0]))
		} else {
			text = append(text, header)
			for _, sub := range subs {
				text = append(text, f.formatGroupLessonLine(sub))
			}
		}

		if cabinet := opts.formatLabel("formatter_cabinet_litolax", strings.Join(cabinetsOf(subs), " ")); cabinet != "" {
			text = append(text, cabinet)
		}
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *LitolaxFormatter) formatTeacherLessons(lessons []any, opts FormatOptions) string {
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

		header := "\n" + opts.b(opts.formatLabel("formatter_pair_litolax", i+1))
		text = append(text, header+"\n"+f.formatTeacherLesson(subs[0]))
		if cabinet := opts.formatLabel("formatter_cabinet_litolax", oldDash(subs[0].Cabinet)); cabinet != "" {
			text = append(text, cabinet)
		}
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func cabinetsOf(subs []LessonPart) []string {
	cabinets := make([]string, 0, len(subs))
	for _, sub := range subs {
		cabinets = append(cabinets, oldDash(sub.Cabinet))
	}
	return cabinets
}

func oldDash(cabinet string) string {
	if cabinet == "" {
		return "-"
	}
	return cabinet
}

func (f *LitolaxFormatter) formatGroupLessonLine(p LessonPart) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	parts = append(parts, p.Lesson)

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

func (f *LitolaxFormatter) formatTeacherLesson(p LessonPart) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	parts = append(parts, p.Group+"-"+p.Lesson)

	if p.Type != "" {
		parts = append(parts, "("+p.Type+")")
	}

	if p.Comment != "" {
		parts = append(parts, "// "+p.Comment)
	}

	return strings.Join(parts, " ")
}
