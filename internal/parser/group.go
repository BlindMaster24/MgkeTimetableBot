package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var groupNumberRe = regexp.MustCompile(`^Группа\s*-\s*(.+)$`)
var subgroupPrefixRe = regexp.MustCompile(`^(\d+)\.`)
var typeInParensRe = regexp.MustCompile(`^\(([^)]+)\)$`)

type GroupParser struct {
	doc *goquery.Document
}

func NewGroupParser(doc *goquery.Document) *GroupParser {
	return &GroupParser{doc: doc}
}

func (p *GroupParser) ContentHash() string {
	return hashDocument(p.doc)
}

func (p *GroupParser) Run() (model.Groups, error) {
	groups := make(model.Groups)

	tables := findContent(p.doc).Find("table")

	tables.Each(func(_ int, table *goquery.Selection) {
		h2 := findPreviousH2(table)
		if h2 == nil {
			return
		}

		label := strings.TrimSpace(h2.Text())
		match := groupNumberRe.FindStringSubmatch(label)
		if match == nil {
			return
		}

		groupNum := strings.TrimSpace(match[1])
		groupNum = strings.TrimSuffix(groupNum, "*")

		if groupNum == "" {
			return
		}

		group := p.parseTable(table, groupNum)
		if group != nil {
			groups[groupNum] = group
		}
	})

	return groups, nil
}

func findPreviousH2(table *goquery.Selection) *goquery.Selection {
	prev := table.Prev()
	for prev.Length() > 0 {
		tag := goquery.NodeName(prev)
		if tag == "h2" {
			return prev
		}
		if tag == "table" {
			return nil
		}
		prev = prev.Prev()
	}
	return nil
}

func (p *GroupParser) parseTable(table *goquery.Selection, groupNum string) *model.Group {
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return nil
	}

	firstRow := rows.First()
	days := parseDayHeaders(firstRow)
	if len(days) == 0 {
		return nil
	}

	for i := range days {
		days[i].Lessons = make([]model.GroupLesson, 0)
	}

	rows.Each(func(i int, row *goquery.Selection) {
		if i <= 1 {
			return
		}
		p.parseLessonRow(row, days)
	})

	for i := range days {
		clearEndingNulls(&days[i].Lessons)
	}

	return &model.Group{
		Group: groupNum,
		Days:  days,
	}
}

func parseDayHeaders(row *goquery.Selection) []model.GroupDay {
	cells := row.Find("th")
	var days []model.GroupDay

	cells.Each(func(_ int, cell *goquery.Selection) {
		text := strings.TrimSpace(cell.Text())
		if text == "" {
			return
		}

		colspan, _ := cell.Attr("colspan")
		if colspan == "2" {
			days = append(days, model.GroupDay{
				Day: text,
			})
		}
	})

	return days
}

func (p *GroupParser) parseLessonRow(row *goquery.Selection, days []model.GroupDay) {
	cells := row.Find("td, th")
	totalCells := cells.Length()
	if totalCells < 2 {
		return
	}

	numDayPairs := (totalCells - 1) / 2

	for dayIdx := 0; dayIdx < numDayPairs && dayIdx < len(days); dayIdx++ {
		lessonCellIdx := 1 + dayIdx*2
		cabinetCellIdx := 2 + dayIdx*2

		if lessonCellIdx >= totalCells {
			break
		}

		lessonCell := cells.Eq(lessonCellIdx)
		cabinetCell := cells.Eq(cabinetCellIdx)

		if cabinetCellIdx >= totalCells {
			cabinetCell = lessonCell
		}

		lesson := parseGridLessonCell(lessonCell, cabinetCell)
		days[dayIdx].Lessons = append(days[dayIdx].Lessons, lesson)
	}
}

func parseGridLessonCell(lessonCell, cabinetCell *goquery.Selection) model.GroupLesson {
	lessonText := cleanCellText(lessonCell)
	cabinetText := cleanCellText(cabinetCell)

	if lessonText == "" || lessonText == "\u00a0" || lessonText == "-" || lessonText == "\u2014" {
		return nil
	}

	cabinetText = removeDashes(cabinetText)

	lessonLines := splitCellLines(lessonCell)
	cabLines := splitCellLines(cabinetCell)

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
		typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[1]))
		if typeMatch != nil {
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
			if m := subgroupPrefixRe.FindStringSubmatch(line); m != nil {
				subgroupNum = int(m[1][0] - '0')
				line = strings.TrimSpace(line[len(m[0]):])
			}
			name = line
		}
		if len(chunk) >= 2 {
			typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[1]))
			if typeMatch != nil {
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

		n := subgroupNum
		result = append(result, &model.GroupLessonExplain{
			Subgroup: &n,
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
	text = strings.TrimSpace(text)
	return text
}

func removeDashes(text string) string {
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`^[-—\s]+$`).ReplaceAllString(text, "")
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
		"Материалы ЭТех":      "МатЭТех",
		"Мат в професс":       "МатвПрофесс",
		"Основы инж гр":       "ОснИнжГр",
		"Физ химия":           "ФизХим",
		"Осн элек и микр":     "ОснЭлекМикр",
		"Ин Яз":               "ИнЯз",
		"ЭлИз":                "ЭлИз",
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
	if m := typeInParensRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}
