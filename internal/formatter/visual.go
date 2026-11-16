package formatter

import (
	"strings"
)

type VisualFormatter struct{}

func (f *VisualFormatter) Name() string { return "visual" }
func (f *VisualFormatter) Label(opts FormatOptions) string {
	return opts.formatLabel("formatter_label_visual")
}
func (f *VisualFormatter) NoTimetable(opts FormatOptions) string {
	return opts.formatLabel("formatter_no_timetable_visual")
}

func (f *VisualFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *VisualFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *VisualFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			if header := opts.formatLabel("formatter_group_visual", name); header != "" {
				text = append(text, header)
			}
		} else {
			if header := opts.formatLabel("formatter_teacher_visual", opts.getFullTeacherName(teacher)); header != "" {
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

func (f *VisualFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return opts.formatLabel("formatter_day_header_visual", opts.b(w), day.Date)
}

var smileNumbers = []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

func (f *VisualFormatter) formatGroupLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return opts.formatLabel("formatter_no_lessons_visual")
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

		header := f.lessonHeader(i, opts)

		withSubgroups := isSubgroupList(lesson)
		showOpts := groupLessonOptions(subs, withSubgroups)

		main := f.formatGroupLesson(subs[0], showOpts, opts)
		text = append(text, f.lessonHeaderBlock(header, main, withSubgroups))

		if withSubgroups {
			reverseOpts := reverseGroupLessonOptions(showOpts)
			lines := make([]string, 0, len(subs))
			for j, sub := range subs {
				value := f.formatGroupLesson(sub, reverseOpts, opts)
				if j > 0 {
					value = "\n" + value
				}
				lines = append(lines, value)
			}
			text = append(text, strings.Join(lines, "\n"))
		}
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *VisualFormatter) formatTeacherLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return opts.formatLabel("formatter_no_lessons_visual")
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

		text = append(text, f.lessonHeaderBlock(f.lessonHeader(i, opts), f.formatTeacherLesson(subs[0], opts), false))
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *VisualFormatter) lessonHeaderBlock(header, main string, withSubgroups bool) string {
	if main == "" {
		if withSubgroups {
			return header + "\n"
		}
		return header
	}
	if withSubgroups {
		return header + "\n" + main + "\n"
	}
	return header + "\n" + main
}

func (f *VisualFormatter) lessonHeader(i int, opts FormatOptions) string {
	num := ""
	if i+1 < len(smileNumbers) {
		num = smileNumbers[i+1]
	}
	return "\n" + strings.TrimSpace(num+" "+opts.formatLabel("formatter_pair_visual"))
}

func (f *VisualFormatter) formatGroupLesson(p LessonPart, show map[string]bool, opts FormatOptions) string {
	var lines []string

	if show["subgroup"] && p.Subgroup > 0 {
		lines = append(lines, "    "+opts.formatLabel("formatter_subgroup_visual", p.Subgroup))
	}

	if show["lesson"] {
		lesson := "    📚 " + p.Lesson
		if show["type"] && p.Type != "" {
			lesson += " (" + p.Type + ")"
		}
		lines = append(lines, lesson)
	}

	if show["teacher"] && p.Teacher != "" {
		lines = append(lines, "    🎓 "+p.Teacher)
	}

	if show["cabinet"] && p.Cabinet != "" {
		lines = append(lines, "    🏫 "+p.Cabinet)
	}

	if show["comment"] && p.Comment != "" {
		lines = append(lines, "// "+p.Comment)
	}

	return strings.Join(lines, "\n")
}

func (f *VisualFormatter) formatTeacherLesson(p LessonPart, opts FormatOptions) string {
	var lines []string

	if p.Subgroup > 0 {
		lines = append(lines, "    "+opts.formatLabel("formatter_subgroup_visual", p.Subgroup))
	}

	lesson := "    📚 " + p.Group + "-" + p.Lesson
	if p.Type != "" {
		lesson += " (" + p.Type + ")"
	}
	lines = append(lines, lesson)

	if p.Cabinet != "" {
		lines = append(lines, "    🏫 "+p.Cabinet)
	}

	if p.Comment != "" {
		lines = append(lines, "// "+p.Comment)
	}

	return strings.Join(lines, "\n")
}
