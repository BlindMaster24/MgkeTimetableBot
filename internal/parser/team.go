package parser

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var shortNamePattern = regexp.MustCompile(`(\W+)\s+(\W)\W+\s+(\W)\W+`)

var teamCardSelectors = []string{
	".entry.employees-list .employee-card",
	".employees-list .employee-card",
	".employee-card",
	".staff-list .staff-card",
}

var teamNameSelectors = []string{
	"h5.employee-card-title",
	".employee-card-title",
	"h3",
	"h4",
	"h5",
}

func ParseTeam(doc *goquery.Document, team map[string]string) map[string]string {
	updated, _ := ParseTeamReport(doc, team)
	return updated
}

func ParseTeamReport(doc *goquery.Document, team map[string]string) (map[string]string, Report) {
	return ParseTeamReportCtx(context.Background(), doc, team)
}

func ParseTeamReportCtx(ctx context.Context, doc *goquery.Document, team map[string]string) (map[string]string, Report) {
	if team == nil {
		team = make(map[string]string)
	}

	builder := newReport(SourceTeam, "")
	result := make(map[string]string, len(team))
	for short, full := range team {
		result[short] = full
	}

	cardsFound := 0
	for _, selector := range teamCardSelectors {
		cards := doc.Find(selector)
		if cards.Length() == 0 {
			continue
		}

		if selector != teamCardSelectors[0] {
			builder.fallback("staff cards found by " + selector)
		}

		cardsFound = cards.Length()
		for i := 0; i < cards.Length(); i++ {
			if err := ctx.Err(); err != nil {
				break
			}
			addTeamMember(result, cardName(cards.Eq(i), builder))
		}
		break
	}

	if cardsFound == 0 {
		items := doc.Find(".main.container #main-p > div.item .content")
		if items.Length() > 0 {
			builder.fallback("legacy staff layout")
			cardsFound = items.Length()
		}
		for i := 0; i < items.Length(); i++ {
			if err := ctx.Err(); err != nil {
				break
			}
			addTeamMember(result, items.Eq(i).Find("h3").First().Text())
		}
	}

	builder.probe(".employee-card", "staff cards", cardsFound, true)
	builder.probe("h5.employee-card-title, h3, h4, h5, img[alt]", "staff names", len(result), true)

	return result, builder.done(len(result))
}

func cardName(card *goquery.Selection, builder *reportBuilder) string {
	for i, selector := range teamNameSelectors {
		text := card.Find(selector).First().Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		if i > 0 {
			builder.fallback("staff name read from " + selector)
		}
		return text
	}

	if alt, ok := card.Find("img[alt]").First().Attr("alt"); ok {
		builder.fallback("staff name read from the image alt text")
		return alt
	}

	return ""
}

func addTeamMember(team map[string]string, rawName string) {
	fullName := strings.TrimSpace(rawName)
	if fullName == "" {
		return
	}
	shortName := shortTeacherName(fullName)
	if shortName == "" {
		return
	}
	team[shortName] = fullName
}

func shortTeacherName(fullName string) string {
	parts := shortNamePattern.FindStringSubmatch(fullName)
	if len(parts) != 4 {
		return ""
	}
	return parts[1] + " " + parts[2] + ". " + parts[3] + "."
}
