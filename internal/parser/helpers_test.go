package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractDayString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Понедельник, 01.09.2025", "01.09.2025"},
		{"Вторник 15.06.2025", "15.06.2025"},
		{"no date", ""},
		{"", ""},
		{"Понедельник, 00.00.0000", ""},
		{"Понедельник, 32.13.2026", ""},
		{"00.00.0000 31.08.2026", "31.08.2026"},
	}
	for _, c := range cases {
		got := extractDayString(c.input)
		if got != c.want {
			t.Errorf("extractDayString(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestDayColumnsIgnoreImpossibleDates(t *testing.T) {
	html := `<table><tr><th colspan="2">Понедельник, 00.00.0000</th><th colspan="2">Вторник, 01.09.2026</th></tr></table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	columns := buildDayColumns(doc.Find("tr").First(), true)
	if len(columns) != 1 || columns[0].Day != "01.09.2026" {
		t.Fatalf("columns = %+v, want only the real date", columns)
	}
}

func TestPtrString(t *testing.T) {
	got := ptrString("hello")
	if got == nil || *got != "hello" {
		t.Error("expected ptr to hello")
	}
	got = ptrString("")
	if got != nil {
		t.Error("expected nil for empty string")
	}
}

func TestFindContent(t *testing.T) {
	html := `<html><body><div id="main-p"><div class="content"><p>Test</p></div></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	content := findContent(doc)
	if content.Length() == 0 {
		t.Error("expected to find content div")
	}
}

func TestFindContentFallback(t *testing.T) {
	html := `<html><body><p>No content div</p></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	content := findContent(doc)
	if content.Length() == 0 {
		t.Error("expected fallback to body")
	}
}

func TestHashDocument(t *testing.T) {
	html := `<html><body><p>Test</p></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	hash := hashDocument(doc)
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashJSON(t *testing.T) {
	hash := hashJSON(map[string]string{"key": "value"})
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}
