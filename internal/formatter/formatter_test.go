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

func makeTeacherDaysData() map[string]any {
	today := time.Now().Format("02.01.2006")
	return map[string]any{
		"days": []any{
			map[string]any{
				"day": today,
				"lessons": []any{
					map[string]any{"lesson": "Математика", "type": "лекция", "group": "100", "cabinet": "101"},
				},
			},
		},
	}
}

func TestTeacherLessonsCarryTheGroupPrefix(t *testing.T) {
	days := extractDaysFromData(makeTeacherDaysData())

	for _, f := range AllFormatters {
		opts := FormatOptions{IsTelegram: true}
		result := f.FormatTeacherFull("Иванов И.И.", days, opts)

		if result == "" {
			t.Fatalf("%s: teacher day returned empty string", f.Name())
		}
		if !strings.Contains(result, "100-Математика") {
			t.Fatalf("%s: a teacher lesson must name its group, got: %s", f.Name(), result)
		}
		if strings.Contains(result, "Иванов") {
			t.Fatalf("%s: the teacher's own schedule must not repeat the teacher name, got: %s", f.Name(), result)
		}
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
	data := map[string]any{
		"days": []any{
			map[string]any{"day": "01.01.2030", "lessons": []any{}},
		},
	}
	days := extractDaysFromData(data)

	want := map[string][2]string{
		"default": {"<i>Пар нет</i>", "Пар нет"},
		"visual":  {"🚫 Нет пар на этот день", "🚫 Нет пар на этот день"},
		"compact": {"<i>Пар нет</i>", "Пар нет"},
		"litolax": {"<i>Пар нет</i>", "Пар нет"},
	}

	for _, f := range AllFormatters {
		expected, ok := want[f.Name()]
		if !ok {
			t.Fatalf("no expectation for formatter %q", f.Name())
		}

		if got := f.FormatGroupFull("Г1", days, FormatOptions{IsTelegram: true}); !strings.Contains(got, expected[0]) {
			t.Fatalf("%s: telegram output %q must carry %q", f.Name(), got, expected[0])
		}
		if got := f.FormatGroupFull("Г1", days, FormatOptions{}); strings.Contains(got, "<i>") || !strings.Contains(got, expected[1]) {
			t.Fatalf("%s: plain output %q must carry %q without tags", f.Name(), got, expected[1])
		}
	}
}

func TestEmptyDays(t *testing.T) {
	want := map[string]string{
		"default": "Нет расписания для отображения",
		"visual":  "🚫 Нет расписания для отображения",
		"compact": "Нет расписания для отображения",
		"litolax": "Нет расписания для отображения",
	}

	for _, f := range AllFormatters {
		expected, ok := want[f.Name()]
		if !ok {
			t.Fatalf("no expectation for formatter %q", f.Name())
		}

		if got := f.FormatGroupFull("Г1", nil, FormatOptions{IsTelegram: true}); !strings.Contains(got, expected) {
			t.Fatalf("%s: expected %q, got: %s", f.Name(), expected, got)
		}
		if got := f.FormatTeacherFull("Иванов И.И.", nil, FormatOptions{IsTelegram: true}); !strings.Contains(got, expected) {
			t.Fatalf("%s: expected %q for a teacher too, got: %s", f.Name(), expected, got)
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

func TestDayHeadersUseFullWeekdayNames(t *testing.T) {
	days := []map[string]any{
		{"day": "07.09.2026", "lessons": []any{map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101"}}},
		{"day": "08.09.2026", "lessons": []any{map[string]any{"lesson": "Физика", "type": "Пр", "cabinet": "202"}}},
	}

	for _, formatter := range AllFormatters {
		plain := formatter.FormatGroupFull("777", days, FormatOptions{})
		if !strings.Contains(plain, "Понедельник, 07.09.2026") {
			t.Errorf("%s: monday header is not spelled out: %q", formatter.Name(), plain)
		}
		if !strings.Contains(plain, "Вторник, 08.09.2026") {
			t.Errorf("%s: tuesday header is not spelled out: %q", formatter.Name(), plain)
		}

		html := formatter.FormatGroupFull("777", days, FormatOptions{IsTelegram: true})
		if !strings.Contains(html, "Понедельник") || !strings.Contains(html, "07.09.2026") {
			t.Errorf("%s: monday header is missing from the telegram output: %q", formatter.Name(), html)
		}

		for _, short := range []string{"Пн,", "Вт,", "Ср,", "Чт,", "Пт,", "Сб,", "Вс,"} {
			if strings.Contains(plain, short) || strings.Contains(html, short) {
				t.Errorf("%s: abbreviated weekday leaked into the output: %q", formatter.Name(), plain)
			}
		}
	}
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		secs int64
		want string
	}{
		{0, "0 сек."},
		{30, "30 сек."},
		{59, "59 сек."},
		{60, "1 мин. 0 сек."},
		{90, "1 мин. 30 сек."},
		{120, "2 мин. 0 сек."},
		{3599, "59 мин. 59 сек."},
		{3600, "1 ч. 0 мин. 0 сек."},
		{3661, "1 ч. 1 мин. 1 сек."},
		{86399, "23 ч. 59 мин. 59 сек."},
		{86400, "1 д. 0 ч. 0 мин. 0 сек."},
		{90000, "1 д. 1 ч. 0 мин. 0 сек."},
		{31536000, "1 г. 0 д. 0 ч. 0 мин. 0 сек."},
	}
	for _, tt := range tests {
		got := FormatSeconds(tt.secs)
		if got != tt.want {
			t.Errorf("FormatSeconds(%d) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

func TestSubgroupLayoutMatchesTheOldBot(t *testing.T) {
	day := func(lessons []any) []map[string]any {
		return extractDaysFromData(map[string]any{
			"days": []any{map[string]any{"day": time.Now().Format("02.01.2006"), "lessons": lessons}},
		})
	}

	first := map[string]any{"subgroup": 1.0, "lesson": "Программирование", "type": "лабораторная", "teacher": "А", "cabinet": "10"}
	second := map[string]any{"subgroup": 2.0, "lesson": "Программирование", "type": "лабораторная", "teacher": "Б", "cabinet": "20"}

	cases := []struct {
		name    string
		lessons []any
		want    map[string]string
	}{
		{
			name:    "two subgroups",
			lessons: []any{[]any{first, second}},
			want: map[string]string{
				"default": "1. Программирование (лабораторная)\n├── 1. А {10}\n└── 2. Б {20}",
				"visual":  "1️⃣ Пара:\n    📚 Программирование (лабораторная)\n\n    🎒 Подгруппа 1:\n    🎓 А\n    🏫 10\n\n    🎒 Подгруппа 2:\n    🎓 Б\n    🏫 20",
				"compact": "1. Программирование\n- 1. {10}\n- 2. {20}",
				"litolax": "Пара: №1\n1. Программирование (лабораторная) А\n2. Программирование (лабораторная) Б\nКаб: 10 20",
			},
		},
		{
			name:    "a single subgroup still keeps the subgroup layout",
			lessons: []any{[]any{first}},
			want: map[string]string{
				"default": "1. Программирование (лабораторная)\n└── 1. А {10}",
				"visual":  "1️⃣ Пара:\n    📚 Программирование (лабораторная)\n\n    🎒 Подгруппа 1:\n    🎓 А\n    🏫 10",
				"compact": "1. Программирование\n- 1. {10}",
				"litolax": "Пара: №1\n1. Программирование (лабораторная) А\nКаб: 10",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range AllFormatters {
				expected, ok := tc.want[f.Name()]
				if !ok {
					t.Fatalf("no expectation for formatter %q", f.Name())
				}
				got := f.FormatGroupFull("100", day(tc.lessons), FormatOptions{})
				if !strings.Contains(got, expected) {
					t.Fatalf("%s: subgroup layout drifted\nwant fragment:\n%s\ngot:\n%s", f.Name(), expected, got)
				}
			}
		})
	}
}
