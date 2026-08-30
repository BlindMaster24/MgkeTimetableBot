package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

const mockGroupGridHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 63ТП</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
<th colspan="2">Вторник, 01.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов И.И.</td>
<td class="sub">101</td>
<td>Физика<br>(Пр)<br>Петров П.П.</td>
<td class="sub">102</td>
</tr>
<tr>
<th>2</th>
<td>1.Русский язык<br>(ЛР)<br>Козлова К.К.
2.Англ. язык<br>(ЛР)<br>Смирнова С.С.</td>
<td class="sub">301
302</td>
<td>-</td>
<td class="sub">&nbsp;</td>
</tr>
</table>

<h2>Группа - 64ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Информатика<br>(Лек)<br>Белов Б.Б.</td>
<td class="sub">201</td>
</tr>
</table>
</div></div>
</body></html>`

func TestGroupParserGridLayout(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	if err != nil {
		t.Fatal(err)
	}

	p := NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	if _, ok := groups["63ТП"]; !ok {
		t.Error("missing group 63ТП")
	}
	if _, ok := groups["64ТП"]; !ok {
		t.Error("missing group 64ТП")
	}
}

func TestGroupParserDaysFromGrid(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["63ТП"]
	if g == nil {
		t.Fatal("expected group 63ТП")
	}

	if len(g.Days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(g.Days))
	}

	if g.Days[0].Day != "Понедельник, 31.08.2026" {
		t.Errorf("first day = %q", g.Days[0].Day)
	}
	if g.Days[1].Day != "Вторник, 01.09.2026" {
		t.Errorf("second day = %q", g.Days[1].Day)
	}
}

func TestGroupParserLessonsFromGrid(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["63ТП"]
	monday := g.Days[0]

	if len(monday.Lessons) == 0 {
		t.Fatal("expected lessons on Monday")
	}

	first := monday.Lessons[0]
	if first == nil {
		t.Fatal("expected non-nil lesson")
	}

	single := model.AsSingle(first)
	if single == nil {
		t.Fatal("expected single lesson")
	}

	if single.Lesson != "Математика" {
		t.Errorf("first lesson = %q, want Математика", single.Lesson)
	}
	if single.Cabinet == nil || *single.Cabinet != "101" {
		t.Errorf("cabinet = %v, want 101", single.Cabinet)
	}
}

func TestGroupParserSubgroupsFromGrid(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["63ТП"]
	monday := g.Days[0]

	if len(monday.Lessons) < 2 {
		t.Fatalf("expected at least 2 lessons, got %d", len(monday.Lessons))
	}

	second := monday.Lessons[1]
	subs := model.AsArray(second)
	if subs == nil {
		t.Fatal("expected subgroup array for lesson 2")
	}

	if len(subs) != 2 {
		t.Fatalf("expected 2 subgroups, got %d", len(subs))
	}

	if subs[0].Lesson != "Русский язык" {
		t.Errorf("subgroup 1 lesson = %q", subs[0].Lesson)
	}
	if subs[1].Lesson != "Англ. язык" {
		t.Errorf("subgroup 2 lesson = %q", subs[1].Lesson)
	}
}

func TestGroupParserEmptyLesson(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["63ТП"]
	tuesday := g.Days[1]

	if len(tuesday.Lessons) != 1 {
		t.Errorf("expected 1 lesson for Tuesday (Физика), got %d", len(tuesday.Lessons))
	}
}

func TestGroupParserClearEndingNulls(t *testing.T) {
	html := `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 99</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов</td>
<td class="sub">101</td>
</tr>
<tr>
<th>5</th>
<td>&nbsp;</td>
<td class="sub">&nbsp;</td>
</tr>
</table>
</div></div>
</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["99"]
	if g == nil {
		t.Fatal("expected group 99")
	}

	day := g.Days[0]
	for _, l := range day.Lessons[1:] {
		if l != nil {
			t.Errorf("expected nil lesson after clearing, got %+v", l)
		}
	}
}

func TestGroupParserMultipleSubgroups(t *testing.T) {
	html := `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 100</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr>
<th>1</th>
<td>1.ФЗКиЗ<br>(Лек)<br>Бураков
2.ФЗКиЗ<br>(Лек)<br>Усикова</td>
<td class="sub">-
-</td>
</tr>
</table>
</div></div>
</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	g := groups["100"]
	if g == nil {
		t.Fatal("expected group 100")
	}

	lesson := g.Days[0].Lessons[0]
	subs := model.AsArray(lesson)
	if subs == nil {
		t.Fatal("expected subgroups")
	}

	if len(subs) != 2 {
		t.Fatalf("expected 2 subgroups, got %d", len(subs))
	}

	if subs[0].Lesson != "ФЗКиЗ" {
		t.Errorf("subgroup 1 lesson = %q", subs[0].Lesson)
	}
}

func TestGroupParserContentHash(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(mockGroupGridHTML))
	p := NewGroupParser(doc)
	hash := p.ContentHash()
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestGroupParserEmptyHTML(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content"><p>No timetable</p></div></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for empty HTML, got %d", len(groups))
	}
}

func TestGroupParserAsteriskGroup(t *testing.T) {
	html := `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 101*</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr>
<th>1</th>
<td>Химия<br>(Лек)<br>Белова</td>
<td class="sub">103</td>
</tr>
</table>
</div></div>
</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := NewGroupParser(doc)
	groups, _ := p.Run()

	if _, ok := groups["101"]; !ok {
		t.Error("expected group 101 (asterisk stripped)")
	}
}

func TestExtractType(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"(Лек)", "Лек"},
		{"(Пр)", "Пр"},
		{"(ЛР)", "ЛР"},
		{"Нет типа", ""},
	}
	for _, tt := range tests {
		got := extractType(tt.input)
		if got != tt.want {
			t.Errorf("extractType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRemoveDashes(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"-", ""},
		{"—", ""},
		{"101", "101"},
		{"- -", ""},
	}
	for _, tt := range tests {
		got := removeDashes(tt.input)
		if got != tt.want {
			t.Errorf("removeDashes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShortenSubjectName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Материалы ЭТех", "МатЭТех"},
		{"Физ химия", "ФизХим"},
		{"Математика", "Математика"},
	}
	for _, tt := range tests {
		got := shortenSubjectName(tt.input)
		if got != tt.want {
			t.Errorf("shortenSubjectName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
