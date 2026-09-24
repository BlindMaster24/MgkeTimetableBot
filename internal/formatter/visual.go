package formatter

import (
	"fmt"
	"strings"
)

type VisualFormatter struct{}

func (f *VisualFormatter) Name() string  { return "visual" }
func (f *VisualFormatter) Label() string { return "🌈 Визуальный" }
func (f *VisualFormatter) NoTimetable() string {
	return "🚫 Нет расписания для отображения"
}

func (f *VisualFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts.now())
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *VisualFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days, opts.now())
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *VisualFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			text = append(text, "👩‍🎓 Группа '"+name+"'")
		} else {
			text = append(text, "👩‍🏫 Преподаватель '"+opts.getFullTeacherName(teacher)+"'")
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
		text = append(text, f.NoTimetable())
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
	return "📅 " + opts.b(w) + ", " + day.Date
}

var smileNumbers = []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

func (f *VisualFormatter) formatGroupLessons(lessons []any, opts FormatOptions) string {
	if len(lessons) == 0 {
		return "🚫 Нет пар на этот день"
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

		header := f.lessonHeader(i)

		withSubgroups := isSubgroupList(lesson)
		showOpts := groupLessonOptions(subs, withSubgroups)

		main := f.formatGroupLesson(subs[0], showOpts)
		text = append(text, f.lessonHeaderBlock(header, main, withSubgroups))

		if withSubgroups {
			reverseOpts := reverseGroupLessonOptions(showOpts)
			lines := make([]string, 0, len(subs))
			for j, sub := range subs {
				value := f.formatGroupLesson(sub, reverseOpts)
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
		return "🚫 Нет пар на этот день"
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

		text = append(text, f.lessonHeaderBlock(f.lessonHeader(i), f.formatTeacherLesson(subs[0]), false))
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

func (f *VisualFormatter) lessonHeader(i int) string {
	num := ""
	if i+1 < len(smileNumbers) {
		num = smileNumbers[i+1]
	}
	return "\n" + num + " Пара:"
}

func (f *VisualFormatter) formatGroupLesson(p LessonPart, show map[string]bool) string {
	var lines []string

	if show["subgroup"] && p.Subgroup > 0 {
		lines = append(lines, fmt.Sprintf("    🎒 Подгруппа %d:", p.Subgroup))
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

func (f *VisualFormatter) formatTeacherLesson(p LessonPart) string {
	var lines []string

	if p.Subgroup > 0 {
		lines = append(lines, fmt.Sprintf("    🎒 Подгруппа %d:", p.Subgroup))
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
