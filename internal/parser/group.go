package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var groupNumberRe = regexp.MustCompile(`(?i)^Группа\s*[-–—:]?\s*(.+)$`)
var groupLooseRe = regexp.MustCompile(`(?i)Группа\s*[-–—:]?\s*(.+)$`)
var subgroupPrefixRe = regexp.MustCompile(`^(\d+)\.`)
var typeInParensRe = regexp.MustCompile(`^\(([^)]+)\)$`)
var dashOnlyRe = regexp.MustCompile(`^[-—\s]+$`)

type GroupParser struct {
	doc    *goquery.Document
	report Report
}

func NewGroupParser(doc *goquery.Document) *GroupParser {
	return &GroupParser{doc: doc}
}

func (p *GroupParser) ContentHash() string {
	return hashDocument(p.doc)
}

func (p *GroupParser) Report() Report {
	return p.report
}

func (p *GroupParser) Run() (model.Groups, error) {
	groups, report := p.Parse()
	p.report = report
	return groups, nil
}

func (p *GroupParser) Parse() (model.Groups, Report) {
	groups := make(model.Groups)
	builder := newReport(SourceGroups, "")

	tables := scopedTables(p.doc, builder)
	builder.probe("table", "timetable tables", tables.Length(), true)

	labelled := 0
	skipped := 0
	withDays := 0
	withLessons := 0

	tables.Each(func(_ int, table *goquery.Selection) {
		match, ok := tableLabel(table, p.doc, groupLabel)
		if !ok {
			return
		}
		labelled++

		group := p.parseTable(table, match.Value)
		if group == nil {
			skipped++
			return
		}
		if match.Loose {
			builder.fallback("group label without the 'Группа -' prefix: " + match.Value)
		}

		withDays++
		if groupHasLessons(group) {
			withLessons++
		}
		groups[match.Value] = group
	})

	builder.probe("heading: Группа - <номер>", "group headings", labelled, true)
	builder.probe("th[colspan] with a date", "groups with day columns", withDays, true)
	builder.probe("td lesson cells", "groups with at least one lesson", withLessons, false)

	if skipped > 0 {
		builder.warn("%d tables had no readable day columns", skipped)
	}
	if withDays > 0 && withLessons == 0 {
		builder.warn("day columns were found, but no lesson cell produced a subject")
	}

	return groups, builder.done(len(groups))
}

func groupLabel(text string) labelMatch {
	if match := groupNumberRe.FindStringSubmatch(text); match != nil {
		if name := normalizeGroupName(match[1]); name != "" {
			return labelMatch{Value: name, OK: true}
		}
	}
	if match := groupLooseRe.FindStringSubmatch(text); match != nil {
		if name := normalizeGroupName(match[1]); name != "" {
			return labelMatch{Value: name, Loose: true, OK: true}
		}
	}
	return labelMatch{}
}

func normalizeGroupName(raw string) string {
	name := trimSeparators(strings.TrimSpace(raw))
	name = strings.TrimSpace(strings.TrimSuffix(name, "*"))
	if name == "" {
		return ""
	}
	if fields := strings.Fields(name); len(fields) > 1 {
		return fields[0]
	}
	return name
}

func (p *GroupParser) parseTable(table *goquery.Selection, groupNum string) *model.Group {
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

	days := make([]model.GroupDay, len(columns))
	for i, column := range columns {
		days[i] = model.GroupDay{
			Day:     column.Day,
			Lessons: make([]model.GroupLesson, 0),
		}
	}

	headerIdx := headerRowIndex(headerRow, rows)
	rows.Each(func(i int, row *goquery.Selection) {
		if i <= headerIdx || isHeaderRow(rows, row) {
			return
		}
		parseGridLessonRow(row, columns, days)
	})

	for i := range days {
		clearEndingNulls(&days[i].Lessons)
	}

	return &model.Group{
		Group: groupNum,
		Days:  days,
	}
}

func groupHasLessons(group *model.Group) bool {
	for _, day := range group.Days {
		if len(day.Lessons) > 0 {
			return true
		}
	}
	return false
}

func parseGridLessonRow(row *goquery.Selection, columns []DayColumn, days []model.GroupDay) {
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

		days[i].Lessons = append(days[i].Lessons, parseGridLessonCell(lessonCell, cabinetCell, column.HasCabinet))
	}
}

