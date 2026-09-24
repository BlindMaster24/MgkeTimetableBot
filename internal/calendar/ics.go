package calendar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type ICSBuilder struct {
	events     []icsEvent
	weekdays   [][2][2]string
	saturday   [][2][2]string
	weekNumber int
	now        time.Time
}

type icsEvent struct {
	uid         string
	dtstamp     string
	start       time.Time
	end         time.Time
	summary     string
	description string
	location    string
}

func NewICSBuilder(weekdays, saturday [][2][2]string, weekNumber int) *ICSBuilder {
	return &ICSBuilder{
		weekdays:   weekdays,
		saturday:   saturday,
		weekNumber: weekNumber,
		now:        time.Now(),
	}
}

func (b *ICSBuilder) AddGroupDay(day model.GroupDay, group string) {
	calls, ok := b.callsFor(day.Day)
	if !ok {
		return
	}

	for index, entry := range day.Lessons {
		if index >= len(calls) {
			continue
		}
		start, end, ok := lessonRange(day.Day, calls[index])
		if !ok {
			continue
		}
		for _, lesson := range groupEntryLessons(entry) {
			if lesson == nil || lesson.Lesson == "" {
				continue
			}
			b.events = append(b.events, b.groupEvent(group, day.Day, index, start, end, lesson))
		}
	}
}

func groupEntryLessons(entry model.GroupLesson) []*model.GroupLessonExplain {
	if entry == nil {
		return nil
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return nil
	}

	var single model.GroupLessonExplain
	if err := json.Unmarshal(raw, &single); err == nil {
		return []*model.GroupLessonExplain{&single}
	}

	var many []*model.GroupLessonExplain
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil
	}
	return many
}

func (b *ICSBuilder) AddTeacherDay(day model.TeacherDay, teacher string) {
	calls, ok := b.callsFor(day.Day)
	if !ok {
		return
	}

	for index, lesson := range day.Lessons {
		if index >= len(calls) {
			continue
		}
		if lesson == nil || lesson.Lesson == "" {
			continue
		}
		start, end, ok := lessonRange(day.Day, calls[index])
		if !ok {
			continue
		}
		b.events = append(b.events, b.teacherEvent(teacher, day.Day, index, start, end, lesson))
	}
}

func (b *ICSBuilder) Build() string {
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//MGKE Timetable Bot//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
	}

	for _, event := range b.events {
		lines = append(lines,
			"BEGIN:VEVENT",
			"UID:"+event.uid,
			"DTSTAMP:"+event.dtstamp,
			"DTSTART:"+formatICSDateTime(event.start),
			"DTEND:"+formatICSDateTime(event.end),
			"SUMMARY:"+escapeICSText(event.summary),
		)
		if event.description != "" {
			lines = append(lines, "DESCRIPTION:"+escapeICSText(event.description))
		}
		if event.location != "" {
			lines = append(lines, "LOCATION:"+escapeICSText(event.location))
		}
		lines = append(lines, "END:VEVENT")
	}

	lines = append(lines, "END:VCALENDAR")
	return strings.Join(lines, "\r\n")
}

func (b *ICSBuilder) EventCount() int {
	return len(b.events)
}

func (b *ICSBuilder) callsFor(day string) ([][2][2]string, bool) {
	date, err := time.Parse("02.01.2006", day)
	if err != nil {
		return nil, false
	}
	if date.Weekday() == time.Saturday {
		return b.saturday, true
	}
	return b.weekdays, true
}

func (b *ICSBuilder) groupEvent(value, day string, index int, start, end time.Time, lesson *model.GroupLessonExplain) icsEvent {
	var parts []string
	if lesson.Teacher != nil && *lesson.Teacher != "" {
		parts = append(parts, "Преподаватель: "+*lesson.Teacher)
	}
	subgroup := 0
	if lesson.Subgroup != nil {
		subgroup = *lesson.Subgroup
	}
	return b.newEvent("group", value, day, index, start, end, lesson.Lesson, lesson.Type, lesson.Cabinet, subgroup, parts)
}

func (b *ICSBuilder) teacherEvent(value, day string, index int, start, end time.Time, lesson *model.TeacherLessonExplain) icsEvent {
	var parts []string
	if lesson.Group != "" {
		parts = append(parts, "Группа: "+lesson.Group)
	}
	subgroup := 0
	if lesson.Subgroup != nil {
		subgroup = *lesson.Subgroup
	}
	return b.newEvent("teacher", value, day, index, start, end, lesson.Lesson, lesson.Type, lesson.Cabinet, subgroup, parts)
}

func (b *ICSBuilder) newEvent(kind, value, day string, index int, start, end time.Time, lesson string, lessonType, cabinet *string, subgroup int, parts []string) icsEvent {
	summary := lesson + lessonTypeSuffix(lessonType) + subgroupSuffix(subgroup)

	cabinetValue := deref(cabinet)
	if cabinetValue != "" {
		parts = append(parts, "Кабинет: "+cabinetValue)
	}

	return icsEvent{
		uid:         eventUID(kind, value, b.weekNumber, day, index, lesson, cabinetValue, deref(lessonType), subgroup),
		dtstamp:     formatICSDateTime(b.now),
		start:       start,
		end:         end,
		summary:     summary,
		description: strings.Join(parts, "\n"),
		location:    cabinetValue,
	}
}

func lessonRange(day string, call [2][2]string) (time.Time, time.Time, bool) {
	start, err := time.Parse("02.01.2006 15:04", day+" "+call[0][0])
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := time.Parse("02.01.2006 15:04", day+" "+call[1][1])
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func eventUID(kind, value string, weekNumber int, day string, index int, lesson, cabinet, lessonType string, subgroup int) string {
	source := fmt.Sprintf("%s:%s:%d:%s:%d:%s:%s:%s:%d", kind, value, weekNumber, day, index, lesson, cabinet, lessonType, subgroup)
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func lessonTypeSuffix(lessonType *string) string {
	if lessonType == nil || *lessonType == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", *lessonType)
}

func subgroupSuffix(subgroup int) string {
	if subgroup == 0 {
		return ""
	}
	return fmt.Sprintf(", подгр. %d", subgroup)
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatICSDateTime(t time.Time) string {
	return t.Format("20060102T150405")
}

func escapeICSText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, ",", "\\,")
	value = strings.ReplaceAll(value, ";", "\\;")
	return value
}
