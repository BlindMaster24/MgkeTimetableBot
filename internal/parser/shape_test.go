package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

const mockTeacherShapeHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для преподавателей</h1>
<h2>Преподаватель - Агеенкова Д. Д.</h2>
<h3>21.09.2026 - 27.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Пятница, 25.09.2026</th>
<th colspan="2">Суббота, 26.09.2026</th>
</tr>
<tr>
<th class="sub">Группа, дисциплина, вид занятия</th>
<th class="sub">Ауд.</th>
<th class="sub">Группа, дисциплина, вид занятия</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>1. 98-Инструмент ПО<br>(ЛР)</td>
<td class="sub">2-206 (к)</td>
<td>98-Инструмент ПО<br>(Лек)</td>
<td class="sub">2-207 (к)</td>
</tr>
<tr>
<th>2</th>
<td>1. 81-Веб-програмСерв<br>(ЛР)</td>
<td class="sub">3-112 (к)</td>
<td>1. 98-Инструмент ПО<br>(ЛР)<br>2. 99-Инструмент ПО<br>(ЛР)</td>
<td class="sub">3-111 (к)<br>3-113 (к)</td>
</tr>
</table>
</div></div>
</body></html>`

const mockGroupShapeHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание занятий для групп</h1>
<h2>Группа - 100</h2>
<h3>21.09.2026 - 27.09.2026</h3>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 21.09.2026</th>
<th colspan="2">Вторник, 22.09.2026</th>
</tr>
<tr>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
<th class="sub">Дисциплина, вид занятия, преподаватель</th>
<th class="sub">Ауд.</th>
</tr>
<tr>
<th>1</th>
<td>2.Основы инж гр<br>(ЛР)<br>Воронько С. В.</td>
<td class="sub">3-212 (к)</td>
<td>Материалы ЭТех<br>(Лек)<br>Самарская Н. В.</td>
<td class="sub">3-113</td>
</tr>
<tr>
<th>2</th>
<td>1.Физ химия<br>(ЛР)<br>Хомченко И. И.<br>2.Осн элек и микр<br>(ЛР)<br>Мурашко А. В.</td>
<td class="sub">2-109<br>3-202 (к)</td>
<td>1.Основы инж гр<br>(ЛР)<br>Воронько С. В.<br>2.Материалы ЭТех<br>(ЛР)<br>Самарская Н. В.</td>
<td class="sub">3-212 (к)</td>
</tr>
<tr>
<th>3</th>
<td>2.Осн элек и микр<br>(ЛР)<br>Мурашко А. В.<br>3.Физ химия<br>(ЛР)<br>Хомченко И. И.</td>
<td class="sub">а<br>б<br>в</td>
<td>-</td>
<td class="sub">&nbsp;</td>
</tr>
</table>
</div></div>
</body></html>`

func TestTeacherCellsParseLikeTheOldParser(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherShapeHTML))
	if err != nil {
		t.Fatal(err)
	}
	teachers, err := NewTeacherParser(doc).Run()
	if err != nil {
		t.Fatal(err)
	}

	entry := teachers["Агеенкова Д. Д."]
	if entry == nil {
		t.Fatal("teacher is missing")
	}
	if len(entry.Days) != 2 {
		t.Fatalf("days = %d", len(entry.Days))
	}

	friday := entry.Days[0].Lessons
	if len(friday) != 2 {
		t.Fatalf("friday lessons = %d", len(friday))
	}

	first := friday[0]
	if first == nil || first.Subgroup == nil || *first.Subgroup != 1 {
		t.Fatalf("the leading number is the subgroup, got %#v", first)
	}
	if first.Group != "98" || first.Lesson != "Инструмент ПО" {
		t.Fatalf("entry split at the first dash: group %q lesson %q", first.Group, first.Lesson)
	}
	if first.Type == nil || *first.Type != "ЛР" {
		t.Fatalf("type = %v", first.Type)
	}
	if first.Cabinet == nil || *first.Cabinet != "2-206 (к)" {
		t.Fatalf("cabinet = %v", first.Cabinet)
	}

	second := friday[1]
	if second == nil || second.Subgroup == nil || *second.Subgroup != 1 {
		t.Fatalf("the leading number is the subgroup, got %#v", second)
	}
	if second.Group != "81" || second.Lesson != "Веб" {
		t.Fatalf("the old parser cuts the subject at the second dash: group %q lesson %q", second.Group, second.Lesson)
	}

	saturday := entry.Days[1].Lessons
	if len(saturday) != 2 {
		t.Fatalf("one slot per row, saturday lessons = %d", len(saturday))
	}
	plain := saturday[0]
	if plain == nil || plain.Subgroup != nil {
		t.Fatalf("no subgroup without the leading number, got %#v", plain)
	}
	if plain.Type == nil || *plain.Type != "Лек" {
		t.Fatalf("type = %v", plain.Type)
	}
}

