package formatter

import (
	"fmt"
	"strings"
)

// scheduleBody is the per-style part of a timetable rendering: the labels a
// formatter chooses and the way one day or one lesson looks. The header, the
// week label, the days, the empty case and the footer are laid out once by
// formatFull.
type scheduleBody interface {
	NoTimetable(opts FormatOptions) string
	groupHeader(group string, opts FormatOptions) string
	teacherHeader(teacher string, opts FormatOptions) string
	formatDayHeader(day DayInfo, opts FormatOptions) string
	formatGroupLessons(lessons []any, opts FormatOptions) string
	formatTeacherLessons(lessons []any, opts FormatOptions) string
}

func formatFull(s scheduleBody, name string, teacher string, days []DayInfo, isGroup bool, opts FormatOptions) string {
	var text []string

	if opts.ShowHeader {
		header := s.teacherHeader(opts.getFullTeacherName(teacher), opts)
		if isGroup {
			header = s.groupHeader(name, opts)
		}
		if header != "" {
			text = append(text, header)
		}
	}

	if opts.WeekLabel != "" {
		text = append(text, opts.WeekLabel)
	}

	if len(days) > 0 {
		for _, day := range days {
			lessons := s.formatTeacherLessons(day.Lessons, opts)
			if isGroup {
				lessons = s.formatGroupLessons(day.Lessons, opts)
			}
			text = append(text, s.formatDayHeader(day, opts)+"\n"+lessons)
		}
	} else if empty := s.NoTimetable(opts); empty != "" {
		text = append(text, empty)
	}

	if footer := formatFooter(opts); strings.TrimSpace(footer) != "" {
		text = append(text, footer)
	}

	return strings.Join(text, "\n\n")
}

// formatTeacherLessonList numbers the lessons of a teacher view; line renders
// one part (each formatter words it differently).
func formatTeacherLessonList(lessons []any, empty string, line func(LessonPart) string) string {
	if len(lessons) == 0 {
		return empty
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

		text = append(text, fmt.Sprintf("%d. ", i+1)+line(subs[0]))
	}

	return strings.TrimSpace(strings.Join(text, "\n"))
}
