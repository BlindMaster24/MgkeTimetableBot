package formatter

import (
	"strings"
	"testing"
	"time"
)

func makeTestDaysData() []any {
	today := time.Now().Format("02.01.2006")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")
	nextWeek := time.Now().AddDate(0, 0, 7).Format("02.01.2006")

	return []any{
		map[string]any{
			"day": today,
			"lessons": []any{
				map[string]any{"lesson": "Математика", "type": "лекция", "teacher": "Иванов И.И.", "cabinet": "101"},
				map[string]any{"lesson": "Физика", "type": "практика", "teacher": "Петров П.П.", "cabinet": "202"},
			},
		},
		map[string]any{
			"day": tomorrow,
			"lessons": []any{
				map[string]any{"lesson": "Информатика", "type": "лабораторная", "teacher": "Сидоров С.С.", "cabinet": "303"},
			},
		},
		map[string]any{
			"day":     nextWeek,
			"lessons": []any{},
		},
	}
}

func makeDataWithDays(days []any) map[string]any {
	return map[string]any{"days": days}
}

func makeSingleLessonData() map[string]any {
	today := time.Now().Format("02.01.2006")
	return map[string]any{
		"days": []any{
			map[string]any{
				"day": today,
				"lessons": []any{
					map[string]any{"lesson": "Английский", "type": "seminar", "cabinet": "10"},
				},
			},
		},
	}
}

func makeSubgroupData() map[string]any {
	today := time.Now().Format("02.01.2006")
	return map[string]any{
		"days": []any{
			map[string]any{
				"day": today,
				"lessons": []any{
					[]any{
						map[string]any{"subgroup": 1.0, "lesson": "Программирование", "type": "лабораторная", "teacher": "А", "cabinet": "10"},
						map[string]any{"subgroup": 2.0, "lesson": "Программирование", "type": "лабораторная", "teacher": "Б", "cabinet": "20"},
					},
				},
			},
		},
	}
}

func TestGetByIndex(t *testing.T) {
	if GetByIndex(0) == nil {
		t.Fatal("GetByIndex(0) returned nil")
	}
	if GetByIndex(-1) == nil {
		t.Fatal("GetByIndex(-1) returned nil")
	}
	if GetByIndex(999) == nil {
		t.Fatal("GetByIndex(999) returned nil")
	}
	if GetByIndex(0).Name() != "default" {
		t.Fatalf("expected default, got %s", GetByIndex(0).Name())
	}
	if GetByIndex(1).Name() != "visual" {
		t.Fatalf("expected visual, got %s", GetByIndex(1).Name())
	}
	if GetByIndex(2).Name() != "compact" {
		t.Fatalf("expected compact, got %s", GetByIndex(2).Name())
	}
	if GetByIndex(3).Name() != "litolax" {
		t.Fatalf("expected litolax, got %s", GetByIndex(3).Name())
	}
}

func TestIndexOf(t *testing.T) {
	if IndexOf("default") != 0 {
		t.Fatal("IndexOf(default) != 0")
	}
	if IndexOf("visual") != 1 {
		t.Fatal("IndexOf(visual) != 1")
	}
	if IndexOf("compact") != 2 {
		t.Fatal("IndexOf(compact) != 2")
	}
	if IndexOf("litolax") != 3 {
		t.Fatal("IndexOf(litolax) != 3")
	}
	if IndexOf("unknown") != 0 {
		t.Fatal("IndexOf(unknown) != 0")
	}
}

func TestDefaultFormatter_GroupDay(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, ShowHeader: true}
	result := f.FormatGroupFull("63ТП", days, opts)

	if result == "" {
		t.Fatal("default formatter returned empty string")
	}
	if !strings.Contains(result, "63ТП") {
		t.Fatalf("expected group name in output, got: %s", result)
	}
	if !strings.Contains(result, "Математика") {
		t.Fatalf("expected lesson name, got: %s", result)
	}
}

func TestDefaultFormatter_TeacherDay(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true}
	result := f.FormatTeacherFull("Иванов И.И.", days, opts)

	if result == "" {
		t.Fatal("default formatter returned empty string")
	}
	if !strings.Contains(result, "Иванов") {
		t.Fatalf("expected teacher name in output, got: %s", result)
	}
}

func TestVisualFormatter_GroupDay(t *testing.T) {
	f := &VisualFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, ShowHeader: true}
	result := f.FormatGroupFull("63ТП", days, opts)

	if result == "" {
		t.Fatal("visual formatter returned empty string")
	}
	if !strings.Contains(result, "👩‍🎓") {
		t.Fatalf("expected emoji in visual output, got: %s", result)
	}
	if !strings.Contains(result, "📚") {
		t.Fatalf("expected book emoji, got: %s", result)
	}
}

func TestCompactFormatter_GroupDay(t *testing.T) {
	f := &CompactFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true}
	result := f.FormatGroupFull("63ТП", days, opts)

	if result == "" {
		t.Fatal("compact formatter returned empty string")
	}
	if !strings.Contains(result, "Математика") {
		t.Fatalf("expected lesson in compact output, got: %s", result)
	}
}

