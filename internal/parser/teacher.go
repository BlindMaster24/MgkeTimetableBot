package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var teacherNameRe = regexp.MustCompile(`^Преподаватель\s*-\s*(.+)$`)
var typeRe = regexp.MustCompile(`\(([^)]+)\)`)
var subgroupRe = regexp.MustCompile(`^(\d+)\.\s*(.+)`)

type TeacherParser struct {
	doc *goquery.Document
}

func NewTeacherParser(doc *goquery.Document) *TeacherParser {
	return &TeacherParser{doc: doc}
}

func (p *TeacherParser) ContentHash() string {
	return hashDocument(p.doc)
}

func (p *TeacherParser) Run() (model.Teachers, error) {
	teachers := make(model.Teachers)

	tables := findContent(p.doc).Find("table")

	tables.Each(func(_ int, table *goquery.Selection) {
		h2 := findPreviousH2(table)
		if h2 == nil {
			return
		}

		label := strings.TrimSpace(h2.Text())
		match := teacherNameRe.FindStringSubmatch(label)
		if match == nil {
			return
		}

		teacherName := strings.TrimSpace(match[1])
		if teacherName == "" {
			return
		}

		teacher := p.parseTable(table, teacherName)
		if teacher != nil {
			teachers[teacherName] = teacher
		}
	})

	return teachers, nil
}

func (p *TeacherParser) parseTable(table *goquery.Selection, teacherName string) *model.Teacher {
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return nil
	}

	firstRow := rows.First()
	days := parseTeacherDayHeaders(firstRow)
	if len(days) == 0 {
		return nil
	}

	for i := range days {
		days[i].Lessons = make([]model.TeacherLesson, 0)
	}

	rows.Each(func(i int, row *goquery.Selection) {
		if i <= 1 {
			return
		}
		p.parseTeacherLessonRow(row, days)
	})

	for i := range days {
		clearEndingTeacherNulls(&days[i].Lessons)
	}

	return &model.Teacher{
		Teacher: teacherName,
		Days:    days,
	}
}

func parseTeacherDayHeaders(row *goquery.Selection) []model.TeacherDay {
	cells := row.Find("th")
	var days []model.TeacherDay

	cells.Each(func(_ int, cell *goquery.Selection) {
		text := strings.TrimSpace(cell.Text())
		if text == "" {
			return
		}

		colspan, _ := cell.Attr("colspan")
		if colspan == "2" {
			days = append(days, model.TeacherDay{
				Day: text,
			})
		}
	})

	return days
}

func (p *TeacherParser) parseTeacherLessonRow(row *goquery.Selection, days []model.TeacherDay) {
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

		lessons := parseTeacherLessonCell(lessonCell, cabinetCell)
		days[dayIdx].Lessons = append(days[dayIdx].Lessons, lessons...)
	}
}

func parseTeacherLessonCell(lessonCell, cabinetCell *goquery.Selection) []model.TeacherLesson {
	lessonText := cleanCellText(lessonCell)

	if lessonText == "" || lessonText == "-" || lessonText == "\u2014" {
		return nil
	}

	cabinetText := cleanCellText(cabinetCell)
	cabinetText = removeDashes(cabinetText)

	lines := splitCellLines(lessonCell)
	cabLines := splitCellLines(cabinetCell)

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
		typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[2]))
		if typeMatch != nil {
			lessonType = typeMatch[1]
		} else {
			name = strings.TrimSpace(chunk[1]) + " " + strings.TrimSpace(chunk[2])
		}
	}

	if name == "" {
		return nil
	}

	return []model.TeacherLesson{{
		Group:    group,
		Lesson:   name,
		Type:     ptrString(lessonType),
		Cabinet:  ptrString(cabinet),
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
			if m := subgroupPrefixRe.FindStringSubmatch(line); m != nil {
				line = strings.TrimSpace(line[len(m[0]):])
			}
			group = line
		}
		if len(chunk) >= 2 {
			name = strings.TrimSpace(chunk[1])
		}
		if len(chunk) >= 3 {
			typeMatch := typeInParensRe.FindStringSubmatch(strings.TrimSpace(chunk[2]))
			if typeMatch != nil {
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
			Group:    group,
			Lesson:   name,
			Type:     ptrString(lessonType),
			Cabinet:  ptrString(cab),
		})
	}

	return result
}

func extractTeacherGroup(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "1.") || strings.HasPrefix(text, "2.") {
		parts := strings.SplitN(text, ".", 2)
		if len(parts) > 1 {
			text = strings.TrimSpace(parts[1])
		}
	}

	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if m := typeRe.FindStringSubmatch(firstLine); m == nil {
			parts := strings.Fields(firstLine)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}

	return ""
}

func extractTeacherLessonName(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if typeRe.MatchString(line) {
			parts := typeRe.Split(line, 2)
			name := strings.TrimSpace(parts[0])

			if strings.HasPrefix(name, "1.") || strings.HasPrefix(name, "2.") {
				dotIdx := strings.Index(name, ".")
				name = strings.TrimSpace(name[dotIdx+1:])
			}

			parts2 := strings.Fields(name)
			if len(parts2) > 1 {
				return strings.Join(parts2[1:], " ")
			}
			return name
		}
	}

	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		parts := strings.Fields(first)
		if len(parts) > 1 {
			return strings.Join(parts[1:], " ")
		}
	}

	return ""
}

func clearEndingTeacherNulls(lessons *[]model.TeacherLesson) {
	for len(*lessons) > 0 && (*lessons)[len(*lessons)-1] == nil {
		*lessons = (*lessons)[:len(*lessons)-1]
	}
}
