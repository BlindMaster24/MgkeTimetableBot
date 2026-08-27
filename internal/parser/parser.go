package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type Parser interface {
	Run() (interface{}, error)
	ContentHash() string
}

func hashDocument(doc *goquery.Document) string {
	text := doc.Text()
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", h)
}

func hashJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func parseLessonRow(row *goquery.Selection, lessonIndex int) (model.GroupLesson, bool) {
	cells := row.Find("td")
	if cells.Length() == 0 {
		return nil, false
	}

	subgroupText := cells.Eq(0).Text()
	subgroup := 0
	if subgroupText != "" {
		subgroupText = trimSpaces(subgroupText)
		if len(subgroupText) <= 2 {
			fmt.Sscanf(subgroupText, "%d", &subgroup)
		}
	}

	lessonName := trimSpaces(cells.Eq(1).Text())
	lessonType := trimSpaces(cells.Eq(2).Text())
	teacher := trimSpaces(cells.Eq(3).Text())
	cabinet := trimSpaces(cells.Eq(4).Text())

	if lessonName == "" {
		return nil, false
	}

	return &model.GroupLessonExplain{
		Subgroup: &subgroup,
		Lesson:   lessonName,
		Type:     ptrString(lessonType),
		Teacher:  ptrString(teacher),
		Cabinet:  ptrString(cabinet),
	}, true
}

func parseTeacherLessonRow(row *goquery.Selection, lessonIndex int) (model.TeacherLesson, bool) {
	cells := row.Find("td")
	if cells.Length() == 0 {
		return nil, false
	}

	subgroupText := cells.Eq(0).Text()
	subgroup := 0
	if subgroupText != "" {
		subgroupText = trimSpaces(subgroupText)
		if len(subgroupText) <= 2 {
			fmt.Sscanf(subgroupText, "%d", &subgroup)
		}
	}

	lessonName := trimSpaces(cells.Eq(1).Text())
	lessonType := trimSpaces(cells.Eq(2).Text())
	cabinet := trimSpaces(cells.Eq(3).Text())
	group := trimSpaces(cells.Eq(4).Text())

	if lessonName == "" {
		return nil, false
	}

	return &model.TeacherLessonExplain{
		Lesson:   lessonName,
		Type:     ptrString(lessonType),
		Subgroup: &subgroup,
		Group:    group,
		Cabinet:  ptrString(cabinet),
	}, true
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func trimSpaces(s string) string {
	runes := []rune(s)
	start := 0
	end := len(runes)

	for start < end && (runes[start] == ' ' || runes[start] == '\t' || runes[start] == '\n' || runes[start] == '\r') {
		start++
	}
	for end > start && (runes[end-1] == ' ' || runes[end-1] == '\t' || runes[end-1] == '\n' || runes[end-1] == '\r') {
		end--
	}

	return string(runes[start:end])
}
