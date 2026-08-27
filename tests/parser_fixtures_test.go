package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

const groupHTMLMultiDay = `<html><body>
<div id="main-p"><div class="content">
<h2>Понедельник, 01.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>63ТП</td><td>Математика лекция Иванов А.А. 101</td></tr>
<tr><td>63ТП</td><td>Физика пр-ка Петров Б.Б. 202</td></tr>
<tr><td>64ИС</td><td>Английский пр-ка Козлова Е.Е. 105</td></tr>
</table>
<h2>Вторник, 02.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>63ТП</td><td>Информатика practicum Сидоров В.В. 404</td></tr>
<tr><td>64ИС</td><td>История лекция Орлов Г.Г. 210</td></tr>
</table>
</div></div>
</body></html>`

const groupHTMLSubgroups = `<html><body>
<div id="main-p"><div class="content">
<h2>Понедельник, 01.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>63ТП</td><td>1. Физика пр-ка Иванов А.А. 101
2. Физика пр-ка Петров Б.Б. 202</td></tr>
<tr><td>63ТП</td><td>Математика лекция Сидоров В.В. 303</td></tr>
</table>
</div></div>
</body></html>`

const groupHTMLEmpty = `<html><body>
<div id="main-p"><div class="content">
<h2>Понедельник, 01.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>71М</td><td>-</td></tr>
</table>
</div></div>
</body></html>`

const teacherHTMLMultiDay = `<html><body>
<div id="main-p"><div class="content">
<h3>Иванов А.А.</h3>
<table>
<tr><th>День</th><th>Предмет</th></tr>
<tr><td>01.09.2025</td><td>63ТП Математика лекция 101</td></tr>
<tr><td>01.09.2025</td><td>64ИС Математика practicum 101</td></tr>
<tr><td>02.09.2025</td><td>63ТП Физика пр-ка 202</td></tr>
</table>
<h3>Петров Б.Б.</h3>
<table>
<tr><th>День</th><th>Предмет</th></tr>
<tr><td>01.09.2025</td><td>63ТП Физика пр-ка 202</td></tr>
<tr><td>01.09.2025</td><td>71М Физика лекция 303</td></tr>
</table>
</div></div>
</body></html>`

const teacherHTMLMultiLine = `<html><body>
<div id="main-p"><div class="content">
<h3>Сидоров В.В.</h3>
<table>
<tr><th>День</th><th>Предмет</th></tr>
<tr><td>01.09.2025</td><td>63ТП Информатика practicum 404
71М Информатика лекция 404</td></tr>
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
		{"multi_day", groupHTMLMultiDay, 2, 2},
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
	if len(g63.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(g63.Days))
	}

	day1 := g63.Days[0]
	if day1.Day != "01.09.2025" {
		t.Errorf("expected day 01.09.2025, got %s", day1.Day)
	}
	if len(day1.Lessons) < 2 {
		t.Fatalf("expected at least 2 lessons on day 1, got %d", len(day1.Lessons))
	}

	for i, l := range day1.Lessons {
		t.Logf("Day 1 lesson %d: %+v", i+1, l)
	}

	g64, ok := groups["64ИС"]
	if !ok {
		t.Fatal("expected group 64ИС")
	}
	if len(g64.Days) < 1 {
		t.Fatalf("expected at least 1 day for 64ИС")
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
	if len(day1.Lessons) < 2 {
		t.Fatalf("expected at least 2 lessons, got %d", len(day1.Lessons))
	}

	l1 := day1.Lessons[0]
	if l1 == nil {
		t.Fatal("expected non-nil first lesson")
	}

	if arr, ok := l1.([]*model.GroupLessonExplain); ok {
		if len(arr) != 2 {
			t.Errorf("expected 2 subgroups, got %d", len(arr))
		}
	} else if single, ok := l1.(*model.GroupLessonExplain); ok {
		t.Logf("single lesson: %+v", single)
	}

	t.Logf("Lesson 1 type: %T, value: %+v", l1, l1)
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
	if hash1 == "" {
		t.Error("expected non-empty hash")
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
	if len(ivanov.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(ivanov.Days))
	}

	t.Logf("Ivanov: %d days", len(ivanov.Days))
	for i, d := range ivanov.Days {
		t.Logf("  Day %d (%s): %d lessons", i, d.Day, len(d.Lessons))
		for j, l := range d.Lessons {
			t.Logf("    Lesson %d: %+v", j+1, l)
		}
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
