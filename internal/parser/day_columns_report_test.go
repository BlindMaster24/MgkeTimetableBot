package parser

import (
	"strings"
	"testing"
)

func TestGroupParserReportsSkippedDayColumns(t *testing.T) {
	html := `<html><body><h2>Группа - 63ТП</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th><th colspan="2">Вторник, 31.02.2026</th></tr>
<tr><th>D</th><th>A</th><th>D</th><th>A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td>101</td><td>Физика<br>(Лек)<br>Петров</td><td>102</td></tr>
</table></body></html>`

	groups, report := NewGroupParser(docFromHTML(t, html)).Parse()
	group := groups["63ТП"]
	if group == nil {
		t.Fatalf("group missing, report: %s", report.Summary())
	}
	if len(group.Days) != 1 || group.Days[0].Day != "31.08.2026" {
		t.Fatalf("days = %+v, want only the valid date", group.Days)
	}
	if !hasWarning(report, "unreadable date") {
		t.Errorf("report must mention the skipped column: %s", report.Summary())
	}
}

func TestGroupParserSkipsUndatedColumns(t *testing.T) {
	html := `<html><body><h2>Группа - 63ТП</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th><th colspan="2">Вторник</th></tr>
<tr><th>D</th><th>A</th><th>D</th><th>A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td>101</td><td>Физика<br>(Лек)<br>Петров</td><td>102</td></tr>
</table></body></html>`

	groups, report := NewGroupParser(docFromHTML(t, html)).Parse()
	group := groups["63ТП"]
	if group == nil {
		t.Fatalf("group missing, report: %s", report.Summary())
	}
	if len(group.Days) != 1 || group.Days[0].Day != "31.08.2026" {
		t.Fatalf("days = %+v, want only the dated day", group.Days)
	}
	if !hasWarning(report, "carried no date") {
		t.Errorf("report must mention the undated column: %s", report.Summary())
	}
}

func TestTeacherParserSkipsUndatedColumns(t *testing.T) {
	html := `<html><body><h2>Преподаватель - Иванов А.А.</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th><th>Вторник</th></tr>
<tr><th>D</th><th>A</th><th>D</th></tr>
<tr><th>1</th><td>63ТП-Математика<br>(Лек)</td><td>101</td><td>63ТП-Физика<br>(Лек)</td></tr>
</table></body></html>`

	teachers, report := NewTeacherParser(docFromHTML(t, html)).Parse()
	teacher := teachers["Иванов А.А."]
	if teacher == nil {
		t.Fatalf("teacher missing, report: %s", report.Summary())
	}
	if len(teacher.Days) != 1 || teacher.Days[0].Day != "31.08.2026" {
		t.Fatalf("days = %+v, want only the dated day", teacher.Days)
	}
	if !hasWarning(report, "carried no date") {
		t.Errorf("report must mention the undated column: %s", report.Summary())
	}
}

func TestGroupParserRejectsNonPaddedDateHeaders(t *testing.T) {
	html := `<html><body><h2>Группа - 63ТП</h2><table>
<tr><th>№</th><th colspan="2">Понедельник, 1.9.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td>101</td></tr>
</table></body></html>`

	groups, report := NewGroupParser(docFromHTML(t, html)).Parse()
	if group := groups["63ТП"]; group != nil {
		t.Fatalf("a header with 1.9.2026 must not become a day label: %+v", group.Days)
	}
	if !hasWarning(report, "unreadable date") {
		t.Errorf("report must mention the skipped column: %s", report.Summary())
	}
}

func TestTeacherParserReportsSkippedDayColumns(t *testing.T) {
	html := `<html><body><h2>Преподаватель - Иванов А.А.</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th><th colspan="2">Вторник, 31.02.2026</th></tr>
<tr><th>D</th><th>A</th><th>D</th><th>A</th></tr>
<tr><th>1</th><td>63ТП-Математика<br>(Лек)</td><td>101</td><td>63ТП-Физика<br>(Лек)</td><td>102</td></tr>
</table></body></html>`

	teachers, report := NewTeacherParser(docFromHTML(t, html)).Parse()
	teacher := teachers["Иванов А.А."]
	if teacher == nil {
		t.Fatalf("teacher missing, report: %s", report.Summary())
	}
	if len(teacher.Days) != 1 || teacher.Days[0].Day != "31.08.2026" {
		t.Fatalf("days = %+v, want only the valid date", teacher.Days)
	}
	if !hasWarning(report, "unreadable date") {
		t.Errorf("report must mention the skipped column: %s", report.Summary())
	}
}

func TestUndatedDayColumnFailsTheLayoutProbe(t *testing.T) {
	html := `<html><body><h2>Группа - 63ТП</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th><th colspan="2">Вторник</th></tr>
<tr><th>D</th><th>A</th><th>D</th><th>A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td>101</td><td>Физика<br>(Лек)<br>Петров</td><td>102</td></tr>
</table></body></html>`

	groups, report := NewGroupParser(docFromHTML(t, html)).Parse()
	if group := groups["63ТП"]; group == nil || len(group.Days) != 1 {
		t.Fatalf("the dated day must survive, report: %s", report.Summary())
	}
	if report.OK() {
		t.Errorf("a page with an undated column must not look clean: %s", report.Summary())
	}

	failing := report.Failing()
	if len(failing) != 1 {
		t.Fatalf("failing probes = %+v, want only the undated column", failing)
	}
	if failing[0].Selector != "th: day header without dd.MM.yyyy" || !failing[0].Inverted || failing[0].Missing != 1 {
		t.Errorf("probe = %+v", failing[0])
	}
	if !strings.Contains(report.Summary(), "skipped: th: day header without dd.MM.yyyy (1 skipped)") {
		t.Errorf("summary = %q", report.Summary())
	}
}

func TestDatedDayColumnsKeepTheLayoutClean(t *testing.T) {
	html := `<html><body><h2>Группа - 63ТП</h2><table>
<tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th></tr>
<tr><th>D</th><th>A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td>101</td></tr>
</table></body></html>`

	_, report := NewGroupParser(docFromHTML(t, html)).Parse()
	if !report.OK() {
		t.Errorf("a fully dated page must stay clean: %s", report.Summary())
	}
	for _, probe := range report.Probes {
		if probe.Inverted && probe.Missing != 0 {
			t.Errorf("a dated page must not report a skipped column: %+v", probe)
		}
	}
}

func hasWarning(report Report, fragment string) bool {
	for _, warning := range report.Warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
