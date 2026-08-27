package v2

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type GroupParserV2 struct {
	doc *goquery.Document
}

func NewGroupParserV2(doc *goquery.Document) *GroupParserV2 {
	return &GroupParserV2{doc: doc}
}

func (p *GroupParserV2) ContentHash() string {
	text := p.doc.Text()
	h := [32]byte{}
	copy(h[:], []byte(text))
	return fmt.Sprintf("%x", h)
}

func (p *GroupParserV2) Run() (model.Groups, error) {
	groups := make(model.Groups)
	tables := p.findTables()

	for _, table := range tables {
		label := p.findHeading(table, "группа")
		if label == "" {
			continue
		}

		parsed := p.parseTable(table, label)
		if parsed == nil {
			continue
		}

		existing := groups[parsed.GroupNumber]
		if existing != nil {
			existing.Days = mergeGroupDays(parsed.Days, existing.Days)
			existing.Group = parsed.Group
		} else {
			groups[parsed.GroupNumber] = &model.Group{
				Group: parsed.Group,
				Days:  parsed.Days,
			}
		}
	}

	clearSundays(groups)

	for _, g := range groups {
		for i := range g.Days {
			postProcessGroupDay(&g.Days[i])
		}
	}

	return groups, nil
}

type parsedGroupTable struct {
	Group       string
	GroupNumber string
	Days        []model.GroupDay
}

func (p *GroupParserV2) parseTable(table *goquery.Selection, label string) *parsedGroupTable {
	group := parseGroupLabel(label)
	groupNumber := extractGroupNumber(group)
	if group == "" || groupNumber == "" {
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

	days := make([]model.GroupDay, len(ranges))
	for i, r := range ranges {
		days[i] = model.GroupDay{Day: r.Day, Lessons: []model.GroupLesson{}}
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

			lesson := parseGroupLesson(lessonCell, cabinetCell)
			assignGroupLesson(&days[i], lessonNumber, lesson)
		}
	}

	return &parsedGroupTable{
		Group:       group,
		GroupNumber: groupNumber,
		Days:        days,
	}
}