func TestTeacherCellKeepsOnlyTheFirstEntry(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockTeacherShapeHTML))
	if err != nil {
		t.Fatal(err)
	}
	teachers, err := NewTeacherParser(doc).Run()
	if err != nil {
		t.Fatal(err)
	}

	entry := teachers["Агеенкова Д. Д."]
	saturday := entry.Days[1].Lessons
	if len(saturday) != 2 {
		t.Fatalf("one slot per row, saturday lessons = %d", len(saturday))
	}

	second := saturday[1]
	if second == nil || second.Subgroup == nil || *second.Subgroup != 1 {
		t.Fatalf("first entry subgroup = %#v", second)
	}
	if second.Group != "98" || second.Lesson != "Инструмент ПО" {
		t.Fatalf("the old parser reads only the first entry of a cell: group %q lesson %q", second.Group, second.Lesson)
	}
	if second.Cabinet == nil || *second.Cabinet != "3-111 (к)3-113 (к)" {
		t.Fatalf("the cabinet is the raw cell text: %v", second.Cabinet)
	}
}

func TestGroupSinglePrefixedLessonIsASubgroupArray(t *testing.T) {
	group := parseShapeGroup(t)

	monday := model.AsArray(group.Days[0].Lessons[0])
	if monday == nil {
		t.Fatalf("a numbered entry is a subgroup array even alone, got %#v", group.Days[0].Lessons[0])
	}
	if len(monday) != 1 {
		t.Fatalf("subgroups = %d", len(monday))
	}
	if monday[0].Subgroup == nil || *monday[0].Subgroup != 2 {
		t.Fatalf("subgroup = %v", monday[0].Subgroup)
	}
	if monday[0].Lesson != "Основы инж гр" {
		t.Fatalf("the number is stripped from the subject, got %q", monday[0].Lesson)
	}
	if monday[0].Cabinet == nil || *monday[0].Cabinet != "3-212 (к)" {
		t.Fatalf("cabinet = %v", monday[0].Cabinet)
	}
}

func TestGroupSingleLessonKeepsTheWholeSubject(t *testing.T) {
	group := parseShapeGroup(t)

	single := model.AsSingle(group.Days[1].Lessons[0])
	if single == nil {
		t.Fatalf("plain entry stays a single lesson, got %#v", group.Days[1].Lessons[0])
	}
	if single.Lesson != "Материалы ЭТех" {
		t.Fatalf("the old parser only maps exact subject names through the csv, got %q", single.Lesson)
	}
	if single.Teacher == nil || *single.Teacher != "Самарская Н. В." {
		t.Fatalf("teacher = %v", single.Teacher)
	}
	if single.Cabinet == nil || *single.Cabinet != "3-113" {
		t.Fatalf("cabinet = %v", single.Cabinet)
	}
}

func TestGroupSubgroupCabinetsFollowTheOldRules(t *testing.T) {
	group := parseShapeGroup(t)

	perSubgroup := model.AsArray(group.Days[0].Lessons[1])
	if len(perSubgroup) != 2 {
		t.Fatalf("subgroups = %d", len(perSubgroup))
	}
	if perSubgroup[0].Cabinet == nil || *perSubgroup[0].Cabinet != "2-109" {
		t.Fatalf("first cabinet = %v", perSubgroup[0].Cabinet)
	}
	if perSubgroup[1].Cabinet == nil || *perSubgroup[1].Cabinet != "3-202 (к)" {
		t.Fatalf("second cabinet = %v", perSubgroup[1].Cabinet)
	}

	shared := model.AsArray(group.Days[1].Lessons[1])
	if len(shared) != 2 {
		t.Fatalf("subgroups = %d", len(shared))
	}
	for i, part := range shared {
		if part.Cabinet == nil || *part.Cabinet != "3-212 (к)" {
			t.Fatalf("a single cabinet is shared, subgroup %d got %v", i, part.Cabinet)
		}
	}

	byNumber := model.AsArray(group.Days[0].Lessons[2])
	if len(byNumber) != 2 {
		t.Fatalf("subgroups = %d", len(byNumber))
	}
	if byNumber[0].Subgroup == nil || *byNumber[0].Subgroup != 2 || *byNumber[1].Subgroup != 3 {
		t.Fatalf("subgroups = %v %v", byNumber[0].Subgroup, byNumber[1].Subgroup)
	}
	if byNumber[0].Cabinet == nil || *byNumber[0].Cabinet != "б" {
		t.Fatalf("more cabinets than subgroups are indexed by the subgroup number: %v", byNumber[0].Cabinet)
	}
	if byNumber[1].Cabinet == nil || *byNumber[1].Cabinet != "в" {
		t.Fatalf("more cabinets than subgroups are indexed by the subgroup number: %v", byNumber[1].Cabinet)
	}
}

func parseShapeGroup(t *testing.T) *model.Group {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(mockGroupShapeHTML))
	if err != nil {
		t.Fatal(err)
	}
	groups, err := NewGroupParser(doc).Run()
	if err != nil {
		t.Fatal(err)
	}

	group := groups["100"]
	if group == nil {
		t.Fatal("group 100 is missing")
	}
	if len(group.Days) != 2 {
		t.Fatalf("days = %d", len(group.Days))
	}
	return group
}
