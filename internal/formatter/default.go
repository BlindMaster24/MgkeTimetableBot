package formatter

import (
	"fmt"
	"strings"
	"time"
)

type DefaultFormatter struct{}

func (f *DefaultFormatter) Name() string { return "default" }
func (f *DefaultFormatter) Label(opts FormatOptions) string {
	return opts.formatLabel("formatter_label_default")
}
func (f *DefaultFormatter) NoTimetable(opts FormatOptions) string {
	return opts.formatLabel("no_timetable")
}

func (f *DefaultFormatter) FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string {
	return f.formatFull(group, "", parseDaysFromSlice(days, opts), true, opts)
}

func (f *DefaultFormatter) FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string {
	return f.formatFull("", teacher, parseDaysFromSlice(days, opts), false, opts)
}

func (f *DefaultFormatter) formatFull(name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	return formatFull(f, name, teacher, days, isGroup, opts)
}

func (f *DefaultFormatter) groupHeader(group string, opts FormatOptions) string {
	return opts.formatLabel("formatter_group_default", group)
}

func (f *DefaultFormatter) teacherHeader(teacher string, opts FormatOptions) string {
	return opts.formatLabel("formatter_teacher_default", teacher)
}

func (f *DefaultFormatter) formatDayHeader(day DayInfo, opts FormatOptions) string {
	w := day.Weekday
	if day.Hint != "" {
		w += " " + opts.i(day.Hint)
	}
	return opts.b(w) + ", " + day.Date
}

func (f *DefaultFormatter) formatGroupLessons(lessons []any, opts FormatOptions) string {
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

		mainLesson := f.formatLessonLine(subs[0], showOpts, opts)
		text = append(text, lessonHeader+mainLesson)

		if withSubgroups {
			reverseOpts := reverseGroupLessonOptions(showOpts)
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

	return strings.TrimSpace(strings.Join(text, "\n"))
}

func (f *DefaultFormatter) formatTeacherLessons(lessons []any, opts FormatOptions) string {
	return formatTeacherLessonList(lessons, opts.italicLabel("no_lessons"), f.formatTeacherLesson)
}

func (f *DefaultFormatter) formatTeacherLesson(p LessonPart) string {
	var parts []string

	if p.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", p.Subgroup))
	}

	parts = append(parts, p.Group+"-"+p.Lesson)

	if p.Type != "" {
		parts = append(parts, "("+p.Type+")")
	}

	if p.Cabinet != "" {
		parts = append(parts, "{"+p.Cabinet+"}")
	}

	if p.Comment != "" {
		parts = append(parts, "// "+p.Comment)
	}

	return strings.Join(parts, " ")
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

func parseDaysFromSlice(days []map[string]any, opts FormatOptions) []DayInfo {
	if len(days) == 0 {
		return nil
	}

	now := opts.now()
	today := now.Format("02.01.2006")
	tomorrow := now.AddDate(0, 0, 1).Format("02.01.2006")

	var result []DayInfo
	for _, dayMap := range days {
		dateStr, _ := dayMap["day"].(string)
		lessons, _ := dayMap["lessons"].([]any)

		wd := ""
		if t, err := time.Parse("02.01.2006", dateStr); err == nil {
			wd = opts.weekdayName(t.Weekday())
		}

		hint := ""
		if dateStr == today {
			hint = opts.formatLabel("day_hint_today")
		} else if dateStr == tomorrow {
			hint = opts.formatLabel("day_hint_tomorrow")
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
