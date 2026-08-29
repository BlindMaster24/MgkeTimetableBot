package formatter

import (
	"fmt"
	"strings"
	"time"
)

var weekdayNames = []string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}

type Formatter interface {
	Name() string
	Label() string
	FormatGroupFull(group string, days []map[string]any, opts FormatOptions) string
	FormatTeacherFull(teacher string, days []map[string]any, opts FormatOptions) string
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

func formatFooter(opts FormatOptions) string {
	var text []string

	if opts.ShowParserTime && opts.ParserUpdateTime > 0 {
		secs := (time.Now().UnixMilli() - opts.ParserUpdateTime) / 1000
		text = append(text, fmt.Sprintf("Информация была загружена %s назад", formatSeconds(secs)))
	}

	if opts.HasParserError {
		text = append(text, "⚠️ В последний раз при получении расписания с сайта произошла ошибка. Есть вероятность, что расписание не актуальное.")
	} else if opts.ShowHints && opts.RandHint != "" {
		text = append(text, "💬 Подсказка: "+opts.RandHint)
	}

	return strings.Join(text, "\n\n")
}

func formatSeconds(secs int64) string {
	if secs < 60 {
		return fmt.Sprintf("%d сек", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%d мин %d сек", secs/60, secs%60)
	}
	return fmt.Sprintf("%d ч %d мин", secs/3600, (secs%3600)/60)
}

func notLessons() string {
	return "🚫 Нет пар на этот день"
}

func notTimetable() string {
	return "Нет расписания для отображения"
}
