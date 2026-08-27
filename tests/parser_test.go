package tests

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

const sampleGroupHTML = `<html><body>
<div id="main-p"><div class="content">
<h2>Понедельник, 01.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>63</td><td>Математика лекция Иванов А.А. 101</td></tr>
<tr><td>63</td><td>1. Физика пр-ка Петров Б.Б. 202
2. Физика лекция Петров Б.Б. 303</td></tr>
</table>
<h2>Вторник, 02.09.2025</h2>
<table>
<tr><th>Группа</th><th>Предмет</th></tr>
<tr><td>63</td><td>Информатика practicum Сидоров В.В. 404</td></tr>
</table>
</div></div>
</body></html>`

const sampleTeacherHTML = `<html><body>
<div id="main-p"><div class="content">
<h3>Иванов А.А.</h3>
<table>
<tr><th>День</th><th>Предмет</th></tr>
<tr><td>01.09.2025</td><td>63 Математика лекция 101</td></tr>
<tr><td>02.09.2025</td><td>63 Математика practicum 101
64 Математика лекция 202</td></tr>
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

	g63, ok := groups["63"]
	if !ok {
		t.Fatal("expected group 63")
	}

	if len(g63.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(g63.Days))
	}

	if g63.Days[0].Day != "01.09.2025" {
		t.Errorf("expected day 01.09.2025, got %s", g63.Days[0].Day)
	}

	lessons := g63.Days[0].Lessons
	if len(lessons) < 2 {
		t.Fatalf("expected at least 2 lessons on day 1, got %d", len(lessons))
	}

	t.Logf("Group 63: %d days, day 1 has %d lessons", len(g63.Days), len(lessons))

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

	tov, ok := teachers["Иванов А.А."]
	if !ok {
		t.Fatal("expected teacher Иванов А.А.")
	}

	if len(tov.Days) < 2 {
		t.Fatalf("expected at least 2 days, got %d", len(tov.Days))
	}

	t.Logf("Teacher Иванов А.А.: %d days", len(tov.Days))

	hash := p.ContentHash()
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}
