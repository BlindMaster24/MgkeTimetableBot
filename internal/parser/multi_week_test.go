package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

const multiWeekGroupHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 777</h2>
<h3>14.09.2026 - 20.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 14.09.2026</th>
<th colspan="2">Вторник, 15.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов А.А.</td>
<td class="sub">101</td>
<td>Физика<br>(Пр)<br>Петров Б.Б.</td>
<td class="sub">202</td>
</tr>
</table>
</div></div>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 777</h2>
<h3>21.09.2026 - 27.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 21.09.2026</th>
<th colspan="2">Вторник, 22.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>История<br>(Лек)<br>Орлов О.О.</td>
<td class="sub">303</td>
<td>Химия<br>(Лаб)<br>Соколов С.С.</td>
<td class="sub">404</td>
</tr>
</table>
</div></div>
</body></html>`

const overlappingWeekGroupHTML = `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 777</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 21.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Лек)<br>Иванов А.А.</td>
<td class="sub">101</td>
</tr>
</table>
</div></div>
<div class="entry"><div class="content">
<h2>Группа - 777</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 21.09.2026</th>
<th colspan="2">Вторник, 22.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Математика<br>(Пр)<br>Иванов А.А.</td>
<td class="sub">505</td>
<td>История<br>(Лек)<br>Орлов О.О.</td>
<td class="sub">303</td>
</tr>
</table>
</div></div>
</body></html>`

const multiWeekTeacherHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для преподавателей</h1>
<h2>Преподаватель - Иванов Иван Иванович</h2>
<h3>14.09.2026 - 20.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 14.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>777<br>Математика<br>(Лек)</td>
<td class="sub">101</td>
</tr>
</table>
</div></div>
<div class="entry"><div class="content">
<h1>Расписание занятий для преподавателей</h1>
<h2>Преподаватель - Иванов Иван Иванович</h2>
<h3>21.09.2026 - 27.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 21.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина</th><th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>778<br>История<br>(Лек)</td>
<td class="sub">303</td>
</tr>
</table>
</div></div>
</body></html>`

func parseGroupsHTML(t *testing.T, raw string) model.Groups {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	groups, report := NewGroupParser(doc).Parse()
	if failing := report.Failing(); len(failing) > 0 {
		t.Fatalf("required probes failed: %v", report.Summary())
	}
	return groups
}

func groupDayList(group *model.Group) []string {
	days := make([]string, 0, len(group.Days))
	for _, day := range group.Days {
		days = append(days, day.Day)
	}
	return days
}

func assertDaysInOrder(t *testing.T, group *model.Group, want []string) {
	t.Helper()

	days := groupDayList(group)
	if len(days) != len(want) {
		t.Fatalf("expected %v, got %v", want, days)
	}
	for i, day := range want {
		if days[i] != day {
			t.Fatalf("days are not merged in order: got %v, want %v", days, want)
		}
	}
}

func TestGroupParserKeepsEveryPublishedWeek(t *testing.T) {
	groups := parseGroupsHTML(t, multiWeekGroupHTML)

	group := groups["777"]
	if group == nil {
		t.Fatalf("group not parsed: %v", groups)
	}
	assertDaysInOrder(t, group, []string{"14.09.2026", "15.09.2026", "21.09.2026", "22.09.2026"})

	for i, want := range []string{"Математика", "Физика", "История", "Химия"} {
		lesson, ok := group.Days[i].Lessons[0].(*model.GroupLessonExplain)
		if !ok {
			t.Fatalf("day %d has no lesson: %v", i, group.Days[i].Lessons)
		}
		if lesson.Lesson != want {
			t.Fatalf("day %d lesson = %q, want %q", i, lesson.Lesson, want)
		}
	}
}

func TestGroupParserOverwritesRepeatedDayWithTheLatestBlock(t *testing.T) {
	groups := parseGroupsHTML(t, overlappingWeekGroupHTML)

	group := groups["777"]
	if group == nil {
		t.Fatalf("group not parsed: %v", groups)
	}
	assertDaysInOrder(t, group, []string{"21.09.2026", "22.09.2026"})

	lesson, ok := group.Days[0].Lessons[0].(*model.GroupLessonExplain)
	if !ok {
		t.Fatalf("no lesson parsed: %v", group.Days[0].Lessons)
	}
	if lesson.Cabinet == nil || *lesson.Cabinet != "505" {
		t.Fatalf("the later block should win for a repeated date, got %v", lesson.Cabinet)
	}
}

func TestTeacherParserKeepsEveryPublishedWeek(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(multiWeekTeacherHTML))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	teachers, report := NewTeacherParser(doc).Parse()
	if failing := report.Failing(); len(failing) > 0 {
		t.Fatalf("required probes failed: %v", report.Summary())
	}

	teacher, ok := teachers["Иванов Иван Иванович"]
	if !ok {
		t.Fatalf("teacher not parsed: %v", teachers)
	}
	if len(teacher.Days) != 2 {
		t.Fatalf("expected both weeks, got %d days", len(teacher.Days))
	}
	if teacher.Days[0].Day != "14.09.2026" || teacher.Days[1].Day != "21.09.2026" {
		t.Fatalf("days are not merged in order: %v", teacher.Days)
	}
}
