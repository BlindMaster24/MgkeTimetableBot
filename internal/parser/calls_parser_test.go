package parser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
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

func TestParseCallsUpdatedAt(t *testing.T) {
	cases := []struct {
		name string
		html string
		raw  string
		zero bool
	}{
		{
			name: "element attribute",
			html: `<html><body><div class="entry"><div class="content" date-updated="14.09.2026 08:00"><table></table></div></div></body></html>`,
			raw:  "14.09.2026 08:00",
		},
		{
			name: "date without a clock",
			html: `<html><body><div date-updated="01.09.2026"></div></body></html>`,
			raw:  "01.09.2026",
		},
		{
			name: "extra text around the date",
			html: `<html><body><div date-updated="Обновлено 14.09.2026 в 08:00"></div></body></html>`,
			raw:  "Обновлено 14.09.2026 в 08:00",
		},
		{
			name: "no attribute at all",
			html: `<html><body><div></div></body></html>`,
			zero: true,
		},
	}

	for _, c := range cases {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(c.html))
		if err != nil {
			t.Fatal(err)
		}

		updated := ParseCallsUpdatedAt(doc)
		if updated.Raw != c.raw {
			t.Errorf("%s: raw = %q, want %q", c.name, updated.Raw, c.raw)
		}
		if c.zero && updated.At != 0 {
			t.Errorf("%s: a page without a date must not carry a timestamp, got %d", c.name, updated.At)
		}
		if !c.zero && c.raw != "" && strings.Contains(c.raw, ":") && updated.At == 0 {
			t.Errorf("%s: the clock was not parsed from %q", c.name, c.raw)
		}
	}
}

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

func fetchAndParseCalls(client *http.Client, rawURL string) *cache.Schedule {
	resp, err := fetchHTML(client, rawURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	schedule, _ := ParseCallsScheduleReport(doc)
	return schedule
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

func TestCallsParserIgnoresDatesAndImpossibleTimes(t *testing.T) {
	html := `<html><body>
<div class="entry"><div class="content">
<h1>Расписание звонков с 01.09.2026</h1>
<table class="table table-bordered">
<tr><th>Понедельник, 24.09.2026</th><th>Вторник, 25.09.2026</th><th>Среда, 26.09.2026</th><th>Четверг, 27.09.2026</th></tr>
<tr><td>1 пара</td><td>8.00 – 8.45<br>8.55 – 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 – 10.35<br>10.45 – 11.30</td></tr>
</table>
</div></div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	variants, _ := ParseCallsVariants(doc)
	if len(variants) == 0 {
		t.Fatal("expected a parsed bell schedule")
	}

	for _, variant := range variants {
		for _, slot := range append(append([][2][2]string{}, variant.Schedule.Weekdays...), variant.Schedule.Saturday...) {
			for _, pair := range slot {
				for _, value := range pair {
					if _, err := time.Parse("15:04", value); err != nil {
						t.Errorf("a date was parsed as a time: %q (%v)", value, err)
					}
				}
			}
		}
	}

	weekdays := variants[0].Schedule.Weekdays
	if len(weekdays) != 2 {
		t.Fatalf("expected the two real bell schedule rows, got %d", len(weekdays))
	}
	if weekdays[0][0] != [2]string{"08:00", "08:45"} {
		t.Errorf("unexpected first slot %v", weekdays[0])
	}
}

func TestParseCallSlotRejectsImpossiblePairs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"regular pair", "8.00 – 8.45", true},
		{"reversed pair", "10:00 00:00", true},
		{"reversed with a real end", "11:00 10:00", false},
		{"pair ending at midnight", "22:00 00:00", true},
		{"impossible hour", "24:09 25:09", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := parseCallSlot(testCase.text) != nil; got != testCase.want {
				t.Fatalf("parseCallSlot(%q) parsed = %v, want %v", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestParseCallSlotIgnoresDates(t *testing.T) {
	if slot := parseCallSlot("24.09.2026 25.09.2026 26.09.2026 27.09.2026"); slot != nil {
		t.Fatalf("dates were parsed as a slot: %v", *slot)
	}

	slot := parseCallSlot("Понедельник, 24.09.2026 8.00 – 8.45 8.55 – 9.40")
	if slot == nil {
		t.Fatal("expected the real times of the row")
	}
	if *slot != [2][2]string{{"08:00", "08:45"}, {"08:55", "09:40"}} {
		t.Fatalf("unexpected slot %v", *slot)
	}
}

func TestFetchAndParseCallsConnectionError(t *testing.T) {
	schedule := fetchAndParseCalls(&http.Client{}, "http://127.0.0.1:1")
	if schedule != nil {
		t.Error("expected nil for connection error")
	}
}
