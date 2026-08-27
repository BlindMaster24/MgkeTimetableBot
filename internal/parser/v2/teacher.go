package v2

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type TeacherParserV2 struct {
	doc *goquery.Document
}

func NewTeacherParserV2(doc *goquery.Document) *TeacherParserV2 {
	return &TeacherParserV2{doc: doc}
}

func (p *TeacherParserV2) ContentHash() string {
	text := p.doc.Text()
	h := [32]byte{}
	copy(h[:], []byte(text))
	return fmt.Sprintf("%x", h)
}

func (p *TeacherParserV2) Run() (model.Teachers, error) {
	teachers := make(model.Teachers)
	tables := p.findTables()

	for _, table := range tables {
		label := p.findHeading(table, "преподаватель")
		if label == "" {
			continue
		}

		parsed := p.parseTable(table, label)
		if parsed == nil {
			continue
		}

		existing := teachers[parsed.Teacher]
		if existing != nil {
			existing.Days = mergeTeacherDays(parsed.Days, existing.Days)
			existing.Teacher = parsed.Teacher
		} else {
			teachers[parsed.Teacher] = &model.Teacher{
				Teacher: parsed.Teacher,
				Days:    parsed.Days,
			}
		}
	}

	clearSundaysTeachers(teachers)

	for _, t := range teachers {
		for i := range t.Days {
			postProcessTeacherDay(&t.Days[i])
		}
	}

	return teachers, nil
}

type parsedTeacherTable struct {
	Teacher string
	Days    []model.TeacherDay
}

func (p *TeacherParserV2) parseTable(table *goquery.Selection, label string) *parsedTeacherTable {
	teacher := parseTeacherLabel(label)
	if teacher == "" {
		return nil
	}

	grid := BuildTableGrid(table)
	headerRowIndex := FindHeaderRowIndex(grid, 5, 5)
	if headerRowIndex < 0 {
		return nil
	}
	ranges := GetDayRangesFromGrid(grid, headerRowIndex)
	if len(ranges) < 5 {
		return nil
	}

	days := make([]model.TeacherDay, len(ranges))
	for i, r := range ranges {
		days[i] = model.TeacherDay{Day: r.Day, Lessons: []model.TeacherLesson{}}
	}

	for rowIndex := headerRowIndex + 1; rowIndex < grid.Height; rowIndex++ {
		row := grid.Grid[rowIndex]
		if row == nil || len(row) == 0 {
			continue
		}

		numberCell := row[0]
		if numberCell == nil || numberCell.Row != rowIndex {
			continue
		}

		lessonNumber := parseLessonNumber(numberCell.Cell.Text())
		if lessonNumber == 0 {
			continue
		}

		for i, r := range ranges {
			lessonCell := getCellSafe(row, rowIndex, r.Start)
			var cabinetCell *goquery.Selection
			if r.Span > 1 {
				cabinetCell = getCellSafe(row, rowIndex, r.Start+1)
			}

			lesson := parseTeacherLesson(lessonCell, cabinetCell)
			assignTeacherLesson(&days[i], lessonNumber, lesson)
		}
	}

	return &parsedTeacherTable{
		Teacher: teacher,
		Days:    days,
	}
}

func (p *TeacherParserV2) findTables() []*goquery.Selection {
	var tables []*goquery.Selection
	p.doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		grid := BuildTableGrid(table)
		hri := FindHeaderRowIndex(grid, 5, 5)
		if hri < 0 {
			return
		}
		ranges := GetDayRangesFromGrid(grid, hri)
		if len(ranges) < 5 {
			return
		}
		tables = append(tables, table)
	})
	return tables
}

func (p *TeacherParserV2) findHeading(table *goquery.Selection, keyword string) string {
	prev := table.Prev()
	depth := 0
	for prev.Length() > 0 && depth < 12 {
		tag := goquery.NodeName(prev)
		if tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" {
			text := cleanText(prev.Text())
			if text != "" && strings.Contains(strings.ToLower(text), keyword) {
				return text
			}
		}
		if tag == "table" {
			break
		}
		prev = prev.Prev()
		depth++
	}
	return ""
}

