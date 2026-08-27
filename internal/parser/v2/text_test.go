package v2

import "testing"

func TestCleanText(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  hello  ", "hello"},
		{"hello\t\tworld", "hello world"},
		{"hello\n\nworld", "hello world"},
		{"   ", ""},
	}
	for _, c := range cases {
		got := cleanText(c.input)
		if got != c.want {
			t.Errorf("cleanText(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizeDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"01.09.2025", "01.09.2025"},
		{"1.9.25", "01.09.2025"},
		{"31.12.2030", "31.12.2030"},
		{"invalid", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeDate(c.input)
		if got != c.want {
			t.Errorf("normalizeDate(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseDayLabel(t *testing.T) {
	cases := []struct {
		input   string
		wantDay string
	}{
		{"Понедельник 01.09.2025", "01.09.2025"},
		{"Вторник, 02.09.2025", "02.09.2025"},
		{"01.09.2025", "01.09.2025"},
		{"no date here", ""},
		{"", ""},
	}
	for _, c := range cases {
		day, _ := parseDayLabel(c.input)
		if day != c.wantDay {
			t.Errorf("parseDayLabel(%q) day=%q, want %q", c.input, day, c.wantDay)
		}
	}
}

func TestParseDayLabelWeekday(t *testing.T) {
	day, weekday := parseDayLabel("Понедельник 01.09.2025")
	if weekday != "Понедельник" {
		t.Errorf("expected weekday Понедельник, got %s", weekday)
	}
	if day != "01.09.2025" {
		t.Errorf("expected day 01.09.2025, got %s", day)
	}
}

func TestCapitalize(t *testing.T) {
	if got := capitalize("hello"); got != "Hello" {
		t.Errorf("capitalize(hello) = %s", got)
	}
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize('') = %s", got)
	}
}

func TestParseLessonNumber(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"1", 1},
		{"  3  ", 3},
		{"10.", 10},
		{"abc", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := parseLessonNumber(c.input)
		if got != c.want {
			t.Errorf("parseLessonNumber(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestIsTeacherLine(t *testing.T) {
	if !isTeacherLine("Иванов А.А.") {
		t.Error("expected true for Иванов А.А.")
	}
	if isTeacherLine("Математика") {
		t.Error("expected false for Математика")
	}
	if isTeacherLine("") {
		t.Error("expected false for empty")
	}
}

func TestParseTypeLine(t *testing.T) {
	got := parseTypeLine("(лекция)")
	if got == nil || *got != "лекция" {
		t.Errorf("expected лекция, got %v", got)
	}
	if parseTypeLine("лекция") != nil {
		t.Error("expected nil for non-parenthesized")
	}
}
