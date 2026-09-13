package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var teacherNameRe = regexp.MustCompile(`(?i)^Преподаватель\s*[-–—:]?\s*(.+)$`)
var teacherLooseRe = regexp.MustCompile(`(?i)Преподаватель\s*[-–—:]?\s*(.+)$`)
var typeRe = regexp.MustCompile(`\(([^)]+)\)`)
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
	skipped := 0
	withDays := 0
	withLessons := 0

	tables.Each(func(_ int, table *goquery.Selection) {
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

		withDays++
		if teacherHasLessons(teacher) {
			withLessons++
		}
		teachers[match.Value] = teacher
	})

	builder.probe("heading: Преподаватель - <ФИО>", "teacher headings", labelled, true)
	builder.probe("th[colspan] with a date", "teachers with day columns", withDays, true)
	builder.probe("td lesson cells", "teachers with at least one lesson", withLessons, false)

	if skipped > 0 {
		builder.warn("%d tables had no readable day columns", skipped)
	}
	if withDays > 0 && withLessons == 0 {
		builder.warn("day columns were found, but no lesson cell produced a subject")
	}

	return teachers, builder.done(len(teachers))
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
		clearEndingTeacherNulls(&days[i].Lessons)
	}

	return &model.Teacher{
		Teacher: teacherName,
		Days:    days,
	}
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

		days[i].Lessons = append(days[i].Lessons, parseTeacherLessonCell(lessonCell, cabinetCell, column.HasCabinet)...)
	}
}

func parseTeacherLessonCell(lessonCell, cabinetCell *goquery.Selection, hasCabinet bool) []model.TeacherLesson {
	lessonText := cleanCellText(lessonCell)
	if lessonText == "" || lessonText == "-" || lessonText == "\u2014" {
		return nil
	}

	cabinetText := ""
	if hasCabinet {
		cabinetText = removeDashes(cleanCellText(cabinetCell))
	}

	lines := splitCellLines(lessonCell)
	cabLines := splitCellLines(cabinetCell)

	if !hasCabinet {
		var inline string
		lines, inline = splitInlineCabinet(lines)
		cabinetText = inline
		cabLines = []string{inline}
		if len(lines) == 0 {
			return nil
		}
	}

	chunks := chunkLines(lines, 3)
	if len(chunks) == 0 {
		return nil
	}

	isSubgroup := len(chunks) > 1 || len(cabLines) > 1
	if !isSubgroup {
		for _, line := range lines {
			if subgroupPrefixRe.MatchString(strings.TrimSpace(line)) {
				isSubgroup = true
				break
			}
		}
	}

	if isSubgroup && len(chunks) > 1 {
		return buildTeacherSubgroups(chunks, cabLines)
	}

	return buildTeacherSingle(chunks, cabinetText)
}

func buildTeacherSingle(chunks [][]string, cabinet string) []model.TeacherLesson {
	if len(chunks) == 0 || len(chunks[0]) == 0 {
		return nil
	}

	chunk := chunks[0]
	group := ""
	name := ""
	lessonType := ""

	if len(chunk) >= 1 {
		group = strings.TrimSpace(chunk[0])
	}
	if len(chunk) >= 2 {
		name = strings.TrimSpace(chunk[1])
	}
	if len(chunk) >= 3 {
		if typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[2])); typeMatch != nil {
			lessonType = typeMatch[1]
		} else {
			name = strings.TrimSpace(chunk[1]) + " " + strings.TrimSpace(chunk[2])
		}
	}

	if name == "" {
		return nil
	}

	return []model.TeacherLesson{{
		Group:   group,
		Lesson:  name,
		Type:    ptrString(lessonType),
		Cabinet: ptrString(cabinet),
	}}
}

func buildTeacherSubgroups(chunks [][]string, cabLines []string) []model.TeacherLesson {
	var result []model.TeacherLesson

	for i, chunk := range chunks {
		group := ""
		name := ""
		lessonType := ""

		if len(chunk) >= 1 {
			line := strings.TrimSpace(chunk[0])
			if match := subgroupPrefixRe.FindStringSubmatch(line); match != nil {
				line = strings.TrimSpace(line[len(match[0]):])
			}
			group = line
		}
		if len(chunk) >= 2 {
			name = strings.TrimSpace(chunk[1])
		}
		if len(chunk) >= 3 {
			if typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[2])); typeMatch != nil {
				lessonType = typeMatch[1]
			}
		}

		cab := ""
		if i < len(cabLines) {
			cab = removeDashes(cabLines[i])
		}

		if name == "" {
			continue
		}

		result = append(result, &model.TeacherLessonExplain{
			Group:   group,
			Lesson:  name,
			Type:    ptrString(lessonType),
			Cabinet: ptrString(cab),
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func clearEndingTeacherNulls(lessons *[]model.TeacherLesson) {
	for len(*lessons) > 0 && (*lessons)[len(*lessons)-1] == nil {
		*lessons = (*lessons)[:len(*lessons)-1]
	}
}
