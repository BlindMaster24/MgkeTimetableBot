package v2

import (
	"encoding/json"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type DiffLine struct {
	Key    string
	Day    string
	Reason string
}

func DiffGroups(current, previous model.Groups, limit int) []string {
	return diffAll(current, previous, limit)
}

func DiffTeachers(current, previous model.Teachers, limit int) []string {
	return diffAll(current, previous, limit)
}

func diffAll[T model.Group | model.Teacher](current, previous map[string]*T, limit int) []string {
	keys := make(map[string]bool)
	for k := range current {
		keys[k] = true
	}
	for k := range previous {
		keys[k] = true
	}

	var output []string
	for key := range keys {
		lines := diffEntries(current[key], previous[key])
		for _, line := range lines {
			output = append(output, key+": "+line.Reason+" "+line.Day)
			if len(output) >= limit {
				return output
			}
		}
	}

	return output
}

func diffEntries[T model.Group | model.Teacher](newEntry, oldEntry *T) []DiffLine {
	var lines []DiffLine

	var newDays []struct{ Day string; Lessons interface{} }
	var oldDays []struct{ Day string; Lessons interface{} }

	switch e := any(newEntry).(type) {
	case *model.Group:
		if e == nil {
			break
		}
		for _, d := range e.Days {
			newDays = append(newDays, struct{ Day string; Lessons interface{} }{d.Day, d.Lessons})
		}
	case *model.Teacher:
		if e == nil {
			break
		}
		for _, d := range e.Days {
			newDays = append(newDays, struct{ Day string; Lessons interface{} }{d.Day, d.Lessons})
		}
	}

	if oldEntry != nil {
		switch e := any(oldEntry).(type) {
		case *model.Group:
			if e == nil {
				break
			}
			for _, d := range e.Days {
				oldDays = append(oldDays, struct{ Day string; Lessons interface{} }{d.Day, d.Lessons})
			}
		case *model.Teacher:
			if e == nil {
				break
			}
			for _, d := range e.Days {
				oldDays = append(oldDays, struct{ Day string; Lessons interface{} }{d.Day, d.Lessons})
			}
		}
	}

	oldMap := make(map[string]interface{})
	for _, d := range oldDays {
		oldMap[d.Day] = d.Lessons
	}

	for _, d := range newDays {
		oldLessons, exists := oldMap[d.Day]
		if !exists {
			lines = append(lines, DiffLine{Key: "new", Day: d.Day, Reason: "added"})
			continue
		}
		newJSON, _ := json.Marshal(d.Lessons)
		oldJSON, _ := json.Marshal(oldLessons)
		if string(newJSON) != string(oldJSON) {
			lines = append(lines, DiffLine{Key: "changed", Day: d.Day, Reason: "updated"})
		}
		delete(oldMap, d.Day)
	}

	for day := range oldMap {
		lines = append(lines, DiffLine{Key: "old", Day: day, Reason: "removed"})
	}

	return lines
}
