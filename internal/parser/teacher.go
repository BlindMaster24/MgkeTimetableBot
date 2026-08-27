package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

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

	content := findContent(p.doc)
	h3s := content.Find("h3")
	h3s.Each(func(i int, h3 *goquery.Selection) {
		teacherName := strings.TrimSpace(h3.Text())
		if teacherName == "" {
			return
		}

		table := h3.NextAllFiltered("table").First()
		if table.Length() == 0 {
			return
		}

		days := p.parseTeacherTable(table)

		if teachers[teacherName] == nil {
			teachers[teacherName] = &model.Teacher{
				Teacher: teacherName,
				Days:    []model.TeacherDay{},
			}
		}
		teachers[teacherName].Days = append(teachers[teacherName].Days, days...)
	})

	return teachers, nil
}

func (p *TeacherParser) parseTeacherTable(table *goquery.Selection) []model.TeacherDay {
	var days []model.TeacherDay

	rows := table.Find("tr")
	rows.Each(func(i int, row *goquery.Selection) {
		if i == 0 {
			return
		}

		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}

		dayStr := strings.TrimSpace(cells.Eq(0).Text())
		if dayStr == "" {
			return
		}

		lessonCell := cells.Eq(1)
		lessonText := strings.TrimSpace(lessonCell.Text())

		var lessons []model.TeacherLesson

		if lessonText == "" || lessonText == "-" || lessonText == "\u2014" {
			days = append(days, model.TeacherDay{
				Day:     dayStr,
				Lessons: lessons,
			})
			return
		}

		lessonCell.Find("br").ReplaceWithHtml("\n")
		lines := strings.Split(lessonCell.Text(), "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, " ", 4)
			if len(parts) < 3 {
				continue
			}

			lesson := &model.TeacherLessonExplain{
				Group:  parts[0],
				Lesson: parts[1],
			}

			if len(parts) > 2 {
				lesson.Type = ptrString(parts[2])
			}
			if len(parts) > 3 {
				lesson.Cabinet = ptrString(parts[3])
			}

			lessons = append(lessons, lesson)
		}

		days = append(days, model.TeacherDay{
			Day:     dayStr,
			Lessons: lessons,
		})
	})

	return days
}
