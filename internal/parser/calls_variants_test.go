package parser

import "testing"

const twoCampusesHTML = `<html><body><div class="entry"><div class="content">
<h2>Расписание звонков</h2>
<h5>Учебный корпус по улице Казинца</h5>
<div class="table-responsive">
<table>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br />10.45 &ndash; 11.30</td></tr>
<tr><td>3 пара</td><td>11.50 &ndash; 12.35<br />12.45 &ndash; 13.30</td></tr>
</table>
</div>
<h5>Учебный корпус по улице Кнорина</h5>
<div class="table-responsive">
<table>
<tr><td>1 пара</td><td>9.00 &ndash; 9.45<br />9.55 &ndash; 10.40</td></tr>
</table>
</div>
</div></div></body></html>`

func TestCallsParserSeparatesCampuses(t *testing.T) {
	doc := docFromHTML(t, twoCampusesHTML)
	variants, report := ParseCallsVariants(doc)

	if len(variants) != 2 {
		t.Fatalf("expected two variants, got %d: %s", len(variants), report.Summary())
	}

	byName := make(map[string]int)
	for _, variant := range variants {
		byName[variant.Name] = variant.Slots()
	}
	if byName["Казинца"] != 3 || byName["Кнорина"] != 1 {
		t.Fatalf("variants = %v", byName)
	}

	if len(variants[0].Schedule.Weekdays) != 3 {
		t.Errorf("the default variant must be the largest one: %v", variants[0])
	}
	if variants[1].Schedule.Weekdays[0][0][0] != "09:00" {
		t.Errorf("Кнорина starts at %s", variants[1].Schedule.Weekdays[0][0][0])
	}
	if len(variants[0].Schedule.Saturday) == 0 {
		t.Error("a variant without its own saturday must reuse the weekdays")
	}

	if len(report.Variants) != 2 {
		t.Errorf("the report must list the variants: %+v", report)
	}
}

func TestCallsParserSingleCampusStaysUnnamed(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<h2>Расписание звонков</h2>
<table><tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr></table>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	variants, report := ParseCallsVariants(doc)

	if len(variants) != 1 {
		t.Fatalf("expected a single variant, got %d", len(variants))
	}
	if variants[0].Name != "" {
		t.Errorf("a single schedule needs no campus name, got %q", variants[0].Name)
	}
	if len(report.Variants) != 0 {
		t.Errorf("no variants should be reported for a single schedule: %+v", report.Variants)
	}
}

func TestCallsParserSaturdayBelongsToItsCampus(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<h5>Учебный корпус по улице Казинца</h5>
<table><tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr></table>
<h5>Учебный корпус по улице Казинца, суббота</h5>
<table><tr><td>1 пара</td><td>9.00 &ndash; 9.45<br />9.55 &ndash; 10.40</td></tr></table>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	variants, _ := ParseCallsVariants(doc)

	if len(variants) != 1 {
		t.Fatalf("expected one campus, got %d: %+v", len(variants), variants)
	}
	weekdays := variants[0].Schedule.Weekdays
	saturday := variants[0].Schedule.Saturday
	if len(weekdays) != 1 || len(saturday) != 1 {
		t.Fatalf("weekdays=%d saturday=%d", len(weekdays), len(saturday))
	}
	if weekdays[0][0][0] != "08:00" {
		t.Errorf("weekday start = %s", weekdays[0][0][0])
	}
	if saturday[0][0][0] != "09:00" {
		t.Errorf("saturday start = %s", saturday[0][0][0])
	}
}
