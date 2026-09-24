package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var weekdayRe = regexp.MustCompile(`(?i)(понедельник|вторник|среда|четверг|пятница|суббота|воскресенье)`)
var inlineCabinetRe = regexp.MustCompile(`^(?:каб\.?\s*)?([0-9]{1,2}[-/][0-9]{2,3}(?:\s*\([^)]*\))?)$`)
var leadingSeparators = " \t\n-–—:.,*"

type DayColumn struct {
	Day        string
	LessonCol  int
	CabinetCol int
	HasCabinet bool
}

type labelMatch struct {
	Value string
	Label string
	Loose bool
	OK    bool
}

type tableRow struct {
	Cells  map[int]*goquery.Selection
	Count  int
	HasDay bool
	HasTD  bool
}

const headingLimit = 8

func precedingHeadings(doc *goquery.Document, el *goquery.Selection, limit int) []string {
	target := el.Get(0)
	root := doc.Get(0)
	if target == nil || root == nil {
		return nil
	}

	var seen []string
	var found bool

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if found {
			return
		}

		if node.Type == html.ElementNode {
			switch node.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6", "caption":
				if text := normalizeLabel(nodeText(node)); text != "" {
					seen = append(seen, text)
					if len(seen) > limit {
						seen = seen[1:]
					}
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child == target {
				found = true
				return
			}
			walk(child)
			if found {
				return
			}
		}
	}

	walk(root)

	reversed := make([]string, 0, len(seen))
	for i := len(seen) - 1; i >= 0; i-- {
		reversed = append(reversed, seen[i])
	}
	return reversed
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
			builder.WriteString(" ")
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func tableHeadings(table *goquery.Selection, doc *goquery.Document) []string {
	candidates := make([]string, 0, headingLimit)

	if caption := table.Find("caption").First(); caption.Length() > 0 {
		if text := normalizeLabel(caption.Text()); text != "" {
			candidates = append(candidates, text)
		}
	}

	for _, text := range precedingHeadings(doc, table, headingLimit) {
		if len(candidates) > 0 && candidates[len(candidates)-1] == text {
			continue
		}
		candidates = append(candidates, text)
	}

	return candidates
}

func tableLabel(table *goquery.Selection, doc *goquery.Document, match func(string) labelMatch) (labelMatch, bool) {
	for _, text := range tableHeadings(table, doc) {
		if found := match(text); found.OK {
			return found, true
		}
	}
	return labelMatch{}, false
}

func tableHeading(table *goquery.Selection, doc *goquery.Document) string {
	if headings := tableHeadings(table, doc); len(headings) > 0 {
		return headings[0]
	}
	return ""
}

func normalizeLabel(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u200b", "")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func cellSpan(cell *goquery.Selection) int {
	span := 1
	if raw, ok := cell.Attr("colspan"); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
			span = parsed
		}
	}
	return span
}

func buildDayColumns(headerRow *goquery.Selection, allowUndated bool) []DayColumn {
	var days []DayColumn
	col := 0

	headerRow.Find("th, td").Each(func(_ int, cell *goquery.Selection) {
		span := cellSpan(cell)
		text := normalizeLabel(cell.Text())

		dated := extractDayString(text) != ""
		named := weekdayRe.MatchString(text)

		if dated || (named && span > 1) || (allowUndated && named) {
			dayValue := extractDayString(text)
			if dayValue == "" {
				dayValue = text
			}
			day := DayColumn{Day: dayValue, LessonCol: col}
			if span > 1 {
				day.HasCabinet = true
				day.CabinetCol = col + 1
			} else {
				day.CabinetCol = col
			}
			days = append(days, day)
		}

		col += span
	})

	return days
}

func findHeaderRow(rows *goquery.Selection) *goquery.Selection {
	var withDate *goquery.Selection
	var withHeader *goquery.Selection

	rows.Each(func(_ int, row *goquery.Selection) {
		if withHeader == nil && row.Find("th").Length() > 0 {
			withHeader = row
		}
		if withDate != nil {
			return
		}
		row.Find("th, td").EachWithBreak(func(_ int, cell *goquery.Selection) bool {
			if extractDayString(cell.Text()) != "" {
				withDate = row
				return false
			}
			return true
		})
	})

	if withDate != nil {
		return withDate
	}
	return withHeader
}

func rowCellIndexes(row *goquery.Selection) (map[int]*goquery.Selection, int, bool) {
	cells := make(map[int]*goquery.Selection)
	col := 0
	hasDay := false

	row.Find("th, td").Each(func(_ int, cell *goquery.Selection) {
		if extractDayString(cell.Text()) != "" {
			hasDay = true
		}
		if _, taken := cells[col]; !taken {
			cells[col] = cell
		}
		col += cellSpan(cell)
	})

	return cells, col, hasDay
}

func isHeaderRow(rows *goquery.Selection, row *goquery.Selection) bool {
	_, _, hasDay := rowCellIndexes(row)
	if hasDay {
		return true
	}
	return row.Find("td").Length() == 0
}

func headerRowIndex(headerRow *goquery.Selection, rows *goquery.Selection) int {
	target := headerRow.Get(0)
	if target == nil {
		return -1
	}
	found := -1
	rows.EachWithBreak(func(i int, row *goquery.Selection) bool {
		if row.Get(0) == target {
			found = i
			return false
		}
		return true
	})
	return found
}

func trimSeparators(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, leadingSeparators)
	return strings.TrimSpace(value)
}

func splitInlineCabinet(lines []string) ([]string, string) {
	if len(lines) == 0 {
		return lines, ""
	}

	last := strings.TrimSpace(lines[len(lines)-1])
	match := inlineCabinetRe.FindStringSubmatch(last)
	if match == nil {
		return lines, ""
	}

	if len(lines) == 1 {
		return lines, ""
	}

	return lines[:len(lines)-1], match[1]
}
