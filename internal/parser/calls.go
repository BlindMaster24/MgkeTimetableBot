package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

var callsTimeRe = regexp.MustCompile(`\b(\d{1,2})[.:](\d{2})\b`)
var saturdayRe = regexp.MustCompile(`(?i)(суббот|выход|сб\.)`)
var callsLineRe = regexp.MustCompile(`(?m)^\s*\d{1,2}\s*(?:пара|звонок)?[.:]?\s*\d{1,2}[.:]\d{2}.*$`)

func ParseCallsSchedule(doc *goquery.Document) *cache.Schedule {
	schedule, _ := ParseCallsScheduleReport(doc)
	return schedule
}

func ParseCallsScheduleReport(doc *goquery.Document) (*cache.Schedule, Report) {
	builder := newReport(SourceCalls, "")

	scope, selector, scoped := findScope(doc)
	builder.probe(selector, "content block with tables", scope.Find("table").Length(), true)

	tables := scope.Find("table")
	if tables.Length() == 0 && scoped {
		builder.fallback("table search fallback: whole document")
		tables = doc.Find("table")
	}
	builder.probe("table", "bell schedule tables", tables.Length(), true)

	var weekdaySlots [][2][2]string
	var saturdaySlots [][2][2]string
	tablesWithSlots := 0

	tables.Each(func(_ int, table *goquery.Selection) {
		slots := extractCallSlots(table)
		if len(slots) == 0 {
			return
		}
		tablesWithSlots++

		if saturdayRe.MatchString(tableHeading(table, doc)) {
			saturdaySlots = append(saturdaySlots, slots...)
			return
		}
		if len(slots) > len(weekdaySlots) {
			weekdaySlots = slots
		}
	})

	builder.probe("tr with two time ranges", "bell schedule rows", tablesWithSlots, true)

	if len(weekdaySlots) == 0 {
		if extracted := extractCallsFromText(scope, builder); len(extracted) > 0 {
			weekdaySlots = extracted
		}
	}

	if len(weekdaySlots) == 0 {
		builder.warn("no bell schedule slots were recognized")
		return nil, builder.done(0)
	}

	if len(saturdaySlots) == 0 {
		saturdaySlots = weekdaySlots
	}

	if len(weekdaySlots) > 16 {
		builder.warn("%d weekday slots look like a mix of several schedules", len(weekdaySlots))
		weekdaySlots = weekdaySlots[:16]
	}

	schedule := &cache.Schedule{
		Weekdays: weekdaySlots,
		Saturday: saturdaySlots,
	}

	return schedule, builder.done(len(weekdaySlots))
}

func extractCallSlots(table *goquery.Selection) [][2][2]string {
	var slots [][2][2]string

	table.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td, th")
		if cells.Length() == 0 {
			return
		}

		timeCell := cells.Last()
		timeCell.Find("br").ReplaceWithHtml(" ")
		text := timeCell.Text()

		lines := strings.Split(strings.TrimSpace(text), "\n")
		if len(lines) < 1 {
			lines = []string{text}
		}

		if slot := parseCallSlot(lines[len(lines)-1]); slot != nil {
			slots = append(slots, *slot)
			return
		}

		if cells.Length() >= 2 {
			joined := rowTimeText(cells)
			if slot := parseCallSlot(joined); slot != nil {
				slots = append(slots, *slot)
			}
		}
	})

	return slots
}

func rowTimeText(cells *goquery.Selection) string {
	parts := make([]string, 0, cells.Length())
	cells.Each(func(_ int, cell *goquery.Selection) {
		text := normalizeCallsText(cell.Text())
		if text != "" {
			parts = append(parts, text)
		}
	})
	return strings.Join(parts, " ")
}

func extractCallsFromText(scope *goquery.Selection, builder *reportBuilder) [][2][2]string {
	text := scope.Text()
	var slots [][2][2]string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !callsLineRe.MatchString(line) {
			continue
		}
		if slot := parseCallSlot(line); slot != nil {
			slots = append(slots, *slot)
		}
	}

	if len(slots) > 0 {
		builder.fallback("bell schedule read from plain text")
	}
	return slots
}

func normalizeCallsText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return strings.Join(strings.Fields(text), " ")
}

func parseCallSlot(text string) *[2][2]string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u2013", " ")
	text = strings.ReplaceAll(text, "\u2014", " ")
	text = strings.ReplaceAll(text, "&ndash;", " ")
	text = strings.ReplaceAll(text, "&mdash;", " ")

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
