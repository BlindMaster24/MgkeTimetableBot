package parser

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

const oneDayTable = `<table>
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 07.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов И.И.</td>
<td class="sub">3-205</td>
</tr>
</table>`

func groupsFromHTML(t *testing.T, html string) map[string]string {
	t.Helper()

	doc := docFromHTML(t, html)
	groups, report := NewGroupParser(doc).Parse()

	labels := make(map[string]string, len(groups))
	for label := range groups {
		labels[label] = label
	}
	if len(groups) == 0 && report.OK() {
		t.Errorf("no groups parsed and the report looks clean: %+v", report)
	}
	return labels
}

func TestGroupParserReadsHeadingVariants(t *testing.T) {
	cases := []struct {
		name    string
		heading string
		want    string
	}{
		{"h2 with a hyphen", `<h2>Группа - 100</h2>`, "100"},
		{"h3 with a colon", `<h3>Группа: 101</h3>`, "101"},
		{"en dash", `<h2>Группа – 102</h2>`, "102"},
		{"no separator", `<h2>Группа 103</h2>`, "103"},
		{"extra words", `<h2>Группа - 104 (изменения)</h2>`, "104"},
		{"uppercase", `<h2>ГРУППА - 105</h2>`, "105"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			html := `<html><body><div class="entry"><div class="content">` +
				testCase.heading + oneDayTable + `</div></div></body></html>`

			labels := groupsFromHTML(t, html)
			if _, ok := labels[testCase.want]; !ok {
				t.Errorf("expected group %s, got %v", testCase.want, labels)
			}
		})
	}
}

func TestGroupParserFindsHeadingThroughWrappers(t *testing.T) {
	html := `<html><body>
<div class="common-page-left-block"><div class="content">
<h2>Группа - 200</h2>
<div class="card"><div class="table-responsive">` + oneDayTable + `</div></div>
</div></div>
</body></html>`

	labels := groupsFromHTML(t, html)
	if _, ok := labels["200"]; !ok {
		t.Errorf("expected group 200 through nested wrappers, got %v", labels)
	}
}

func TestGroupParserReadsCaption(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<table><caption>Группа - 300</caption>
<tr>
<th rowspan="2">№</th>
<th colspan="2">Вторник, 08.09.2026</th>
</tr>
<tr><th class="sub">Предмет</th><th class="sub">Ауд.</th></tr>
<tr><th>1</th><td>Физика<br>(Пр)<br>Петров П.П.</td><td class="sub">101</td></tr>
</table>
</div></div></body></html>`

	labels := groupsFromHTML(t, html)
	if _, ok := labels["300"]; !ok {
		t.Errorf("expected the caption to name the group, got %v", labels)
	}
}

func TestGroupParserReadsSingleColumnDays(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<h2>Группа - 400</h2>
<table>
<tr>
<th>№</th>
<th>Понедельник, 07.09.2026</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td>
</tr>
</table>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	groups, _ := NewGroupParser(doc).Parse()

	group, ok := groups["400"]
	if !ok {
		t.Fatalf("expected group 400, got %v", groups)
	}
	if len(group.Days) != 1 || len(group.Days[0].Lessons) != 1 {
		t.Fatalf("expected one lesson, got %+v", group.Days)
	}

	lesson, ok := group.Days[0].Lessons[0].(*model.GroupLessonExplain)
	if !ok {
		t.Fatalf("unexpected lesson type: %T", group.Days[0].Lessons[0])
	}
	if lesson.Lesson != "Математика" {
		t.Errorf("lesson = %q", lesson.Lesson)
	}
	if lesson.Cabinet == nil || *lesson.Cabinet != "3-205" {
		t.Errorf("cabinet = %v", lesson.Cabinet)
	}
}

func TestGroupParserWithoutHeadingsReportsFailure(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">` + oneDayTable + `</div></div></body></html>`

	doc := docFromHTML(t, html)
	groups, report := NewGroupParser(doc).Parse()

	if len(groups) != 0 {
		t.Errorf("expected no groups without a heading, got %v", groups)
	}
	if failing := report.Failing(); len(failing) == 0 {
		t.Errorf("expected a failing probe, got %+v", report)
	}
}

