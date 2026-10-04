package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func indexDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func tablesOf(t *testing.T, doc *goquery.Document) []*goquery.Selection {
	t.Helper()
	var tables []*goquery.Selection
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		tables = append(tables, table)
	})
	return tables
}

func TestHeadingIndexKeepsNearestFirst(t *testing.T) {
	doc := indexDoc(t, `<html><body>
		<h2>Группа - 100</h2>
		<h2>Понедельник</h2>
		<table><tr><td>first</td></tr></table>
		<h2>Вторник</h2>
		<table><tr><td>second</td></tr></table>
	</body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	if len(tables) != 2 {
		t.Fatalf("found %d tables, want 2", len(tables))
	}
	first := tableHeadings(tables[0], index)
	if len(first) != 2 || first[0] != "Понедельник" || first[1] != "Группа - 100" {
		t.Errorf("first table headings = %v, want nearest first", first)
	}
	second := tableHeadings(tables[1], index)
	if len(second) == 0 || second[0] != "Вторник" {
		t.Errorf("second table headings = %v, want Вторник first", second)
	}
}

func TestHeadingIndexPutsCaptionFirst(t *testing.T) {
	doc := indexDoc(t, `<html><body>
		<h2>Группа - 100</h2>
		<table><caption>Неделя 5</caption><tr><td>x</td></tr></table>
	</body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	headings := tableHeadings(tables[0], index)
	if len(headings) != 2 || headings[0] != "Неделя 5" || headings[1] != "Группа - 100" {
		t.Errorf("headings = %v, want caption first", headings)
	}
}

func TestHeadingIndexSkipsCaptionDuplicates(t *testing.T) {
	doc := indexDoc(t, `<html><body>
		<h2>Неделя 5</h2>
		<table><caption>Неделя 5</caption><tr><td>x</td></tr></table>
	</body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	headings := tableHeadings(tables[0], index)
	if len(headings) != 1 || headings[0] != "Неделя 5" {
		t.Errorf("headings = %v, want the duplicate caption dropped", headings)
	}
}

func TestHeadingIndexKeepsAnEightHeadingWindow(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("<html><body>")
	for i := 1; i <= 10; i++ {
		builder.WriteString("<h2>Заголовок ")
		builder.WriteString(strings.Repeat("x", i))
		builder.WriteString("</h2>")
	}
	builder.WriteString("<table><tr><td>x</td></tr></table></body></html>")
	doc := indexDoc(t, builder.String())
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	headings := tableHeadings(tables[0], index)
	if len(headings) != headingLimit {
		t.Fatalf("headings = %d, want a window of %d", len(headings), headingLimit)
	}
	if !strings.HasPrefix(headings[0], "Заголовок xxxxxxxxxx") {
		t.Errorf("nearest heading = %q, want the tenth", headings[0])
	}
	if !strings.HasPrefix(headings[len(headings)-1], "Заголовок xxx") {
		t.Errorf("oldest heading = %q, want the third", headings[len(headings)-1])
	}
}

func TestHeadingIndexSeesOuterHeadingsFromNestedTables(t *testing.T) {
	doc := indexDoc(t, `<html><body>
		<h2>Группа - 100</h2>
		<table><tr><td><table><tr><td>inner</td></tr></table></td></tr></table>
	</body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	if len(tables) != 2 {
		t.Fatalf("found %d tables, want 2", len(tables))
	}
	inner := tableHeadings(tables[1], index)
	if len(inner) == 0 || inner[0] != "Группа - 100" {
		t.Errorf("inner table headings = %v, want the outer heading", inner)
	}
}

func TestHeadingIndexStaysEmptyWithoutHeadings(t *testing.T) {
	doc := indexDoc(t, `<html><body><table><tr><td>x</td></tr></table></body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	if headings := tableHeadings(tables[0], index); len(headings) != 0 {
		t.Errorf("headings = %v, want none", headings)
	}
}

func TestTableLabelReadsGroupNamesFromTheIndex(t *testing.T) {
	doc := indexDoc(t, `<html><body>
		<h2>Группа - 63ТП</h2>
		<table><tr><td>x</td></tr></table>
	</body></html>`)
	index := headingIndex(doc)
	tables := tablesOf(t, doc)
	match, ok := tableLabel(tables[0], index, groupLabel)
	if !ok || match.Value != "63ТП" {
		t.Errorf("match = %+v, %v, want group 63ТП", match, ok)
	}
}
