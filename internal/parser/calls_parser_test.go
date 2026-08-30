package parser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const mockBellScheduleHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание звонков</h1>
<table class="table table-bordered">
<thead>
<tr><th colspan="2">1 смена</th></tr>
</thead>
<tbody>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br/>8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br/>10.45 &ndash; 11.30</td></tr>
<tr><td>3 пара</td><td>11.50 &ndash; 12.35<br/>12.45 &ndash; 13.30</td></tr>
<tr><th colspan="2">2 смена</th></tr>
<tr><td>4 пара</td><td>13.40 &ndash; 14.25<br/>14.35 &ndash; 15.20</td></tr>
<tr><td>5 пара</td><td>15.40 &ndash; 17.10</td></tr>
<tr><td>6 пара</td><td>17.20 &ndash; 18.50</td></tr>
<tr><td>7 пара</td><td>19.00 &ndash; 20.30</td></tr>
</tbody>
</table>
</div></div>
</body></html>`

func TestParseCallsSchedule(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	if schedule == nil {
		t.Fatal("expected non-nil schedule")
	}

	if len(schedule.Weekdays) == 0 {
		t.Fatal("expected weekdays calls")
	}

	if len(schedule.Weekdays) < 6 {
		t.Fatalf("expected at least 6 lesson slots, got %d", len(schedule.Weekdays))
	}

	if schedule.Weekdays[0][0] != "08:00" {
		t.Errorf("first lesson start = %q, want 08:00", schedule.Weekdays[0][0])
	}
	if schedule.Weekdays[0][1] != "08:45" {
		t.Errorf("first lesson end = %q, want 08:45", schedule.Weekdays[0][1])
	}
}

func TestParseCallsScheduleTwoHalves(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	lesson1 := schedule.Weekdays[0]
	if lesson1[0] != "08:00" || lesson1[1] != "08:45" {
		t.Errorf("lesson 1 first half = %v, want [08:00 08:45]", lesson1)
	}

	lesson1second := schedule.Weekdays[1]
	if lesson1second[0] != "08:55" || lesson1second[1] != "09:40" {
		t.Errorf("lesson 1 second half = %v, want [08:55 09:40]", lesson1second)
	}
}

func TestParseCallsScheduleSingleTime(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	lesson5 := schedule.Weekdays[8]
	if lesson5[0] != "15:40" || lesson5[1] != "17:10" {
		t.Errorf("lesson 5 (single time) = %v, want [15:40 17:10]", lesson5)
	}
}

func TestParseCallsScheduleSaturday(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	if len(schedule.Saturday) != len(schedule.Weekdays) {
		t.Errorf("Saturday should equal Weekdays, got %d vs %d", len(schedule.Saturday), len(schedule.Weekdays))
	}
}

func TestParseCallsScheduleEmptyHTML(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content"><p>No schedule</p></div></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	schedule := ParseCallsSchedule(doc)

	if schedule != nil {
		t.Error("expected nil for empty HTML")
	}
}

func TestParseCallTimes(t *testing.T) {
	tests := []struct {
		input string
		count int
		first [2]string
	}{
		{"8.00 – 8.45", 1, [2]string{"08:00", "08:45"}},
		{"8.00 – 8.45\n8.55 – 9.40", 2, [2]string{"08:00", "08:45"}},
		{"9:00 – 9:45\n9:55 – 10:40", 2, [2]string{"09:00", "09:45"}},
		{"", 0, [2]string{}},
		{"no times here", 0, [2]string{}},
	}

	for _, tt := range tests {
		result := parseCallTimes(tt.input)
		if len(result) != tt.count {
			t.Errorf("parseCallTimes(%q) returned %d items, want %d", tt.input, len(result), tt.count)
			continue
		}
		if tt.count > 0 && result[0] != tt.first {
			t.Errorf("parseCallTimes(%q)[0] = %v, want %v", tt.input, result[0], tt.first)
		}
	}
}

func TestNormalizeCallsTime(t *testing.T) {
	tests := []struct {
		h, m, want string
	}{
		{"8", "00", "08:00"},
		{"12", "30", "12:30"},
		{"0", "45", "00:45"},
	}
	for _, tt := range tests {
		got := normalizeCallsTime(tt.h, tt.m)
		if got != tt.want {
			t.Errorf("normalizeCallsTime(%q, %q) = %q, want %q", tt.h, tt.m, got, tt.want)
		}
	}
}

func TestFetchAndParseCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mockBellScheduleHTML))
	}))
	defer srv.Close()

	schedule := fetchAndParseCalls(&http.Client{}, srv.URL)
	if schedule == nil {
		t.Fatal("expected non-nil schedule")
	}

	if len(schedule.Weekdays) < 6 {
		t.Errorf("expected at least 6 weekday slots, got %d", len(schedule.Weekdays))
	}
}

func TestFetchAndParseCallsConnectionError(t *testing.T) {
	schedule := fetchAndParseCalls(&http.Client{}, "http://127.0.0.1:1")
	if schedule != nil {
		t.Error("expected nil for connection error")
	}
}
