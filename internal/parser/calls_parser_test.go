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

	if len(schedule.Weekdays) != 7 {
		t.Fatalf("expected 7 lesson slots, got %d", len(schedule.Weekdays))
	}
}

func TestParseCallsScheduleFirstLesson(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	lesson1 := schedule.Weekdays[0]
	if lesson1[0][0] != "08:00" || lesson1[0][1] != "08:45" {
		t.Errorf("lesson 1 first half = %v, want [08:00 08:45]", lesson1[0])
	}
	if lesson1[1][0] != "08:55" || lesson1[1][1] != "09:40" {
		t.Errorf("lesson 1 second half = %v, want [08:55 09:40]", lesson1[1])
	}
}

func TestParseCallsScheduleSecondLesson(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	lesson2 := schedule.Weekdays[1]
	if lesson2[0][0] != "09:50" || lesson2[0][1] != "10:35" {
		t.Errorf("lesson 2 first half = %v, want [09:50 10:35]", lesson2[0])
	}
	if lesson2[1][0] != "10:45" || lesson2[1][1] != "11:30" {
		t.Errorf("lesson 2 second half = %v, want [10:45 11:30]", lesson2[1])
	}
}

func TestParseCallsScheduleSingleTimeLesson(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockBellScheduleHTML))
	schedule := ParseCallsSchedule(doc)

	lesson5 := schedule.Weekdays[4]
	if lesson5[0][0] != "15:40" || lesson5[0][1] != "17:10" {
		t.Errorf("lesson 5 first half = %v, want [15:40 17:10]", lesson5[0])
	}
	if lesson5[1][0] != "15:40" || lesson5[1][1] != "17:10" {
		t.Errorf("lesson 5 second half should equal first for single-time lesson, got %v", lesson5[1])
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

	if len(schedule.Weekdays) != 7 {
		t.Errorf("expected 7 weekday slots, got %d", len(schedule.Weekdays))
	}
}

func TestFetchAndParseCallsConnectionError(t *testing.T) {
	schedule := fetchAndParseCalls(&http.Client{}, "http://127.0.0.1:1")
	if schedule != nil {
		t.Error("expected nil for connection error")
	}
}
