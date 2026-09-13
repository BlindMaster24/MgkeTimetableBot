package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
	var fallback *goquery.Selection
	var fallbackSelector string

	for _, selector := range contentScopes {
		found := doc.Find(selector)
		if found.Length() == 0 {
			continue
		}

		if found.Find("table").Length() > 0 {
			return found.First(), selector, true
		}
		if fallback == nil {
			fallback = found.First()
			fallbackSelector = selector
		}
	}

	if fallback != nil {
		return fallback, fallbackSelector, true
	}

	return doc.Selection, "document", false
}

func scopedTables(doc *goquery.Document, builder *reportBuilder) *goquery.Selection {
	scope, selector, scoped := findScope(doc)
	builder.probe(selector, "content block with tables", scope.Find("table").Length(), true)

	tables := scope.Find("table")
	if tables.Length() == 0 && scoped {
		builder.fallback("table search fallback: whole document")
		builder.probe("document", "tables anywhere", doc.Find("table").Length(), true)
		return doc.Find("table")
	}

	return tables
}

func extractDayString(text string) string {
	match := dayPattern.FindString(text)
	if match == "" {
		return ""
	}
	return strings.TrimSpace(match)
}
