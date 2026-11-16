package google

import (
	"fmt"
	"strconv"
	"strings"
)

type GroupLesson struct {
	Subgroup *int
	Name     string
	Type     string
	Teacher  string
	Cabinet  string
	Comment  string
}

type TeacherLesson struct {
	Subgroup *int
	Group    string
	Name     string
	Type     string
	Cabinet  string
	Comment  string
}

type Texts struct {
	Subgroup string
	Subject  string
	Type     string
	Teacher  string
	Cabinet  string
	Comment  string
	Group    string
}

func GroupLessonInfo(texts Texts, lessons []GroupLesson) (string, string, string) {
	if len(lessons) == 0 {
		return "", "", ""
	}

	sameName := true
	for _, lesson := range lessons {
		if lesson.Name != lessons[0].Name {
			sameName = false
			break
		}
	}

	var title string
	if sameName && len(lessons) > 1 {
		subgroups := make([]string, 0, len(lessons))
		for _, lesson := range lessons {
			subgroups = append(subgroups, subgroupValue(lesson.Subgroup))
		}
		title = strings.Join(subgroups, ",") + " - " + lessons[0].Name
	} else {
		parts := make([]string, 0, len(lessons))
		for _, lesson := range lessons {
			parts = append(parts, subgroupPrefix(lesson.Subgroup)+lesson.Name)
		}
		title = strings.Join(parts, " | ")
	}

	blocks := make([]string, 0, len(lessons))
	for _, lesson := range lessons {
		var lines []string
		if lesson.Subgroup != nil && *lesson.Subgroup != 0 {
			lines = append(lines, fmt.Sprintf(texts.Subgroup, *lesson.Subgroup))
		}
		lines = append(lines, fmt.Sprintf(texts.Subject, lesson.Name))
		if lesson.Type != "" {
			lines = append(lines, fmt.Sprintf(texts.Type, lesson.Type))
		}
		if lesson.Teacher != "" {
			lines = append(lines, fmt.Sprintf(texts.Teacher, lesson.Teacher))
		}
		lines = append(lines, fmt.Sprintf(texts.Cabinet, cabinetValue(lesson.Cabinet)))
		if lesson.Comment != "" {
			lines = append(lines, fmt.Sprintf(texts.Comment, lesson.Comment))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	location := ""
	if hasAnyCabinet(len(lessons), func(i int) string { return lessons[i].Cabinet }) {
		cabinets := make([]string, 0, len(lessons))
		for _, lesson := range lessons {
			cabinets = append(cabinets, cabinetValue(lesson.Cabinet))
		}
		location = strings.Join(cabinets, " | ")
	}

	return title, strings.Join(blocks, "\n\n"), location
}

func TeacherLessonInfo(texts Texts, lesson TeacherLesson) (string, string, string) {
	var lines []string
	lines = append(lines, fmt.Sprintf(texts.Subject, lesson.Name))
	if lesson.Type != "" {
		lines = append(lines, fmt.Sprintf(texts.Type, lesson.Type))
	}
	if lesson.Group != "" {
		lines = append(lines, fmt.Sprintf(texts.Group, subgroupPrefix(lesson.Subgroup)+lesson.Group))
	}
	lines = append(lines, fmt.Sprintf(texts.Cabinet, cabinetValue(lesson.Cabinet)))
	if lesson.Comment != "" {
		lines = append(lines, fmt.Sprintf(texts.Comment, lesson.Comment))
	}

	title := subgroupPrefix(lesson.Subgroup) + lesson.Group + "-" + lesson.Name
	return title, strings.Join(lines, "\n"), lesson.Cabinet
}

func subgroupPrefix(subgroup *int) string {
	if subgroup == nil || *subgroup == 0 {
		return ""
	}
	return strconv.Itoa(*subgroup) + ". "
}

func subgroupValue(subgroup *int) string {
	if subgroup == nil {
		return ""
	}
	return strconv.Itoa(*subgroup)
}

func cabinetValue(cabinet string) string {
	if cabinet == "" {
		return "-"
	}
	return cabinet
}

func hasAnyCabinet(count int, cabinet func(int) string) bool {
	for i := 0; i < count; i++ {
		if cabinet(i) != "" {
			return true
		}
	}
	return false
}
