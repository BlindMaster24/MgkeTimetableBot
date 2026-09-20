package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"golang.org/x/net/html"
)

var dayPattern = regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)

var contentScopes = []string{
	"#main-p .content",
	".common-page-left-block .content",
	".entry .content",
	"#main-p",
	".common-page-left-block",
	".entry",
}

func findContent(doc *goquery.Document) *goquery.Selection {
	scope, _, _ := findScope(doc)
	return scope
}

func findScope(doc *goquery.Document) (*goquery.Selection, string, bool) {
	scope, selector, scoped := findScopes(doc)
	return scope.First(), selector, scoped
}

func findScopes(doc *goquery.Document) (*goquery.Selection, string, bool) {
	var fallback *goquery.Selection
	var fallbackSelector string

	for _, selector := range contentScopes {
		found := doc.Find(selector)
		if found.Length() == 0 {
			continue
		}

		if found.Find("table").Length() > 0 {
			return found, selector, true
		}
		if fallback == nil {
			fallback = found
			fallbackSelector = selector
		}
	}

	if fallback != nil {
		return fallback, fallbackSelector, true
	}

	return doc.Selection, "document", false
}

func scopedTables(doc *goquery.Document, builder *reportBuilder) *goquery.Selection {
	scope, selector, scoped := findScopes(doc)
	builder.probe(selector, "content block with tables", scope.Find("table").Length(), true)

	tables := scope.Find("table")
	if tables.Length() == 0 && scoped {
		builder.fallback("table search fallback: whole document")
		builder.probe("document", "tables anywhere", doc.Find("table").Length(), true)
		return doc.Find("table")
	}

	return tables
}

func eachTable(tables *goquery.Selection, visit func(table *goquery.Selection)) {
	seen := make(map[*html.Node]bool)
	tables.Each(func(_ int, table *goquery.Selection) {
		node := table.Get(0)
		if node == nil || seen[node] {
			return
		}
		seen[node] = true
		visit(table)
	})
}

func dayDate(day string) (time.Time, bool) {
	parsed, err := time.Parse("02.01.2006", extractDayString(day))
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func mergeDays[T any](existing, incoming []T, dayOf func(T) string) []T {
	index := make(map[string]int, len(existing))
	for i, day := range existing {
		index[dayOf(day)] = i
	}

	for _, day := range incoming {
		if i, ok := index[dayOf(day)]; ok {
			existing[i] = day
			continue
		}
		index[dayOf(day)] = len(existing)
		existing = append(existing, day)
	}

	sort.SliceStable(existing, func(i, j int) bool {
		left, leftOK := dayDate(dayOf(existing[i]))
		right, rightOK := dayDate(dayOf(existing[j]))
		if !leftOK || !rightOK {
			return false
		}
		return left.Before(right)
	})

	return existing
}

func mergeGroupDays(existing, incoming *model.Group) *model.Group {
	existing.Days = mergeDays(existing.Days, incoming.Days, func(day model.GroupDay) string { return day.Day })
	return existing
}

func mergeTeacherDays(existing, incoming *model.Teacher) *model.Teacher {
	existing.Days = mergeDays(existing.Days, incoming.Days, func(day model.TeacherDay) string { return day.Day })
	return existing
}

func extractDayString(text string) string {
	match := dayPattern.FindString(text)
	if match == "" {
		return ""
	}
	return strings.TrimSpace(match)
}

func hashDocument(doc *goquery.Document) string {
	return hashBytes([]byte(doc.Text()))
}

func hashJSON(v any) string {
	data, _ := json.Marshal(v)
	return hashBytes(data)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
