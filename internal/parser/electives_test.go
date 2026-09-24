package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

const mockElectivesHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 63ТП</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
<th colspan="2">Вторник, 01.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>Факультатив по математике<br>(ф-в)<br>Иванов И.И.</td>
<td class="sub">101</td>
<td>Химия<br>(Лек)<br>Петров П.П.</td>
<td class="sub">103</td>
</tr>
<tr>
<th>2</th>
<td>Факультатив по математике<br>(ф-в)<br>Иванов И.И.</td>
<td class="sub">101</td>
<td>Химия<br>(Лек)<br>Петров П.П.</td>
<td class="sub">103</td>
</tr>
<tr>
<th>3</th>
<td>Математика<br>(Лек)<br>Сидоров С.С.</td>
<td class="sub">102</td>
<td>-</td>
<td class="sub">&nbsp;</td>
</tr>
</table>
</div></div>
</body></html>`

const mockTeacherElectivesHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для преподавателей</h1>
<h2>Преподаватель - Иванов И.И.</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 31.08.2026</th>
</tr>
<tr>
<th class="sub">Группа, дисциплина, вид занятия</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>63ТП<br>Факультатив по математике<br>(ф-в)</td>
<td class="sub">101</td>
</tr>
<tr>
<th>2</th>
<td>63ТП<br>Факультатив по математике<br>(ф-в)</td>
<td class="sub">101</td>
</tr>
<tr>
<th>3</th>
<td>63ТП<br>Математика<br>(Лек)</td>
<td class="sub">102</td>
</tr>
</table>
</div></div>
</body></html>`

func parseGroupFixture(t *testing.T, html string) *model.Group {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	groups, err := NewGroupParser(doc).Run()
	if err != nil {
		t.Fatal(err)
	}

	group := groups["63ТП"]
	if group == nil {
		t.Fatal("group 63ТП is missing")
	}
	return group
}

func firstDayLessons(t *testing.T, group *model.Group) []model.GroupLesson {
	t.Helper()

	for _, day := range group.Days {
		if extractDayString(day.Day) == "31.08.2026" {
			return day.Lessons
		}
	}
	t.Fatal("day 31.08.2026 is missing")
	return nil
}

func TestTwoIdenticalElectivesBecomeOneWithTheTwoHoursComment(t *testing.T) {
	lessons := firstDayLessons(t, parseGroupFixture(t, mockElectivesHTML))

	if len(lessons) != 3 {
		t.Fatalf("the day must keep three slots, got %d", len(lessons))
	}

	first := model.AsSingle(lessons[0])
	if first == nil {
		t.Fatalf("the first lesson must stay a single lesson, got %#v", lessons[0])
	}
	if first.Comment == nil || *first.Comment != twoHoursComment {
		t.Fatalf("the first elective must be marked as %q, got %#v", twoHoursComment, first.Comment)
	}

	if lessons[1] != nil {
		t.Fatalf("the repeated elective must be emptied, got %#v", lessons[1])
	}

	if lessons[2] == nil {
		t.Fatal("the lesson after the electives must stay in place")
	}
}

func TestElectivesWithADifferentTeacherAreNotMerged(t *testing.T) {
	html := strings.ReplaceAll(mockElectivesHTML,
		"<th>2</th>\n<td>Факультатив по математике<br>(ф-в)<br>Иванов И.И.</td>",
		"<th>2</th>\n<td>Факультатив по математике<br>(ф-в)<br>Смирнова С.С.</td>")

	lessons := firstDayLessons(t, parseGroupFixture(t, html))

	if len(lessons) != 3 {
		t.Fatalf("expected three slots, got %d", len(lessons))
	}
	for i, lesson := range lessons {
		part := model.AsSingle(lesson)
		if part == nil {
			t.Fatalf("slot %d must stay in place", i)
		}
		if part.Comment != nil {
			t.Fatalf("slot %d must not be marked as merged, got %#v", i, part.Comment)
		}
	}
}

func TestTeacherElectivesUseTheSameMerge(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherElectivesHTML))
	if err != nil {
		t.Fatal(err)
	}

	teachers, err := NewTeacherParser(doc).Run()
	if err != nil {
		t.Fatal(err)
	}

	teacher := teachers["Иванов И.И."]
	if teacher == nil {
		t.Fatal("teacher Иванов И.И. is missing")
	}

	var lessons []model.TeacherLesson
	for _, day := range teacher.Days {
		if extractDayString(day.Day) == "31.08.2026" {
			lessons = day.Lessons
		}
	}
	if len(lessons) != 3 {
		t.Fatalf("the day must keep three slots, got %d", len(lessons))
	}
	if lessons[0] == nil || lessons[0].Comment == nil || *lessons[0].Comment != twoHoursComment {
		t.Fatalf("the first teacher elective must be marked as %q", twoHoursComment)
	}
	if lessons[0].Group != "63ТП" {
		t.Fatalf("the teacher lesson must keep its group, got %q", lessons[0].Group)
	}
	if lessons[1] != nil {
		t.Fatalf("the repeated teacher elective must be emptied, got %#v", lessons[1])
	}
	if lessons[2] == nil {
		t.Fatal("the lesson after the electives must stay in place")
	}
}