func parseTeacherLabel(label string) string {
	text := cleanText(label)
	if text == "" || !strings.Contains(strings.ToLower(text), "преподаватель") {
		return ""
	}
	parts := strings.SplitN(text, "-", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseTeacherLesson(lessonCell, cabinetCell *goquery.Selection) model.TeacherLesson {
	lines := extractLines(lessonCell)
	if len(lines) == 0 {
		return nil
	}

	entries := parseTeacherEntries(lines)
	if len(entries) == 0 {
		return nil
	}

	cabinets := extractLines(cabinetCell)
	applyTeacherCabinets(entries, cabinets)

	return entries[0]
}

func parseTeacherEntries(lines []string) []*model.TeacherLessonExplain {
	var entries []*model.TeacherLessonExplain
	var subgroup *int
	var group, lesson, typ string

	push := func() {
		if lesson == "" || group == "" {
			subgroup = nil
			group = ""
			lesson = ""
			typ = ""
			return
		}
		e := &model.TeacherLessonExplain{
			Group:  group,
			Lesson: lesson,
		}
		if subgroup != nil {
			e.Subgroup = subgroup
		}
		if typ != "" {
			e.Type = &typ
		}
		entries = append(entries, e)
		subgroup = nil
		group = ""
		lesson = ""
		typ = ""
	}

	for _, raw := range lines {
		if c := parseTeacherCombinedLine(raw); c != nil {
			push()
			entries = append(entries, c)
			continue
		}
		if t := parseTypeLine(raw); t != nil && lesson != "" {
			typ = *t
			continue
		}
		if lesson == "" || group == "" {
			if p := parseTeacherGroupLessonLine(raw); p != nil {
				lesson = p.Lesson
				group = p.Group
				subgroup = p.Subgroup
			}
			continue
		}
		push()
		if p := parseTeacherGroupLessonLine(raw); p != nil {
			lesson = p.Lesson
			group = p.Group
			subgroup = p.Subgroup
		}
	}
	push()
	return entries
}

var teacherCombinedRe = regexp.MustCompile(`^(?:(\d+)\s*[.)]\s*)?(.+?)\s*-\s*(.+?)(?:\s*\(([^)]+)\))?\s*$`)

func parseTeacherCombinedLine(line string) *model.TeacherLessonExplain {
	match := teacherCombinedRe.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	var subgroup *int
	if match[1] != "" {
		v := 0
		fmt.Sscanf(match[1], "%d", &v)
		subgroup = &v
	}

	groupPart := strings.TrimSpace(match[2])
	gi := parseGroupPart(groupPart)

	if subgroup == nil {
		subgroup = gi.Subgroup
	}

	group := gi.Group
	l := strings.TrimSpace(match[3])
	var typ *string
	if match[4] != "" {
		t := strings.TrimSpace(match[4])
		typ = &t
	}

	return &model.TeacherLessonExplain{
		Lesson:   l,
		Type:     typ,
		Subgroup: subgroup,
		Group:    group,
	}
}

type parsedTeacherGroupLessonLine struct {
	Lesson   string
	Group    string
	Subgroup *int
}

var teacherGroupLessonRe = regexp.MustCompile(`^(.+?)\s*-\s*(.+)$`)

func parseTeacherGroupLessonLine(line string) *parsedTeacherGroupLessonLine {
	normalized := cleanText(line)
	if normalized == "" {
		return nil
	}
	parts := strings.SplitN(normalized, "-", 2)
	if len(parts) < 2 {
		return nil
	}
	gi := parseGroupPart(strings.TrimSpace(parts[0]))
	return &parsedTeacherGroupLessonLine{
		Lesson:   strings.TrimSpace(parts[1]),
		Group:    gi.Group,
		Subgroup: gi.Subgroup,
	}
}

type groupInfo struct {
	Group    string
	Subgroup *int
}

func parseGroupPart(value string) groupInfo {
	cleaned := strings.ReplaceAll(value, " ", "")
	re := regexp.MustCompile(`^(?:(\d+)\.)?(.+)$`)
	match := re.FindStringSubmatch(cleaned)
	if match == nil {
		return groupInfo{Group: cleaned}
	}
	var subgroup *int
	if match[1] != "" {
		v := 0
		fmt.Sscanf(match[1], "%d", &v)
		subgroup = &v
	}
	return groupInfo{Group: strings.TrimSpace(match[2]), Subgroup: subgroup}
}

func applyTeacherCabinets(entries []*model.TeacherLessonExplain, cabinets []string) {
	if len(entries) == 0 {
		return
	}
	normalized := make([]string, len(cabinets))
	copy(normalized, cabinets)
	if len(normalized) == 0 {
		normalized = []string{""}
	}
	hasAny := false
	for _, v := range normalized {
		if v != "" && v != "-" {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return
	}
	if len(normalized) == 1 {
		val := normalized[0]
		for _, e := range entries {
			e.Cabinet = &val
		}
		return
	}
	for i, e := range entries {
		idx := i
		if idx >= len(normalized) {
			idx = len(normalized) - 1
		}
		val := normalized[idx]
		e.Cabinet = &val
	}
}

func assignTeacherLesson(day *model.TeacherDay, lessonNumber int, lesson model.TeacherLesson) {
	index := lessonNumber - 1
	for len(day.Lessons) < index {
		day.Lessons = append(day.Lessons, nil)
	}
	if len(day.Lessons) == index {
		day.Lessons = append(day.Lessons, lesson)
	} else {
		day.Lessons[index] = lesson
	}
}

func postProcessTeacherDay(day *model.TeacherDay) {
	for i := 0; i < len(day.Lessons); i++ {
		lesson := day.Lessons[i]
		if lesson == nil {
			continue
		}
		if lesson.Type == nil || *lesson.Type != "ф-в" || lesson.Comment != nil {
			continue
		}
		similarIdx := -1
		for j := len(day.Lessons) - 1; j > i; j-- {
			fl := day.Lessons[j]
			if fl == nil {
				continue
			}
			if lesson.Type == fl.Type && lesson.Lesson == fl.Lesson && lesson.Group == fl.Group && lesson.Subgroup == fl.Subgroup {
				similarIdx = j
				break
			}
		}
		if similarIdx >= 0 {
			comment := "2 часа"
			lesson.Comment = &comment
			day.Lessons[similarIdx] = nil
		}
	}
	for len(day.Lessons) > 0 && day.Lessons[len(day.Lessons)-1] == nil {
		day.Lessons = day.Lessons[:len(day.Lessons)-1]
	}
}

func mergeTeacherDays(newDays, oldDays []model.TeacherDay) []model.TeacherDay {
	days := make(map[string]*model.TeacherDay)
	for i := range oldDays {
		days[oldDays[i].Day] = &oldDays[i]
	}
	for i := range newDays {
		days[newDays[i].Day] = &newDays[i]
	}
	var result []model.TeacherDay
	for _, d := range days {
		result = append(result, *d)
	}
	return result
}

func clearSundaysTeachers(teachers model.Teachers) {
	for _, t := range teachers {
		var filtered []model.TeacherDay
		for _, d := range t.Days {
			parsed, err := time.Parse("02.01.2006", d.Day)
			if err == nil && parsed.Weekday() != time.Sunday {
				filtered = append(filtered, d)
			}
		}
		t.Days = filtered
	}
}
