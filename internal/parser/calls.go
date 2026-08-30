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

	var weekdays [][2][2]string

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

func extractCallSlots(table *goquery.Selection) [][2][2]string {
	var slots [][2][2]string

	rows := table.Find("tr")
	rows.Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}

		timeCell := cells.Last()
		timeCell.Find("br").ReplaceWithHtml(" ")
		timeText := timeCell.Text()

		slot := parseCallSlot(timeText)
		if slot != nil {
			slots = append(slots, *slot)
		}
	})

	return slots
}

func parseCallSlot(text string) *[2][2]string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u2013", " ")
	text = strings.ReplaceAll(text, "\u2014", " ")

	matches := callsTimeRe.FindAllStringSubmatch(text, -1)
	if len(matches) < 2 {
		return nil
	}

	if len(matches) >= 4 {
		start1 := normalizeCallsTime(matches[0][1], matches[0][2])
		end1 := normalizeCallsTime(matches[1][1], matches[1][2])
		start2 := normalizeCallsTime(matches[2][1], matches[2][2])
		end2 := normalizeCallsTime(matches[3][1], matches[3][2])
		slot := [2][2]string{{start1, end1}, {start2, end2}}
		return &slot
	}

	start := normalizeCallsTime(matches[0][1], matches[0][2])
	end := normalizeCallsTime(matches[1][1], matches[1][2])
	slot := [2][2]string{{start, end}, {start, end}}
	return &slot
}

func normalizeCallsTime(h, m string) string {
	if len(h) == 1 {
		h = "0" + h
	}
	return h + ":" + m
}
