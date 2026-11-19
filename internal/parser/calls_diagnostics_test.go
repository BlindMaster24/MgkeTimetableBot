package parser

import (
	"strings"
	"testing"
)

func callsPage(heading, rows string) string {
	return `<html><body><div class="entry"><div class="content">
<h2>Расписание звонков</h2>` + heading + `<table>
` + rows + `
</table>
</div></div></body></html>`
}

const validCallsRow = `<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr>`

func failingProbe(t *testing.T, report Report, selector string) Probe {
	t.Helper()

	for _, probe := range report.Failing() {
		if probe.Selector == selector {
			return probe
		}
	}
	t.Fatalf("probe %q is not failing: %s", selector, report.Summary())
	return Probe{}
}

func TestCallsParserReportsAMissingPageDateAsAdvisory(t *testing.T) {
	variants, report := ParseCallsVariants(docFromHTML(t, callsPage("", validCallsRow)))

	if len(variants) != 1 || len(variants[0].Schedule.Weekdays) != 1 {
		t.Fatalf("the slots must still parse: %+v", variants)
	}
	if failing := report.Failing(); len(failing) != 0 {
		t.Fatalf("a missing page date must not fail the parse: %+v", failing)
	}
	if !report.OK() {
		t.Errorf("the slots and rows are fine, so the parse stays clean: %s", report.Summary())
	}
	advisories := report.Advisories()
	if len(advisories) != 1 || advisories[0].Selector != "[date-updated] with dd.MM.yyyy" {
		t.Fatalf("advisories = %+v", advisories)
	}
	if !strings.Contains(report.Summary(), "advisory: [date-updated] with dd.MM.yyyy") {
		t.Errorf("summary = %q", report.Summary())
	}

	dated := callsPage(`<h5>с 01.09.2026 по 07.09.2026</h5>`, validCallsRow)
	if _, datedReport := ParseCallsVariants(docFromHTML(t, dated)); len(datedReport.Advisories()) != 0 {
		t.Errorf("a dated heading must satisfy the date probe: %s", datedReport.Summary())
	}
}

func TestCallsParserReportsRowsWithoutAPairLabel(t *testing.T) {
	rows := validCallsRow + "\n" + `<tr><td>первая</td><td>9.50 &ndash; 10.35<br />10.45 &ndash; 11.30</td></tr>`
	variants, report := ParseCallsVariants(docFromHTML(t, callsPage(`<h5>01.09.2026</h5>`, rows)))

	if len(variants) != 1 || len(variants[0].Schedule.Weekdays) != 2 {
		t.Fatalf("both slots must parse: %+v", variants)
	}
	probe := failingProbe(t, report, "td: pair label")
	if !probe.Inverted || probe.Missing != 1 {
		t.Errorf("probe = %+v, want one unlabelled row", probe)
	}
	if !hasWarning(report, "carried no readable pair label") {
		t.Errorf("report must mention the unlabelled row: %s", report.Summary())
	}
	if !strings.Contains(report.Summary(), "td: pair label (1 skipped)") {
		t.Errorf("summary = %q", report.Summary())
	}
}

func TestCallsParserIgnoresShiftHeaderRows(t *testing.T) {
	rows := `<tr><th colspan="2">1 смена</th></tr>` + "\n" + validCallsRow
	_, report := ParseCallsVariants(docFromHTML(t, callsPage(`<h5>01.09.2026</h5>`, rows)))

	if !report.OK() {
		t.Errorf("a shift header is not a slot row: %s", report.Summary())
	}
}

func TestCallsParserReportsSlotRowsWithoutAValidSlot(t *testing.T) {
	rows := validCallsRow + "\n" + `<tr><td>2 пара</td><td>8.00 &ndash; 8.00</td></tr>`
	_, report := ParseCallsVariants(docFromHTML(t, callsPage(`<h5>01.09.2026</h5>`, rows)))

	probe := failingProbe(t, report, "tr: two time ranges")
	if !probe.Inverted || probe.Missing != 1 {
		t.Errorf("probe = %+v, want one unreadable row", probe)
	}
	if !hasWarning(report, "carried times but no valid slot") {
		t.Errorf("report must mention the unreadable row: %s", report.Summary())
	}
}

func TestCallsParserMarksTheRequiredProbes(t *testing.T) {
	_, report := ParseCallsVariants(docFromHTML(t, callsPage(`<h5>01.09.2026</h5>`, validCallsRow)))

	bySelector := make(map[string]Probe, len(report.Probes))
	for _, probe := range report.Probes {
		bySelector[probe.Selector] = probe
	}

	required := map[string]int{
		"table":                   1,
		"tr with two time ranges": 1,
	}
	for selector, found := range required {
		probe, ok := bySelector[selector]
		if !ok {
			t.Fatalf("probe %q is missing: %s", selector, report.Summary())
		}
		if !probe.Required || probe.Advisory {
			t.Errorf("probe %q must be required and not advisory: %+v", selector, probe)
		}
		if probe.Found != found {
			t.Errorf("probe %q found %d, want %d", selector, probe.Found, found)
		}
	}

	date := bySelector["[date-updated] with dd.MM.yyyy"]
	if !date.Advisory || date.Required {
		t.Errorf("the page date must stay advisory: %+v", date)
	}
	if !report.OK() {
		t.Errorf("a healthy page must parse cleanly: %s", report.Summary())
	}
}

func TestCallsParserUsesTheScopedTablesOnly(t *testing.T) {
	page := `<html><body>
<div class="entry"><div class="content">
<h2>Расписание звонков</h2>
<h5>01.09.2026</h5>
<table><tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr></table>
</div></div>
<table><tr><td>9 пара</td><td>20.00 &ndash; 20.45<br />20.55 &ndash; 21.40</td></tr></table>
</body></html>`

	variants, report := ParseCallsVariants(docFromHTML(t, page))
	if len(variants) != 1 {
		t.Fatalf("variants = %+v", variants)
	}
	if got := len(variants[0].Schedule.Weekdays); got != 1 {
		t.Fatalf("only the scoped table may be parsed, got %d slots", got)
	}

	var contentBlock Probe
	for _, probe := range report.Probes {
		if probe.Expected == "content block with tables" {
			contentBlock = probe
		}
	}
	if !contentBlock.Required || contentBlock.Advisory {
		t.Errorf("the scoped block probe must stay required: %+v", contentBlock)
	}
}

func TestCallsParserKeepsGoingAfterAnEmptyTable(t *testing.T) {
	page := `<html><body><div class="entry"><div class="content">
<h2>Расписание звонков</h2>
<h5>01.09.2026</h5>
<table><tr><th>Смена</th></tr></table>
<table><tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr></table>
</div></div></body></html>`

	variants, _ := ParseCallsVariants(docFromHTML(t, page))
	if len(variants) != 1 || len(variants[0].Schedule.Weekdays) != 1 {
		t.Fatalf("an empty table before a full one must not stop the parse: %+v", variants)
	}
}
