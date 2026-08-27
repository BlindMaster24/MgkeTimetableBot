package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var dayPattern = regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)

func findContent(doc *goquery.Document) *goquery.Selection {
	selectors := []string{
		"#main-p .content",
		".common-page-left-block .content",
		".entry .content",
		".common-page-left-block",
	}
	for _, sel := range selectors {
		if s := doc.Find(sel).First(); s.Length() > 0 {
			return s
		}
	}
	return doc.Selection
}

func extractDayString(text string) string {
	match := dayPattern.FindString(text)
	if match == "" {
		return ""
	}
	return strings.TrimSpace(match)
}