func TestCallsParserClassifiesSaturdayByHeading(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<h5>Будни</h5>
<table><tr><td>1 пара</td><td>8.00 &ndash; 8.45<br />8.55 &ndash; 9.40</td></tr></table>
<h5>Суббота</h5>
<table><tr><td>1 пара</td><td>9.00 &ndash; 9.45<br />9.55 &ndash; 10.40</td></tr></table>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	schedule, _ := ParseCallsScheduleReport(doc)

	if schedule == nil {
		t.Fatal("expected a schedule")
	}
	if len(schedule.Weekdays) != 1 || schedule.Weekdays[0][0][0] != "08:00" {
		t.Errorf("weekdays = %v", schedule.Weekdays)
	}
	if len(schedule.Saturday) != 1 || schedule.Saturday[0][0][0] != "09:00" {
		t.Errorf("saturday = %v", schedule.Saturday)
	}
}

func TestCallsParserFallsBackToText(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<div class="schedule">
1 пара 8.00 - 8.45 8.55 - 9.40
2 пара 9.50 - 10.35 10.45 - 11.30
</div>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	schedule, report := ParseCallsScheduleReport(doc)

	if schedule == nil || len(schedule.Weekdays) != 2 {
		t.Fatalf("expected two text slots, got %+v (report: %s)", schedule, report.Summary())
	}
	if len(report.Fallbacks) == 0 {
		t.Error("expected the text fallback to be reported")
	}
}

func TestCallsParserWithoutDataReportsFailure(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content"><p>Расписание звонков скоро появится</p></div></div></body></html>`

	doc := docFromHTML(t, html)
	schedule, report := ParseCallsScheduleReport(doc)

	if schedule != nil {
		t.Errorf("expected no schedule, got %+v", schedule)
	}
	if len(report.Warnings) == 0 {
		t.Error("expected a warning about missing slots")
	}
}

func TestTeacherParserReadsHeadingVariants(t *testing.T) {
	html := `<html><body><div class="entry"><div class="content">
<h3>Преподаватель: Иванов Иван Иванович</h3>
<table>
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 07.09.2026</th>
</tr>
<tr><th class="sub">Дисциплина</th><th class="sub">Ауд.</th></tr>
<tr><th>1</th><td>63ТП<br>Математика<br>(Лек)</td><td class="sub">101</td></tr>
</table>
</div></div></body></html>`

	doc := docFromHTML(t, html)
	teachers, _ := NewTeacherParser(doc).Parse()

	if _, ok := teachers["Иванов Иван Иванович"]; !ok {
		t.Errorf("expected the teacher with a colon heading, got %v", teachers)
	}
}

func TestTeamParserFallsBackToImageAlt(t *testing.T) {
	html := `<html><body>
<div class="employees-list">
<div class="employee-card"><img src="/photo.jpg" alt="Козел Георгий Владимирович"></div>
</div>
</body></html>`

	doc := docFromHTML(t, html)
	team, report := ParseTeamReport(doc, nil)

	if team["Козел Г. В."] != "Козел Георгий Владимирович" {
		t.Errorf("expected the name from the alt text, got %v (report: %s)", team, report.Summary())
	}
	if len(report.Fallbacks) == 0 {
		t.Error("expected the alt-text fallback to be reported")
	}
}

func TestReportSummaryMentionsFailingSelectors(t *testing.T) {
	report := Report{Source: SourceGroups, Items: 0}
	report.Warn("day columns were found, but no lesson cell produced a subject")

	summary := report.Summary()
	if !strings.Contains(summary, "groups") {
		t.Errorf("summary has no source: %q", summary)
	}
	if !strings.Contains(summary, "no lesson cell") {
		t.Errorf("summary hides the warning: %q", summary)
	}
}
