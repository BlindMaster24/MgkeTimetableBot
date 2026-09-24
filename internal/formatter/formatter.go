package formatter

import (
	"fmt"
	"math"
	"strings"
	"time"
)

var weekdayNames = []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

type Formatter interface {
	Name() string
	Label() string
	FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string
	FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string
	NoTimetable() string
}

type FormatOptions struct {
	ShowHeader       bool
	ShowParserTime   bool
	ParserUpdateTime int64
	HasParserError   bool
	ShowHints        bool
	WeekLabel        string
	IsTelegram       bool
	RandHint         string
	TeacherNames     map[string]string
}

func (o FormatOptions) b(text string) string {
	if o.IsTelegram {
		return "<b>" + text + "</b>"
	}
	return text
}

func (o FormatOptions) i(text string) string {
	if o.IsTelegram {
		return "<i>" + text + "</i>"
	}
	return text
}

func (o FormatOptions) getFullTeacherName(shortName string) string {
	if o.TeacherNames != nil {
		if full, ok := o.TeacherNames[shortName]; ok {
			return full
		}
	}
	return shortName
}

var AllFormatters = []Formatter{
	&DefaultFormatter{},
	&VisualFormatter{},
	&CompactFormatter{},
	&LitolaxFormatter{},
}

func GetByIndex(i int) Formatter {
	if i < 0 || i >= len(AllFormatters) {
		return AllFormatters[0]
	}
	return AllFormatters[i]
}

func IndexOf(name string) int {
	for i, f := range AllFormatters {
		if f.Name() == name {
			return i
		}
	}
	return 0
}

type DayInfo struct {
	Date    string
	Weekday string
	Hint    string
	Lessons []any
}

type LessonPart struct {
	Subgroup int
	Lesson   string
	Type     string
	Teacher  string
	Cabinet  string
	Comment  string
	Group    string
}

func parseLessonPart(m map[string]any) LessonPart {
	p := LessonPart{}
	if sub, ok := m["subgroup"].(float64); ok {
		p.Subgroup = int(sub)
	}
	p.Lesson, _ = m["lesson"].(string)
	p.Type, _ = m["type"].(string)
	p.Teacher, _ = m["teacher"].(string)
	p.Cabinet, _ = m["cabinet"].(string)
	p.Comment, _ = m["comment"].(string)
	p.Group, _ = m["group"].(string)
	return p
}

func getSubgroups(lesson any) []LessonPart {
	switch v := lesson.(type) {
	case map[string]any:
		return []LessonPart{parseLessonPart(v)}
	case []any:
		var parts []LessonPart
		for _, sub := range v {
			if subMap, ok := sub.(map[string]any); ok {
				parts = append(parts, parseLessonPart(subMap))
			}
		}
		return parts
	}
	return nil
}

func allEqual(fn func(LessonPart) string, parts []LessonPart) bool {
	if len(parts) <= 1 {
		return true
	}
	first := fn(parts[0])
	for _, p := range parts[1:] {
		if fn(p) != first {
			return false
		}
	}
	return true
}

func isSubgroupList(lesson any) bool {
	_, ok := lesson.([]any)
	return ok
}

func groupLessonOptions(subs []LessonPart, withSubgroups bool) map[string]bool {
	lessonsEqual := allEqual(func(p LessonPart) string { return p.Lesson }, subs)
	typeEqual := lessonsEqual && allEqual(func(p LessonPart) string { return p.Type }, subs)
	teacherEqual := len(subs) > 1 && typeEqual && allEqual(func(p LessonPart) string { return p.Teacher }, subs)
	cabinetEqual := teacherEqual && allEqual(func(p LessonPart) string { return p.Cabinet }, subs)
	commentEqual := allEqual(func(p LessonPart) string { return p.Comment }, subs)

	return map[string]bool{
		"subgroup": !withSubgroups,
		"lesson":   !withSubgroups || lessonsEqual,
		"type":     !withSubgroups || typeEqual,
		"teacher":  !withSubgroups || teacherEqual,
		"cabinet":  !withSubgroups || cabinetEqual,
		"comment":  !withSubgroups || commentEqual,
	}
}

func reverseGroupLessonOptions(show map[string]bool) map[string]bool {
	reversed := make(map[string]bool, len(show))
	for key, value := range show {
		reversed[key] = !value
	}
	return reversed
}

func formatFooter(opts FormatOptions) string {
	var text []string

	if opts.ShowParserTime && opts.ParserUpdateTime > 0 {
		secs := int64(math.Ceil(float64(time.Now().UnixMilli()-opts.ParserUpdateTime) / 1000))
		text = append(text, fmt.Sprintf("Информация была загружена %s назад", FormatSeconds(secs)))
	}

	if opts.HasParserError {
		text = append(text, "⚠️ В последний раз при получении расписания с сайта произошла ошибка. Есть вероятность, что расписание не актуальное. Если проблема не исчезнет - сообщите разработчику.")
	} else if opts.ShowHints && opts.RandHint != "" {
		text = append(text, "💬 Подсказка: "+opts.RandHint)
	}

	return strings.Join(text, "\n\n")
}

var secondsPeriods = []int64{60, 3600, 86400, 31536000}
var secondsSuffixes = []string{"сек.", "мин.", "ч.", "д.", "г."}

func FormatSeconds(secs int64) string {
	if secs < 0 {
		secs = 0
	}

	values := 3
	times := make([]int64, len(secondsSuffixes))
	filled := make([]bool, len(secondsSuffixes))
	rest := secs
	countZero := false

	for i := values; i >= 0; i-- {
		period := rest / secondsPeriods[i]
		if period > 0 || countZero {
			times[i+1] = period
			filled[i+1] = true
			rest -= period * secondsPeriods[i]
			countZero = true
		}
	}
	times[0] = rest
	filled[0] = true

	var parts []string
	for i := len(times) - 1; i >= 0; i-- {
		if !filled[i] {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d %s", times[i], secondsSuffixes[i]))
	}
	return strings.Join(parts, " ")
}
