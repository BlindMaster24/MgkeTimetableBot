package parser

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

var shortNamePattern = regexp.MustCompile(`(\W+)\s+(\W)\W+\s+(\W)\W+`)

func ParseTeam(doc *goquery.Document, team map[string]string) map[string]string {
	if team == nil {
		team = make(map[string]string)
	}

	cards := doc.Find(".entry.employees-list .employee-card")
	if cards.Length() > 0 {
		cards.Each(func(_ int, card *goquery.Selection) {
			addTeamMember(team, card.Find("h5.employee-card-title").First().Text())
		})
		return team
	}

	doc.Find(".main.container #main-p > div.item").Each(func(_ int, item *goquery.Selection) {
		content := item.Find(".content").First()
		if content.Length() == 0 {
			return
		}
		addTeamMember(team, content.Find("h3").First().Text())
	})

	return team
}

func addTeamMember(team map[string]string, rawName string) {
	fullName := trimSpaces(rawName)
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
