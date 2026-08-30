package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

var callsTimeRe = regexp.MustCompile(`\b(\d{1,2})[.:](\d{2})\b`)

func ParseCallsSchedule(doc *goquery.Document) *cache.Schedule {
	content := findContent(doc)
	tables := content.Find("table")

	var weekdays [][2]string

	tables.Each(func(_ int, table *goquery.Selection) {
		slots := extractCallSlots(table)
		if len(slots) > len(weekdays) {
			weekdays = slots
		}
	})

	if len(weekdays) == 0 {
		return nil
	}

	return &cache.Schedule{
		Weekdays: weekdays,
		Saturday: weekdays,
	}
}

func extractCallSlots(table *goquery.Selection) [][2]string {
	var slots [][2]string

	rows := table.Find("tr")
	rows.Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}

		cells.Last().Find("br").ReplaceWithHtml(" ")
		timeText := cells.Last().Text()
		parsed := parseCallTimes(timeText)
		slots = append(slots, parsed...)
	})

	return slots
}

func parseCallTimes(text string) [][2]string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u2013", "-")
	text = strings.ReplaceAll(text, "\u2014", "-")

	matches := callsTimeRe.FindAllStringSubmatch(text, -1)
	if len(matches) < 2 {
		return nil
	}

	var result [][2]string
	for i := 0; i+1 < len(matches); i += 2 {
		start := normalizeCallsTime(matches[i][1], matches[i][2])
		end := normalizeCallsTime(matches[i+1][1], matches[i+1][2])
		result = append(result, [2]string{start, end})
	}

	return result
}

func normalizeCallsTime(h, m string) string {
	if len(h) == 1 {
		h = "0" + h
	}
	return h + ":" + m
}