func (p *GroupParserV2) findTables() []*goquery.Selection {
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

func (p *GroupParserV2) findHeading(table *goquery.Selection, keyword string) string {
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

func parseGroupLabel(label string) string {
	text := cleanText(label)
	if text == "" || !strings.Contains(strings.ToLower(text), "группа") {
		return ""
	}
	parts := strings.SplitN(text, "-", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func extractGroupNumber(group string) string {
	re := regexp.MustCompile(`\d+`)
	return re.FindString(group)
}

func parseGroupLesson(lessonCell, cabinetCell *goquery.Selection) model.GroupLesson {
	lines := extractLines(lessonCell)
	if len(lines) == 0 {
		return nil
	}

	entries := parseGroupEntries(lines)
	if len(entries) == 0 {
		return nil
	}

	cabinets := extractLines(cabinetCell)
	applyCabinets(entries, cabinets)

	if len(entries) == 1 && entries[0].Subgroup == nil {
		return entries[0]
	}
	return entries
}

func parseGroupEntries(lines []string) []*model.GroupLessonExplain {
	var entries []*model.GroupLessonExplain
	var sub *int
	var lesson, typ, teacher string

	push := func() {
		if lesson == "" {
			sub = nil
			lesson = ""
			typ = ""
			teacher = ""
			return
		}
		e := &model.GroupLessonExplain{
			Subgroup: sub,
			Lesson:   lesson,
		}
		if typ != "" {
			e.Type = &typ
		}
		if teacher != "" {
			e.Teacher = &teacher
		}
		entries = append(entries, e)
		sub = nil
		lesson = ""
		typ = ""
		teacher = ""
	}

	for _, raw := range lines {
		if c := parseCombinedLine(raw); c != nil {
			push()
			entries = append(entries, c)
			continue
		}
		if t := parseTypeLine(raw); t != nil && lesson != "" {
			typ = *t
			continue
		}
		if lesson == "" {
			if p := parseLessonLine(raw); p != nil {
				lesson = p.Lesson
				sub = p.Subgroup
			}
			continue
		}
		if teacher == "" {
			teacher = raw
			continue
		}
		if isTeacherLine(raw) {
			teacher = teacher + ", " + raw
			continue
		}
		push()
		if p := parseLessonLine(raw); p != nil {
			lesson = p.Lesson
			sub = p.Subgroup
		}
	}
	push()
	return entries
}

var combinedRe = regexp.MustCompile(`^(?:(\d+)\s*[.)]\s*)?(.+?)\s*\(([^)]+)\)\s*(.+)?$`)

func parseCombinedLine(line string) *model.GroupLessonExplain {
	match := combinedRe.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	var sub *int
	if match[1] != "" {
		v := 0
		fmt.Sscanf(match[1], "%d", &v)
		sub = &v
	}
	l := strings.TrimSpace(match[2])
	t := strings.TrimSpace(match[3])
	te := cleanText(match[4])
	return &model.GroupLessonExplain{Subgroup: sub, Lesson: l, Type: &t, Teacher: &te}
}

func parseTypeLine(line string) *string {
	s := strings.TrimSpace(line)
	if len(s) > 2 && s[0] == '(' && s[len(s)-1] == ')' {
		v := s[1 : len(s)-1]
		return &v
	}
	return nil
}

var lessonLineRe = regexp.MustCompile(`^(?:(\d+)\s*[.)]\s*)?(.+)$`)

type parsedLessonLine struct {
	Lesson   string
	Subgroup *int
}

func parseLessonLine(line string) *parsedLessonLine {
	match := lessonLineRe.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	var sub *int
	if match[1] != "" {
		v := 0
		fmt.Sscanf(match[1], "%d", &v)
		sub = &v
	}
	return &parsedLessonLine{Lesson: strings.TrimSpace(match[2]), Subgroup: sub}
}

func applyCabinets(entries []*model.GroupLessonExplain, cabinets []string) {
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
	hasSub := false
	for _, e := range entries {
		if e.Subgroup != nil {
			hasSub = true
			break
		}
	}
	if hasSub {
		for i, e := range entries {
			idx := i
			if e.Subgroup != nil && *e.Subgroup-1 >= 0 && *e.Subgroup-1 < len(normalized) {
				idx = *e.Subgroup - 1
			} else if idx >= len(normalized) {
				idx = len(normalized) - 1
			}
			val := normalized[idx]
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

func assignGroupLesson(day *model.GroupDay, lessonNumber int, lesson model.GroupLesson) {
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

func getCellSafe(row []*GridCell, rowIndex, colIndex int) *goquery.Selection {
	if colIndex < 0 || colIndex >= len(row) || row[colIndex] == nil {
		return nil
	}
	cell := row[colIndex]
	if cell.Row != rowIndex {
		return nil
	}
	return cell.Cell
}

func postProcessGroupDay(day *model.GroupDay) {
	for i := 0; i < len(day.Lessons); i++ {
		lesson := day.Lessons[i]
		if lesson == nil {
			continue
		}
		arr := model.AsArray(lesson)
		if arr == nil {
			if s := model.AsSingle(lesson); s != nil {
				arr = []*model.GroupLessonExplain{s}
			}
		}
		if arr == nil {
			continue
		}
		allPhv := true
		for _, e := range arr {
			if e.Type == nil || *e.Type != "ф-в" || e.Comment != nil {
				allPhv = false
				break
			}
		}
		if !allPhv {
			continue
		}
		similarIdx := -1
		firstNotNull := false
		for j := len(day.Lessons) - 1; j > i; j-- {
			fl := day.Lessons[j]
			if firstNotNull && fl == nil {
				break
			}
			if fl == nil {
				continue
			}
			firstNotNull = true
			fArr := model.AsArray(fl)
			if fArr == nil {
				if fs := model.AsSingle(fl); fs != nil {
					fArr = []*model.GroupLessonExplain{fs}
				}
			}
			if fArr == nil || len(fArr) != len(arr) {
				continue
			}
			match := true
			for k := 0; k < len(arr); k++ {
				if arr[k].Type != fArr[k].Type || arr[k].Lesson != fArr[k].Lesson || arr[k].Teacher != fArr[k].Teacher {
					match = false
					break
				}
			}
			if match {
				similarIdx = j
				break
			}
		}
		if similarIdx >= 0 {
			comment := "2 часа"
			for _, e := range arr {
				e.Comment = &comment
			}
			day.Lessons[similarIdx] = nil
		}
	}
	for len(day.Lessons) > 0 && day.Lessons[len(day.Lessons)-1] == nil {
		day.Lessons = day.Lessons[:len(day.Lessons)-1]
	}
}

func mergeGroupDays(newDays, oldDays []model.GroupDay) []model.GroupDay {
	days := make(map[string]*model.GroupDay)
	for i := range oldDays {
		days[oldDays[i].Day] = &oldDays[i]
	}
	for i := range newDays {
		days[newDays[i].Day] = &newDays[i]
	}
	var result []model.GroupDay
	for _, d := range days {
		result = append(result, *d)
	}
	return result
}

func clearSundays(groups model.Groups) {
	for _, g := range groups {
		var filtered []model.GroupDay
		for _, d := range g.Days {
			t, err := time.Parse("02.01.2006", d.Day)
			if err == nil && t.Weekday() != time.Sunday {
				filtered = append(filtered, d)
			}
		}
		g.Days = filtered
	}
}
