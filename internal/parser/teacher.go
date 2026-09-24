package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var teacherNameRe = regexp.MustCompile(`(?i)^Преподаватель\s*[-–—:]?\s*(.+)$`)
var teacherLooseRe = regexp.MustCompile(`(?i)Преподаватель\s*[-–—:]?\s*(.+)$`)
var typeRe = regexp.MustCompile(`\(([^)]+)\)`)
var typeOnlyRe = regexp.MustCompile(`^\s*\([^()]*\)\s*$`)
var subgroupRe = regexp.MustCompile(`^(\d+)\.\s*(.+)`)

type TeacherParser struct {
	doc    *goquery.Document
	report Report
}

func NewTeacherParser(doc *goquery.Document) *TeacherParser {
	return &TeacherParser{doc: doc}
}

func (p *TeacherParser) ContentHash() string {
	return hashDocument(p.doc)
}

func (p *TeacherParser) Report() Report {
	return p.report
}

func (p *TeacherParser) Run() (model.Teachers, error) {
	teachers, report := p.Parse()
	p.report = report
	return teachers, nil
}

func (p *TeacherParser) Parse() (model.Teachers, Report) {
	teachers := make(model.Teachers)
	builder := newReport(SourceTeachers, "")

	tables := scopedTables(p.doc, builder)
	builder.probe("table", "timetable tables", tables.Length(), true)

	labelled := 0
	blocks := 0
	skipped := 0
	withDays := 0
	withLessons := 0

	eachTable(tables, func(table *goquery.Selection) {
		match, ok := tableLabel(table, p.doc, teacherLabel)
		if !ok {
			return
		}
		labelled++

		teacher := p.parseTable(table, match.Value)
		if teacher == nil {
			skipped++
			return
		}
		if match.Loose {
			builder.fallback("teacher label without the 'Преподаватель -' prefix: " + match.Value)
		}

		blocks++
		withDays++
		if teacherHasLessons(teacher) {
			withLessons++
		}
		if existing, ok := teachers[match.Value]; ok {
			teachers[match.Value] = mergeTeacherDays(existing, teacher)
			return
		}
		teachers[match.Value] = teacher
	})

	builder.probe("heading: Преподаватель - <ФИО>", "teacher headings", labelled, true)
	builder.probe("th[colspan] with a date", "teachers with day columns", withDays, true)
	builder.probe("td lesson cells", "teachers with at least one lesson", withLessons, false)
	builder.probe("th[colspan] with dd.MM.yyyy", "day columns carrying a parseable date", teacherDatedDays(teachers), true)
	builder.probe("table with a day header", "timetable blocks across every published week", blocks, true)

	if skipped > 0 {
		builder.warn("%d tables had no readable day columns", skipped)
	}
	if withDays > 0 && withLessons == 0 {
		builder.warn("day columns were found, but no lesson cell produced a subject")
	}

	return teachers, builder.done(len(teachers))
}

func teacherDatedDays(teachers model.Teachers) int {
	dated := 0
	for _, teacher := range teachers {
		for _, day := range teacher.Days {
			if extractDayString(day.Day) != "" {
				dated++
			}
		}
	}
	return dated
}

func teacherLabel(text string) labelMatch {
	if match := teacherNameRe.FindStringSubmatch(text); match != nil {
		if name := trimSeparators(match[1]); name != "" {
			return labelMatch{Value: name, OK: true}
		}
	}
	if match := teacherLooseRe.FindStringSubmatch(text); match != nil {
		if name := trimSeparators(match[1]); name != "" {
			return labelMatch{Value: name, Loose: true, OK: true}
		}
	}
	return labelMatch{}
}

