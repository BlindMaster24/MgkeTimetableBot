package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func docFromHTML(t *testing.T, html string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	return doc
}

func TestParseTeamCards(t *testing.T) {
	doc := docFromHTML(t, `<html><body>
<div class="entry employees-list">
<div class="employee-card"><h5 class="employee-card-title">Иванов Иван Иванович</h5></div>
<div class="employee-card"><h5 class="employee-card-title">Петрова Мария Сергеевна</h5></div>
</div>
</body></html>`)

	team := ParseTeam(doc, nil)

	if len(team) != 2 {
		t.Fatalf("expected 2 members, got %d: %v", len(team), team)
	}
	if team["Иванов И. И."] != "Иванов Иван Иванович" {
		t.Errorf("short name mapping missing: %v", team)
	}
	if team["Петрова М. С."] != "Петрова Мария Сергеевна" {
		t.Errorf("short name mapping missing: %v", team)
	}
}

func TestParseTeamItems(t *testing.T) {
	doc := docFromHTML(t, `<html><body>
<div class="main container"><div id="main-p">
<div class="item"><div class="content"><h3>Сидоров Пётр Олегович</h3><p>Директор</p></div></div>
<div class="item"><div class="content"><h3>Кузьмина Анна Петровна</h3></div></div>
</div></div>
</body></html>`)

	team := ParseTeam(doc, nil)

	if team["Сидоров П. О."] != "Сидоров Пётр Олегович" {
		t.Errorf("expected the director in the list: %v", team)
	}
	if team["Кузьмина А. П."] != "Кузьмина Анна Петровна" {
		t.Errorf("expected the second member in the list: %v", team)
	}
}

func TestParseTeamKeepsExisting(t *testing.T) {
	doc := docFromHTML(t, `<html><body>
<div class="entry employees-list">
<div class="employee-card"><h5 class="employee-card-title">Новый Преподаватель Иванович</h5></div>
</div>
</body></html>`)

	team := ParseTeam(doc, map[string]string{"Старый П. И.": "Старый Пётр Иванович"})

	if team["Старый П. И."] != "Старый Пётр Иванович" {
		t.Errorf("existing members must survive another page: %v", team)
	}
	if team["Новый П. И."] != "Новый Преподаватель Иванович" {
		t.Errorf("new member missing: %v", team)
	}
}

func TestParseTeamSkipsUnparsableNames(t *testing.T) {
	doc := docFromHTML(t, `<html><body>
<div class="entry employees-list">
<div class="employee-card"><h5 class="employee-card-title">Вакансия</h5></div>
<div class="employee-card"><h5 class="employee-card-title">  </h5></div>
</div>
</body></html>`)

	team := ParseTeam(doc, nil)

	if len(team) != 0 {
		t.Errorf("expected no members, got %v", team)
	}
}
