package parser

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

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

	content := findContent(p.doc)
	h2s := content.Find("h2")
	h2s.Each(func(i int, h2 *goquery.Selection) {
		dayStr := extractDayString(h2.Text())
		if dayStr == "" {
			return
		}

		table := h2.NextAllFiltered("table").First()
		if table.Length() == 0 {
			return
		}

		groupsOnDay := p.parseDayTable(table)

		for groupKey, lessons := range groupsOnDay {
			if groups[groupKey] == nil {
				groups[groupKey] = &model.Group{
					Group: groupKey,
					Days:  []model.GroupDay{},
				}
			}
			groups[groupKey].Days = append(groups[groupKey].Days, model.GroupDay{
				Day:     dayStr,
				Lessons: lessons,
			})
		}
	})

	return groups, nil
}

func (p *GroupParser) parseDayTable(table *goquery.Selection) map[string][]model.GroupLesson {
	result := make(map[string][]model.GroupLesson)

	rows := table.Find("tr")
	rows.Each(func(i int, row *goquery.Selection) {
		if i == 0 {
			return
		}

		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}

		groupName := strings.TrimSpace(cells.Eq(0).Text())
		if groupName == "" {
			return
		}

		lessonCell := cells.Eq(1)
		lessonText := strings.TrimSpace(lessonCell.Text())

		if lessonText == "" || lessonText == "-" || lessonText == "\u2014" {
			result[groupName] = append(result[groupName], nil)
			return
		}

		subgroups := parseGroupLessonCell(lessonCell)
		if len(subgroups) == 1 {
			result[groupName] = append(result[groupName], subgroups[0])
		} else if len(subgroups) > 1 {
			result[groupName] = append(result[groupName], subgroups)
		}
	})

	return result
}

func parseGroupLessonCell(cell *goquery.Selection) []*model.GroupLessonExplain {
	var subgroups []*model.GroupLessonExplain

	cell.Find("br").ReplaceWithHtml("\n")
	lines := strings.Split(cell.Text(), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 4)
		if len(parts) < 2 {
			continue
		}

		subgroup := 0
		lessonIdx := 0
		if len(parts) >= 3 {
			var parsed int
			if _, err := fmt.Sscanf(parts[0], "%d", &parsed); err == nil {
				subgroup = parsed
				lessonIdx = 1
			}
		}

		if lessonIdx >= len(parts) {
			continue
		}

		lesson := &model.GroupLessonExplain{
			Subgroup: &subgroup,
			Lesson:   parts[lessonIdx],
		}

		if len(parts) > lessonIdx+1 {
			lesson.Type = ptrString(parts[lessonIdx+1])
		}
		if len(parts) > lessonIdx+2 {
			lesson.Teacher = ptrString(parts[lessonIdx+2])
		}
		if len(parts) > lessonIdx+3 {
			lesson.Cabinet = ptrString(parts[lessonIdx+3])
		}

		subgroups = append(subgroups, lesson)
	}

	return subgroups
}
