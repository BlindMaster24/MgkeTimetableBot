package formatter

import (
	"fmt"
	"strings"
)

type VisualFormatter struct{}

func (f *VisualFormatter) Name() string  { return "visual" }
func (f *VisualFormatter) Label() string { return "🌈 Визуальный" }

func (f *VisualFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull(group, "", daysInfo, true, opts)
}

func (f *VisualFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	daysInfo := parseDaysFromSlice(days)
	return f.formatFull("", teacher, daysInfo, false, opts)
}

func (f *VisualFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		if isGroup {
			text = append(text, "👩‍🎓 Группа '"+name+"'")
		} else {
			text = append(text, "👩‍🏫 Преподаватель '"+teacher+"'")
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

func (f *VisualFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return "📅 " + opts.b(w) + ", " + day.Date
}

var smileNumbers = []string{"0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

func (f *VisualFormatter) formatLessons(lessons []any, opts FormatOptions) string {
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

		numStr := ""
		if i+1 < len(smileNumbers) {
			numStr = smileNumbers[i+1]
		}
		text = append(text, "\n"+numStr+" Пара:")

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
		text = append(text, mainLesson)

		if withSubgroups {
			reverseOpts := map[string]bool{}
			for k, v := range showOpts {
				reverseOpts[k] = !v
			}
			for j, sub := range subs {
				value := f.formatLessonLine(sub, reverseOpts, opts)
				if j > 0 {
					value = "\n" + value
				}
				text = append(text, value)
			}
			text = append(text, "")
		}
	}

	return strings.Join(text, "\n")
}

func (f *VisualFormatter) formatLessonLine(p LessonPart, show map[string]bool, opts FormatOptions) string {
	var parts []string

	if show["subgroup"] && p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("    🎒 Подгруппа %d:", p.Subgroup))
	}

	lessonPart := ""
	if show["lesson"] {
		lessonPart = p.Lesson
	}
	if show["type"] && p.Type != "" {
		lessonPart += " (" + p.Type + ")"
	}
	if lessonPart != "" {
		parts = append(parts, "    📚 "+lessonPart)
	}

	if show["teacher"] && p.Teacher != "" {
		parts = append(parts, "    🎓 "+p.Teacher)
	}

	if show["cabinet"] && p.Cabinet != "" {
		parts = append(parts, "    🏫 "+p.Cabinet)
	}

	if show["comment"] && p.Comment != "" {
		parts = append(parts, "    // "+p.Comment)
	}

	return strings.Join(parts, "\n")
}