func parseGridLessonCell(lessonCell, cabinetCell *goquery.Selection, hasCabinet bool) model.GroupLesson {
	lessonText := cleanCellText(lessonCell)
	if lessonText == "" || lessonText == "\u00a0" || lessonText == "-" || lessonText == "\u2014" {
		return nil
	}

	cabinetText := ""
	if hasCabinet {
		cabinetText = removeDashes(cleanCellText(cabinetCell))
	}

	lessonLines := splitCellLines(lessonCell)
	cabLines := splitCellLines(cabinetCell)

	if !hasCabinet {
		var inline string
		lessonLines, inline = splitInlineCabinet(lessonLines)
		cabinetText = inline
		cabLines = []string{inline}
		if len(lessonLines) == 0 {
			return nil
		}
	}

	chunks := chunkLines(lessonLines, 3)
	cabChunks := chunkLines(cabLines, 1)

	isSubgroup := len(chunks) > 1
	if !isSubgroup {
		for _, line := range lessonLines {
			if subgroupPrefixRe.MatchString(strings.TrimSpace(line)) {
				isSubgroup = true
				break
			}
		}
	}

	if isSubgroup && len(chunks) > 1 {
		return buildSubgroups(chunks, cabChunks)
	}

	return buildSingleLesson(chunks, cabinetText)
}

func splitCellLines(cell *goquery.Selection) []string {
	cell.Find("br").ReplaceWithHtml("\n")
	text := cell.Text()
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func chunkLines(lines []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(lines); i += size {
		end := i + size
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, lines[i:end])
	}
	return chunks
}

func buildSingleLesson(chunks [][]string, cabinet string) model.GroupLesson {
	if len(chunks) == 0 || len(chunks[0]) == 0 {
		return nil
	}

	chunk := chunks[0]
	name := ""
	lessonType := ""
	teacher := ""

	if len(chunk) >= 1 {
		name = strings.TrimSpace(chunk[0])
	}
	if len(chunk) >= 2 {
		if typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[1])); typeMatch != nil {
			lessonType = typeMatch[1]
		}
	}
	if len(chunk) >= 3 {
		teacher = strings.TrimSpace(chunk[2])
	}

	name = strings.TrimPrefix(name, "1.")
	name = strings.TrimPrefix(name, "2.")
	name = strings.TrimSpace(name)
	name = shortenSubjectName(name)

	if name == "" {
		return nil
	}

	return &model.GroupLessonExplain{
		Lesson:  name,
		Type:    ptrString(lessonType),
		Teacher: ptrString(teacher),
		Cabinet: ptrString(cabinet),
	}
}

func buildSubgroups(chunks [][]string, cabChunks [][]string) model.GroupLesson {
	var result []*model.GroupLessonExplain

	for i, chunk := range chunks {
		subgroupNum := i + 1
		name := ""
		lessonType := ""
		teacher := ""

		if len(chunk) >= 1 {
			line := strings.TrimSpace(chunk[0])
			if match := subgroupPrefixRe.FindStringSubmatch(line); match != nil {
				subgroupNum = int(match[1][0] - '0')
				line = strings.TrimSpace(line[len(match[0]):])
			}
			name = line
		}
		if len(chunk) >= 2 {
			if typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[1])); typeMatch != nil {
				lessonType = typeMatch[1]
			}
		}
		if len(chunk) >= 3 {
			teacher = strings.TrimSpace(chunk[2])
		}

		name = shortenSubjectName(name)

		cab := ""
		if i < len(cabChunks) && len(cabChunks[i]) > 0 {
			cab = removeDashes(cabChunks[i][0])
		}

		if name == "" {
			continue
		}

		number := subgroupNum
		result = append(result, &model.GroupLessonExplain{
			Subgroup: &number,
			Lesson:   name,
			Type:     ptrString(lessonType),
			Teacher:  ptrString(teacher),
			Cabinet:  ptrString(cab),
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func cleanCellText(cell *goquery.Selection) string {
	text := cell.Text()
	text = strings.ReplaceAll(text, "\u00a0", "")
	text = strings.ReplaceAll(text, "&nbsp;", "")
	return strings.TrimSpace(text)
}

func removeDashes(text string) string {
	text = strings.TrimSpace(text)
	text = dashOnlyRe.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func clearEndingNulls(lessons *[]model.GroupLesson) {
	for len(*lessons) > 0 && (*lessons)[len(*lessons)-1] == nil {
		*lessons = (*lessons)[:len(*lessons)-1]
	}
}

func shortenSubjectName(name string) string {
	name = strings.TrimSpace(name)
	replacements := map[string]string{
		"Материалы ЭТех":       "МатЭТех",
		"Мат в професс":        "МатвПрофесс",
		"Основы инж гр":        "ОснИнжГр",
		"Физ химия":            "ФизХим",
		"Осн элек и микр":      "ОснЭлекМикр",
		"Ин Яз":                "ИнЯз",
		"ЭлИз":                 "ЭлИз",
		"Лабораторные занятия": "ЛабЗанятия",
	}
	for old, short := range replacements {
		if strings.Contains(name, old) {
			name = strings.Replace(name, old, short, 1)
		}
	}
	return name
}

func extractType(text string) string {
	if match := typeInParensRe.FindStringSubmatch(text); match != nil {
		return match[1]
	}
	return ""
}
