package calendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type ICSBuilder struct {
	events []icsEvent
}

type icsEvent struct {
	UID         string
	DTStart     time.Time
	DTEnd       time.Time
	Summary     string
	Description string
}

func NewICSBuilder() *ICSBuilder {
	return &ICSBuilder{}
}

func (b *ICSBuilder) AddGroupDay(day model.GroupDay, group string) {
	t, err := time.Parse("02.01.2006", day.Day)
	if err != nil {
		return
	}

	for i, l := range day.Lessons {
		text := formatGroupLessonForICS(l)
		if text == "" || text == "-" {
			continue
		}
		start := t.Add(time.Duration(8+i) * time.Hour)
		end := start.Add(time.Hour)

		b.events = append(b.events, icsEvent{
			UID:         fmt.Sprintf("%s-%s-%d@bot", group, day.Day, i+1),
			DTStart:     start,
			DTEnd:       end,
			Summary:     fmt.Sprintf("%d. %s", i+1, text),
			Description: fmt.Sprintf("Группа: %s", group),
		})
	}
}

func (b *ICSBuilder) AddTeacherDay(day model.TeacherDay, teacher string) {
	t, err := time.Parse("02.01.2006", day.Day)
	if err != nil {
		return
	}

	for i, l := range day.Lessons {
		text := formatTeacherLessonForICS(l)
		if text == "" || text == "-" {
			continue
		}
		start := t.Add(time.Duration(8+i) * time.Hour)
		end := start.Add(time.Hour)

		b.events = append(b.events, icsEvent{
			UID:         fmt.Sprintf("%s-%s-%d@bot", teacher, day.Day, i+1),
			DTStart:     start,
			DTEnd:       end,
			Summary:     fmt.Sprintf("%d. %s", i+1, text),
			Description: fmt.Sprintf("Преподаватель: %s", teacher),
		})
	}
}

func (b *ICSBuilder) Build() string {
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//MgkeBot//Timetable//RU\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")

	for _, e := range b.events {
		sb.WriteString("BEGIN:VEVENT\r\n")
		sb.WriteString(fmt.Sprintf("UID:%s\r\n", e.UID))
		sb.WriteString(fmt.Sprintf("DTSTART:%s\r\n", e.DTStart.Format("20060102T150405")))
		sb.WriteString(fmt.Sprintf("DTEND:%s\r\n", e.DTEnd.Format("20060102T150405")))
		sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", e.Summary))
		sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", e.Description))
		sb.WriteString("END:VEVENT\r\n")
	}

	sb.WriteString("END:VCALENDAR\r\n")
	return sb.String()
}

func (b *ICSBuilder) EventCount() int {
	return len(b.events)
}

func formatGroupLessonForICS(l model.GroupLesson) string {
	if l == nil {
		return "-"
	}
	if s := model.AsSingle(l); s != nil {
		return formatSingleForICS(s.Lesson, s.Type, s.Teacher, s.Cabinet, s.Comment)
	}
	if arr := model.AsArray(l); arr != nil {
		parts := make([]string, 0, len(arr))
		for _, e := range arr {
			parts = append(parts, formatSingleForICS(e.Lesson, e.Type, e.Teacher, e.Cabinet, e.Comment))
		}
		return strings.Join(parts, " | ")
	}
	return "-"
}

func formatTeacherLessonForICS(l model.TeacherLesson) string {
	if l == nil {
		return "-"
	}
	return formatSingleForICS(l.Lesson, l.Type, &l.Group, l.Cabinet, l.Comment)
}

func formatSingleForICS(lesson string, typ, extra1, extra2, extra3 *string) string {
	var parts []string
	parts = append(parts, lesson)
	if typ != nil && *typ != "" {
		parts = append(parts, fmt.Sprintf("(%s)", *typ))
	}
	if extra1 != nil && *extra1 != "" {
		parts = append(parts, *extra1)
	}
	if extra2 != nil && *extra2 != "" {
		parts = append(parts, *extra2)
	}
	if extra3 != nil && *extra3 != "" {
		parts = append(parts, fmt.Sprintf("[%s]", *extra3))
	}
	return strings.Join(parts, " ")
}
