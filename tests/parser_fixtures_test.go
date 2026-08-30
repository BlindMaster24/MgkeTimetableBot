package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

const groupHTMLMultiDay = `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
<th colspan="2">Вторник, 02.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td class="sub">101</td><td>Информатика<br>(Пр)<br>Сидоров</td><td class="sub">404</td></tr>
</table>
<h2>Группа - 64ИС</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
<th colspan="2">Вторник, 02.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Английский<br>(Пр)<br>Козлова</td><td class="sub">105</td><td>История<br>(Лек)<br>Орлов</td><td class="sub">210</td></tr>
</table>
</div></div>
</body></html>`

const groupHTMLSubgroups = `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>1.Физика<br>(Пр)<br>Иванов
2.Физика<br>(Пр)<br>Петров</td><td class="sub">101
202</td></tr>
<tr><th>2</th><td>Математика<br>(Лек)<br>Сидоров</td><td class="sub">303</td></tr>
</table>
</div></div>
</body></html>`

const groupHTMLEmpty = `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 71М</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>-</td><td class="sub">&nbsp;</td></tr>
</table>
</div></div>
</body></html>`

const teacherHTMLMultiDay = `<html><body>
<div class="entry"><div class="content">
<h2>Преподаватель - Иванов А.А.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
<th colspan="2">Вторник, 02.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>63ТП<br>Математика<br>(Лек)</td><td class="sub">101</td><td>-</td><td class="sub">&nbsp;</td></tr>
</table>
<h2>Преподаватель - Петров Б.Б.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>63ТП<br>Физика<br>(Пр)</td><td class="sub">202</td></tr>
</table>
</div></div>
</body></html>`

const teacherHTMLMultiLine = `<html><body>
<div class="entry"><div class="content">
<h2>Преподаватель - Сидоров В.В.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>63ТП<br>Информатика<br>(Пр)
71М<br>Информатика<br>(Лек)</td><td class="sub">404
404</td></tr>
</table>
</div></div>
</body></html>`

func TestGroupParserV2_Fixtures(t *testing.T) {
	tests := []struct {
		name           string
		html           string
		expectedGroups int
		minDays        int
	}{
		{"multi_day", groupHTMLMultiDay, 2, 1},
		{"subgroups", groupHTMLSubgroups, 1, 1},
		{"empty_lessons", groupHTMLEmpty, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			if err != nil {
				t.Fatal(err)
			}
			p := parser.NewGroupParser(doc)
			groups, err := p.Run()
			if err != nil {
				t.Fatal(err)
			}
			if len(groups) != tt.expectedGroups {
				t.Errorf("expected %d groups, got %d", tt.expectedGroups, len(groups))
			}
			for name, g := range groups {
				if len(g.Days) < tt.minDays {
					t.Errorf("group %s: expected at least %d days, got %d", name, tt.minDays, len(g.Days))
				}
			}
		})
	}
}

func TestGroupParser_VerifyLessonContent(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLMultiDay))
	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	g63, ok := groups["63ТП"]
	if !ok {
		t.Fatal("expected group 63ТП")
	}

	g64, ok := groups["64ИС"]
	if !ok {
		t.Fatal("expected group 64ИС")
	}

	t.Logf("63ТП: %d days, 64ИС: %d days", len(g63.Days), len(g64.Days))
}

func TestGroupParser_Subgroups(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLSubgroups))
	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	g63, ok := groups["63ТП"]
	if !ok {
		t.Fatal("expected group 63ТП")
	}

	day1 := g63.Days[0]
	if len(day1.Lessons) < 1 {
		t.Fatalf("expected at least 1 lesson, got %d", len(day1.Lessons))
	}

	t.Logf("Lesson 1 type: %T", day1.Lessons[0])
}

func TestGroupParser_EmptyLessons(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLEmpty))
	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	g71, ok := groups["71М"]
	if !ok {
		t.Fatal("expected group 71М")
	}
	if len(g71.Days) < 1 {
		t.Fatal("expected at least 1 day")
	}
}

func TestGroupParser_HashConsistency(t *testing.T) {
	doc1, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLMultiDay))
	p1 := parser.NewGroupParser(doc1)
	p1.Run()
	hash1 := p1.ContentHash()

	doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLMultiDay))
	p2 := parser.NewGroupParser(doc2)
	p2.Run()
	hash2 := p2.ContentHash()

	if hash1 != hash2 {
		t.Errorf("same HTML should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestGroupParser_DifferentHTML_DifferentHash(t *testing.T) {
	doc1, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLMultiDay))
	p1 := parser.NewGroupParser(doc1)
	p1.Run()
	hash1 := p1.ContentHash()

	doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLSubgroups))
	p2 := parser.NewGroupParser(doc2)
	p2.Run()
	hash2 := p2.ContentHash()

	if hash1 == hash2 {
		t.Error("different HTML should produce different hashes")
	}
}

func TestTeacherParserV2_Fixtures(t *testing.T) {
	tests := []struct {
		name             string
		html             string
		expectedTeachers int
	}{
		{"multi_day", teacherHTMLMultiDay, 2},
		{"multi_line", teacherHTMLMultiLine, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			if err != nil {
				t.Fatal(err)
			}
			p := parser.NewTeacherParser(doc)
			teachers, err := p.Run()
			if err != nil {
				t.Fatal(err)
			}
			if len(teachers) != tt.expectedTeachers {
				t.Errorf("expected %d teachers, got %d", tt.expectedTeachers, len(teachers))
			}
		})
	}
}

func TestTeacherParser_VerifyContent(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(teacherHTMLMultiDay))
	p := parser.NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	ivanov, ok := teachers["Иванов А.А."]
	if !ok {
		t.Fatal("expected teacher Иванов А.А.")
	}

	t.Logf("Ivanov: %d days", len(ivanov.Days))
	for i, d := range ivanov.Days {
		t.Logf("  Day %d (%s): %d lessons", i, d.Day, len(d.Lessons))
	}
}

func TestTeacherParser_HashConsistency(t *testing.T) {
	doc1, _ := goquery.NewDocumentFromReader(strings.NewReader(teacherHTMLMultiDay))
	p1 := parser.NewTeacherParser(doc1)
	p1.Run()
	hash1 := p1.ContentHash()

	doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(teacherHTMLMultiDay))
	p2 := parser.NewTeacherParser(doc2)
	p2.Run()
	hash2 := p2.ContentHash()

	if hash1 != hash2 {
		t.Errorf("same HTML should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestGroupParser_JSONSerialization(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(groupHTMLMultiDay))
	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	var restored map[string]any
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if _, ok := restored["63ТП"]; !ok {
		t.Error("expected group 63ТП in restored JSON")
	}
	if _, ok := restored["64ИС"]; !ok {
		t.Error("expected group 64ИС in restored JSON")
	}
}

func TestTeacherParser_JSONSerialization(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(teacherHTMLMultiDay))
	p := parser.NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.MarshalIndent(teachers, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	var restored map[string]any
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if _, ok := restored["Иванов А.А."]; !ok {
		t.Error("expected teacher Ivanov in restored JSON")
	}
	if _, ok := restored["Петров Б.Б."]; !ok {
		t.Error("expected teacher Petrov in restored JSON")
	}
}