func TestLitolaxFormatter_GroupDay(t *testing.T) {
	f := &LitolaxFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true}
	result := f.FormatGroupFull("63ТП", days, opts)

	if result == "" {
		t.Fatal("litolax formatter returned empty string")
	}
	if !strings.Contains(result, "День -") {
		t.Fatalf("expected day header in litolax, got: %s", result)
	}
}

func TestDayHintsToday(t *testing.T) {
	for _, f := range AllFormatters {
		today := time.Now().Format("02.01.2006")
		data := map[string]any{
			"days": []any{
				map[string]any{"day": today, "lessons": []any{map[string]any{"lesson": "Тест", "cabinet": "1"}}},
			},
		}
		days := extractDaysFromData(data)
		opts := FormatOptions{IsTelegram: true}
		result := f.FormatGroupFull("Г1", days, opts)

		if !strings.Contains(result, "сегодня") {
			t.Fatalf("%s: expected '(сегодня)' hint, got: %s", f.Name(), result)
		}
	}
}

func TestDayHintsTomorrow(t *testing.T) {
	for _, f := range AllFormatters {
		tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")
		data := map[string]any{
			"days": []any{
				map[string]any{"day": tomorrow, "lessons": []any{map[string]any{"lesson": "Тест", "cabinet": "1"}}},
			},
		}
		days := extractDaysFromData(data)
		opts := FormatOptions{IsTelegram: true}
		result := f.FormatGroupFull("Г1", days, opts)

		if !strings.Contains(result, "завтра") {
			t.Fatalf("%s: expected '(завтра)' hint, got: %s", f.Name(), result)
		}
	}
}

func TestHTMLFormatting(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "<b>") {
		t.Fatalf("expected HTML bold tag, got: %s", result)
	}
}

func TestNoHTMLFormatting(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeDataWithDays(makeTestDaysData())
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: false}
	result := f.FormatGroupFull("Г1", days, opts)

	if strings.Contains(result, "<b>") {
		t.Fatalf("should not contain HTML tags when IsTelegram=false, got: %s", result)
	}
}

func TestNoLessons(t *testing.T) {
	for _, f := range AllFormatters {
		data := map[string]any{
			"days": []any{
				map[string]any{"day": "01.01.2030", "lessons": []any{}},
			},
		}
		days := extractDaysFromData(data)
		opts := FormatOptions{IsTelegram: true}
		result := f.FormatGroupFull("Г1", days, opts)

		if !strings.Contains(result, "Нет пар") && !strings.Contains(result, "нет") {
			t.Fatalf("%s: expected 'no lessons' message, got: %s", f.Name(), result)
		}
	}
}

func TestEmptyDays(t *testing.T) {
	for _, f := range AllFormatters {
		opts := FormatOptions{IsTelegram: true}
		result := f.FormatGroupFull("Г1", nil, opts)

		if !strings.Contains(result, "Нет расписания") {
			t.Fatalf("%s: expected 'no timetable' message, got: %s", f.Name(), result)
		}
	}
}

func TestFooterParserError(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeSingleLessonData()
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, HasParserError: true}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "⚠️") {
		t.Fatalf("expected parser error warning, got: %s", result)
	}
}

func TestFooterHint(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeSingleLessonData()
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, ShowHints: true, RandHint: "Тестовая подсказка"}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "Тестовая подсказка") {
		t.Fatalf("expected hint in footer, got: %s", result)
	}
}

func TestSubgroups(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeSubgroupData()
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "1.") {
		t.Fatalf("expected subgroup 1, got: %s", result)
	}
	if !strings.Contains(result, "2.") {
		t.Fatalf("expected subgroup 2, got: %s", result)
	}
}

func TestWeekLabel(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeSingleLessonData()
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, WeekLabel: "Неделя №5"}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "Неделя №5") {
		t.Fatalf("expected week label, got: %s", result)
	}
}

func TestParserTimeFooter(t *testing.T) {
	f := &DefaultFormatter{}
	data := makeSingleLessonData()
	days := extractDaysFromData(data)

	opts := FormatOptions{IsTelegram: true, ShowParserTime: true, ParserUpdateTime: time.Now().Add(-5 * time.Minute).UnixMilli()}
	result := f.FormatGroupFull("Г1", days, opts)

	if !strings.Contains(result, "загружена") {
		t.Fatalf("expected parser time footer, got: %s", result)
	}
}

func extractDaysFromData(data map[string]any) []map[string]any {
	var daysRaw []any

	switch v := data["days"].(type) {
	case []any:
		daysRaw = v
	case []map[string]any:
		daysRaw = make([]any, len(v))
		for i, m := range v {
			daysRaw[i] = m
		}
	}

	if len(daysRaw) == 0 {
		return nil
	}

	var result []map[string]any
	for _, d := range daysRaw {
		if m, ok := d.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{30, "30 сек"},
		{120, "2 мин 0 сек"},
		{3661, "1 ч 1 мин"},
	}
	for _, tt := range tests {
		got := formatSeconds(tt.secs)
		if got != tt.want {
			t.Errorf("formatSeconds(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}
