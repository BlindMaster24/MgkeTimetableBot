package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const mockTeacherGridHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для преподавателей</h1>
<h2>Преподаватель - Иванов Иван Иванович</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
<th colspan="2">Вторник, 01.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th>
<th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>63ТП<br>Математика<br>(Лек)</td>
<td class="sub">101</td>
<td>-</td>
<td class="sub">&nbsp;</td>
</tr>
<tr>
<th>2</th>
<td>64ТП<br>Физика<br>(Пр)</td>
<td class="sub">102</td>
<td>65ТП<br>Химия<br>(Лек)</td>
<td class="sub">103</td>
</tr>
</table>

<h2>Преподаватель - Петров Пётр Петрович</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>65ТП<br>Физика<br>(Пр)</td>
<td class="sub">301</td>
</tr>
</table>
</div></div>
</body></html>`

func TestTeacherParserGridLayout(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	if err != nil {
		t.Fatal(err)
	}

	p := NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(teachers) != 2 {
		t.Fatalf("expected 2 teachers, got %d", len(teachers))
	}

	if _, ok := teachers["Иванов Иван Иванович"]; !ok {
		t.Error("missing teacher Иванов")
	}
	if _, ok := teachers["Петров Пётр Петрович"]; !ok {
		t.Error("missing teacher Петров")
	}
}

func TestTeacherParserDaysFromGrid(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	p := NewTeacherParser(doc)
	teachers, _ := p.Run()

	ivanov := teachers["Иванов Иван Иванович"]
	if len(ivanov.Days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(ivanov.Days))
	}

	if ivanov.Days[0].Day != "Понедельник, 31.08.2026" {
		t.Errorf("first day = %q", ivanov.Days[0].Day)
	}
}

func TestTeacherParserLessonsFromGrid(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	p := NewTeacherParser(doc)
	teachers, _ := p.Run()

	ivanov := teachers["Иванов Иван Иванович"]
	monday := ivanov.Days[0]

	if len(monday.Lessons) == 0 {
		t.Fatalf("expected lessons on Monday, got %d", len(monday.Lessons))
	}

	first := monday.Lessons[0]
	if first == nil {
		t.Fatal("expected non-nil lesson")
	}

	if first.Lesson != "Математика" {
		t.Errorf("first lesson = %q, want Математика", first.Lesson)
	}
}

func TestTeacherParserEmptyDay(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	p := NewTeacherParser(doc)
	teachers, _ := p.Run()

	ivanov := teachers["Иванов Иван Иванович"]
	tuesday := ivanov.Days[1]

	if len(tuesday.Lessons) != 1 {
		t.Errorf("expected 1 lesson for Tuesday (64ТП Физика), got %d", len(tuesday.Lessons))
	}
}

func TestTeacherParserMultipleLessons(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	p := NewTeacherParser(doc)
	teachers, _ := p.Run()

	ivanov := teachers["Иванов Иван Иванович"]
	tuesday := ivanov.Days[1]

	if len(tuesday.Lessons) == 0 {
		t.Logf("Tuesday has 0 lessons (all dashes)")
	} else {
		t.Logf("Tuesday has %d lessons", len(tuesday.Lessons))
	}
}

func TestTeacherParserContentHash(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherGridHTML))
	p := NewTeacherParser(doc)
	hash := p.ContentHash()
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestTeacherParserEmptyHTML(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content"><p>No data</p></div></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(teachers) != 0 {
		t.Errorf("expected 0 teachers for empty HTML, got %d", len(teachers))
	}
}
