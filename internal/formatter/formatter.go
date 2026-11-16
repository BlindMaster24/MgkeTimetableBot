package formatter

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Formatter interface {
	Name() string
	Label(opts FormatOptions) string
	FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string
	FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string
	NoTimetable(opts FormatOptions) string
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
	Now              time.Time
	Labels           map[string]string
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

func (o FormatOptions) now() time.Time {
	if !o.Now.IsZero() {
		return o.Now
	}
	return time.Now()
}

var labelKeys = []string{
	"formatter_label_default",
	"formatter_label_visual",
	"formatter_label_compact",
	"formatter_label_litolax",
	"no_timetable",
	"no_lessons",
	"formatter_no_timetable_visual",
	"formatter_no_lessons_visual",
	"formatter_group_default",
	"formatter_teacher_default",
	"formatter_group_compact",
	"formatter_teacher_compact",
	"formatter_group_visual",
	"formatter_teacher_visual",
	"formatter_group_litolax",
	"formatter_teacher_litolax",
	"formatter_day_header_visual",
	"formatter_day_header_litolax",
	"formatter_pair_visual",
	"formatter_pair_litolax",
	"formatter_subgroup_visual",
	"formatter_cabinet_litolax",
	"formatter_loaded_ago",
	"formatter_hint_prefix",
	"formatter_seconds_second",
	"formatter_seconds_minute",
	"formatter_seconds_hour",
	"formatter_seconds_day",
	"formatter_seconds_year",
	"day_hint_today",
	"day_hint_tomorrow",
	"parser_error_warning",
	"weekday_0",
	"weekday_1",
	"weekday_2",
	"weekday_3",
	"weekday_4",
	"weekday_5",
	"weekday_6",
}

func LabelKeys() []string {
	return append([]string(nil), labelKeys...)
}

func (o FormatOptions) formatLabel(key string, args ...any) string {
	label := o.Labels[key]
	if label == "" {
		return ""
	}
	if len(args) == 0 {
		return label
	}
	return fmt.Sprintf(label, args...)
}

func (o FormatOptions) italicLabel(key string) string {
	if label := o.Labels[key]; label != "" {
		return o.i(label)
	}
	return ""
}

func (o FormatOptions) weekdayName(day time.Weekday) string {
	return o.Labels[fmt.Sprintf("weekday_%d", int(day))]
}

func (o FormatOptions) secondsSuffixes() []string {
	keys := []string{"formatter_seconds_second", "formatter_seconds_minute", "formatter_seconds_hour", "formatter_seconds_day", "formatter_seconds_year"}
	suffixes := make([]string, len(keys))
	for i, key := range keys {
		suffixes[i] = o.Labels[key]
	}
	return suffixes
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
		secs := int64(math.Ceil(float64(opts.now().UnixMilli()-opts.ParserUpdateTime) / 1000))
		if secs < 0 {
			secs = 0
		}
		if loaded := opts.formatLabel("formatter_loaded_ago", formatSeconds(secs, opts.secondsSuffixes())); loaded != "" {
			text = append(text, loaded)
		}
	}

	if opts.HasParserError {
		if warning := opts.formatLabel("parser_error_warning"); warning != "" {
			text = append(text, warning)
		}
	} else if opts.ShowHints && opts.RandHint != "" {
		text = append(text, opts.formatLabel("formatter_hint_prefix")+opts.RandHint)
	}

	return strings.Join(text, "\n\n")
}

var secondsPeriods = []int64{60, 3600, 86400, 31536000}

func FormatSeconds(secs int64) string {
	return formatSeconds(secs, nil)
}

func formatSeconds(secs int64, suffixes []string) string {
	if secs < 0 {
		secs = 0
	}

	parts := len(secondsPeriods) + 1
	values := 3
	times := make([]int64, parts)
	filled := make([]bool, parts)
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

	var text []string
	for i := parts - 1; i >= 0; i-- {
		if !filled[i] {
			continue
		}
		suffix := ""
		if i < len(suffixes) {
			suffix = suffixes[i]
		}
		text = append(text, strings.TrimSpace(fmt.Sprintf("%d %s", times[i], suffix)))
	}
	return strings.Join(text, " ")
}
