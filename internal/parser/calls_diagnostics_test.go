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

func TestCallsParserRequiresAParseablePageDate(t *testing.T) {
	variants, report := ParseCallsVariants(docFromHTML(t, callsPage("", validCallsRow)))

	if len(variants) != 1 || len(variants[0].Schedule.Weekdays) != 1 {
		t.Fatalf("the slots must still parse: %+v", variants)
	}
	probe := failingProbe(t, report, "[date-updated] with dd.MM.yyyy")
	if probe.Inverted {
		t.Errorf("the page date probe is not an inverted probe: %+v", probe)
	}
	if report.OK() {
		t.Errorf("a page without a date must not look clean: %s", report.Summary())
	}

	dated := callsPage(`<h5>с 01.09.2026 по 07.09.2026</h5>`, validCallsRow)
	if _, datedReport := ParseCallsVariants(docFromHTML(t, dated)); !datedReport.OK() {
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
