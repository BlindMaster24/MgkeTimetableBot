package parser

import (
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

var callsTimeRe = regexp.MustCompile(`\b(\d{1,2})[.:](\d{2})\b`)
var saturdayRe = regexp.MustCompile(`(?i)(суббот|выход|сб\.)`)
var callsLineRe = regexp.MustCompile(`(?m)^\s*\d{1,2}\s*(?:пара|звонок)?[.:]?\s*\d{1,2}[.:]\d{2}.*$`)

type callsGroup struct {
	name     string
	weekdays [][2][2]string
	saturday [][2][2]string
}

func ParseCallsSchedule(doc *goquery.Document) *cache.Schedule {
	schedule, _ := ParseCallsScheduleReport(doc)
	return schedule
}

func ParseCallsScheduleReport(doc *goquery.Document) (*cache.Schedule, Report) {
	variants, report := ParseCallsVariants(doc)
	if len(variants) == 0 {
		return nil, report
	}

	schedule := cache.Schedule{
		Weekdays: variants[0].Schedule.Weekdays,
		Saturday: variants[0].Schedule.Saturday,
	}
	return &schedule, report
}

func ParseCallsVariants(doc *goquery.Document) ([]cache.CallsVariant, Report) {
	builder := newReport(SourceCalls, "")

	scope, selector, scoped := findScope(doc)
	builder.probe(selector, "content block with tables", scope.Find("table").Length(), true)

	tables := scope.Find("table")
	if tables.Length() == 0 && scoped {
		builder.fallback("table search fallback: whole document")
		tables = doc.Find("table")
	}
	builder.probe("table", "bell schedule tables", tables.Length(), true)

	var groups []*callsGroup
	byName := make(map[string]*callsGroup)
	var current *callsGroup
	tablesWithSlots := 0

	tables.Each(func(_ int, table *goquery.Selection) {
		heading := tableHeading(table, doc)
		isSaturday := saturdayRe.MatchString(heading)

		if !isSaturday {
			current = groupFor(byName, &groups, campusLabel(heading))
		}

		slots := extractCallSlots(table)
		if len(slots) == 0 {
			return
		}
		tablesWithSlots++

		if current == nil {
			current = groupFor(byName, &groups, campusLabel(heading))
		}
		if isSaturday {
			current.saturday = append(current.saturday, slots...)
			return
		}
		if len(slots) > len(current.weekdays) {
			current.weekdays = slots
		}
	})

	builder.probe("tr with two time ranges", "bell schedule rows", tablesWithSlots, true)

	variants := buildVariants(groups)
	if len(variants) == 0 {
		if extracted := extractCallsFromText(scope, builder); len(extracted) > 0 {
			variants = []cache.CallsVariant{{
				Name:     "",
				Schedule: cache.CallsSchedule{Weekdays: extracted, Saturday: extracted},
			}}
		}
	}

	if len(variants) == 0 {
		builder.warn("no bell schedule slots were recognized")
		return nil, builder.done(0)
	}

	if len(variants) == 1 {
		variants[0].Name = ""
	}
	for i := range variants {
		builder.variant(variants[i].Name)
	}

	return variants, builder.done(len(variants[0].Schedule.Weekdays))
}

func groupFor(index map[string]*callsGroup, groups *[]*callsGroup, name string) *callsGroup {
	if group, ok := index[name]; ok {
		return group
	}

	group := &callsGroup{name: name}
	index[name] = group
	*groups = append(*groups, group)
	return group
}

func buildVariants(groups []*callsGroup) []cache.CallsVariant {
	ordered := make([]*callsGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.weekdays) > 0 {
			ordered = append(ordered, group)
		}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return len(ordered[i].weekdays) > len(ordered[j].weekdays)
	})

	variants := make([]cache.CallsVariant, 0, len(ordered))
	for _, group := range ordered {
		saturday := group.saturday
		if len(saturday) == 0 {
			saturday = group.weekdays
		}
		if len(saturday) > 16 {
			saturday = saturday[:16]
		}
		weekdays := group.weekdays
		if len(weekdays) > 16 {
			weekdays = weekdays[:16]
		}

		variants = append(variants, cache.CallsVariant{
			Name: group.name,
			Schedule: cache.CallsSchedule{
				Weekdays: weekdays,
				Saturday: saturday,
			},
		})
	}
	return variants
}

func campusLabel(heading string) string {
	heading = normalizeLabel(heading)
	if heading == "" {
		return ""
	}

	lowered := strings.ToLower(heading)
	if saturdayRe.MatchString(lowered) {
		return ""
	}

	campusMarkers := []string{"корпус", "улиц", "филиал", "площадк", "здани"}
	for _, marker := range campusMarkers {
		if !strings.Contains(lowered, marker) {
			continue
		}
		fields := strings.Fields(heading)
		if len(fields) > 1 {
			if short := trimSeparators(fields[len(fields)-1]); short != "" {
				return short
			}
		}
		return heading
	}

	return heading
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
