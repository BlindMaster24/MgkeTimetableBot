package tests

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

const sampleGroupHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 63ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
<th colspan="2">Вторник, 02.09.2025</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов А.А.</td>
<td class="sub">101</td>
<td>Информатика<br>(Пр)<br>Сидоров В.В.</td>
<td class="sub">404</td>
</tr>
<tr>
<th>2</th>
<td>1.Физика<br>(Пр)<br>Петров Б.Б.
2.Физика<br>(Лек)<br>Петров Б.Б.</td>
<td class="sub">202
303</td>
<td>История<br>(Лек)<br>Орлов Г.Г.</td>
<td class="sub">210</td>
</tr>
</table>
</div></div>
</body></html>`

const sampleTeacherHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание для преподавателей</h1>
<h2>Преподаватель - Иванов А.А.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 01.09.2025</th>
<th colspan="2">Вторник, 02.09.2025</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>63ТП<br>Математика<br>(Лек)</td>
<td class="sub">101</td>
<td>64ИС<br>Физика<br>(Пр)</td>
<td class="sub">202</td>
</tr>
</table>
</div></div>
</body></html>`

func TestGroupParser(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sampleGroupHTML))
	if err != nil {
		t.Fatal(err)
	}

	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) == 0 {
		t.Fatal("expected groups, got none")
	}

	g63, ok := groups["63ТП"]
	if !ok {
		t.Fatal("expected group 63ТП")
	}

	if len(g63.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(g63.Days))
	}

	if g63.Days[0].Day != "Понедельник, 01.09.2025" {
		t.Errorf("expected day 'Понедельник, 01.09.2025', got %s", g63.Days[0].Day)
	}

	lessons := g63.Days[0].Lessons
	if len(lessons) < 2 {
		t.Fatalf("expected at least 2 lessons on day 1, got %d", len(lessons))
	}

	t.Logf("Group 63ТП: %d days, day 1 has %d lessons", len(g63.Days), len(lessons))

	hash := p.ContentHash()
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestTeacherParser(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sampleTeacherHTML))
	if err != nil {
		t.Fatal(err)
	}

	p := parser.NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(teachers) == 0 {
		t.Fatal("expected teachers, got none")
	}

	ivanov, ok := teachers["Иванов А.А."]
	if !ok {
		t.Fatal("expected teacher Иванов А.А.")
	}

	if len(ivanov.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(ivanov.Days))
	}

	t.Logf("Teacher Иванов А.А.: %d days", len(ivanov.Days))

	hash := p.ContentHash()
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}