func (p *TeacherParser) parseTable(table *goquery.Selection, teacherName string) *model.Teacher {
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return nil
	}

	headerRow := findHeaderRow(rows)
	if headerRow == nil {
		return nil
	}

	columns := buildDayColumns(headerRow, true)
	if len(columns) == 0 {
		return nil
	}

	days := make([]model.TeacherDay, len(columns))
	for i, column := range columns {
		days[i] = model.TeacherDay{
			Day:     column.Day,
			Lessons: make([]model.TeacherLesson, 0),
		}
	}

	headerIdx := headerRowIndex(headerRow, rows)
	rows.Each(func(i int, row *goquery.Selection) {
		if i <= headerIdx || isHeaderRow(rows, row) {
			return
		}
		parseTeacherGridRow(row, columns, days)
	})

	for i := range days {
		mergeTeacherElectives(&days[i].Lessons)
		clearEndingTeacherNulls(&days[i].Lessons)
	}

	return &model.Teacher{
		Teacher: teacherName,
		Days:    days,
	}
}

func mergeTeacherElectives(lessons *[]model.TeacherLesson) {
	all := *lessons
	for i := 0; i < len(all); i++ {
		lesson := all[i]
		if lesson == nil || !isElective(lesson.Type) || lesson.Comment != nil {
			continue
		}

		similarIndex := -1
		for j := len(all) - 1; j > i; j-- {
			other := all[j]
			if other == nil {
				continue
			}
			if sameString(lesson.Type, other.Type) && lesson.Lesson == other.Lesson &&
				lesson.Group == other.Group && sameInt(lesson.Subgroup, other.Subgroup) {
				similarIndex = j
				break
			}
		}

		if similarIndex >= 0 {
			comment := twoHoursComment
			lesson.Comment = &comment
			all[similarIndex] = nil
		}
	}
}

func sameInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func teacherHasLessons(teacher *model.Teacher) bool {
	for _, day := range teacher.Days {
		if len(day.Lessons) > 0 {
			return true
		}
	}
	return false
}

func parseTeacherGridRow(row *goquery.Selection, columns []DayColumn, days []model.TeacherDay) {
	cells, _, _ := rowCellIndexes(row)

	for i, column := range columns {
		lessonCell, ok := cells[column.LessonCol]
		if !ok {
			return
		}

		cabinetCell := lessonCell
		if column.HasCabinet {
			if cell, exists := cells[column.CabinetCol]; exists {
				cabinetCell = cell
			}
		}

		days[i].Lessons = append(days[i].Lessons, parseTeacherLessonCell(lessonCell, cabinetCell, column.HasCabinet))
	}
}

func parseTeacherLessonCell(lessonCell, cabinetCell *goquery.Selection, hasCabinet bool) model.TeacherLesson {
	lessonText := cleanCellText(lessonCell)
	if lessonText == "" || lessonText == "-" || lessonText == "\u2014" {
		return nil
	}

	cabinet := ""
	if hasCabinet {
		cabinet = removeDashes(cleanCellText(cabinetCell))
	}

	data := textNodes(lessonCell)
	if len(data) == 0 {
		return nil
	}

	return buildTeacherEntry(data[0], dataAt(data, 1), cabinet)
}

func dataAt(data []string, index int) string {
	if index < len(data) {
		return data[index]
	}
	return ""
}

func buildTeacherEntry(nameLine, typeLine, cabinet string) model.TeacherLesson {
	lessonType := ""
	if match := typeRe.FindStringSubmatch(typeLine); match != nil {
		lessonType = match[1]
	}

	groupPart := nameLine
	lesson := ""
	if parts := strings.SplitN(nameLine, "-", 3); len(parts) > 1 {
		groupPart = parts[0]
		lesson = parts[1]
	}

	group := strings.Join(strings.Fields(groupPart), "")
	var subgroup *int
	if parts := strings.SplitN(group, ".", 2); len(parts) == 2 {
		if number, err := strconv.Atoi(parts[0]); err == nil {
			subgroup = &number
		}
		group = parts[1]
	}

	return &model.TeacherLessonExplain{
		Lesson:   shortenSubjectName(lesson),
		Type:     ptrString(lessonType),
		Subgroup: subgroup,
		Group:    group,
		Cabinet:  ptrString(cabinet),
	}
}

func clearEndingTeacherNulls(lessons *[]model.TeacherLesson) {
	for len(*lessons) > 0 && (*lessons)[len(*lessons)-1] == nil {
		*lessons = (*lessons)[:len(*lessons)-1]
	}
}
