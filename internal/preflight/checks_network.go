package preflight

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func checkGroups(client *http.Client, url string) (model.Groups, Check) {
	if strings.TrimSpace(url) == "" {
		return nil, Check{Name: "site:groups", Level: LevelSkip, Detail: "parser.endpoints.timetable_group is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return nil, Check{Name: "site:groups", Level: LevelFail, Detail: err.Error()}
	}

	parsed := parser.NewGroupParser(doc)
	groups, err := parsed.Run()
	report := parsed.Report()

	if err != nil {
		return nil, Check{Name: "site:groups", Level: LevelFail, Detail: "parser: " + err.Error()}
	}
	if len(groups) == 0 {
		check := Check{Name: "site:groups", Level: LevelFail, Detail: url + ": the page produced no groups", Hints: probeHints(report)}
		return nil, check
	}

	dated, bad, unordered, withLessons := dayQuality(groupDays(groups))
	hints := probeHints(report)
	level := LevelOK

	if dated == 0 {
		level = LevelFail
		hints = append(hints, "error: no day column carried a dd.MM.yyyy date")
	}
	if len(bad) > 0 {
		level = LevelFail
		hints = append(hints, "error: day labels are not dd.MM.yyyy: "+strings.Join(limit(bad, 3), "; "))
	}
	if len(unordered) > 0 {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: days out of order: "+strings.Join(limit(unordered, 3), "; "))
	}
	if withLessons == 0 {
		level = LevelFail
		hints = append(hints, "error: no group had a single lesson")
	}
	if rangeHint := dateRangeHint(groupDays(groups)); rangeHint != "" {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: "+rangeHint)
	}
	if len(report.Fallbacks) > 0 && level == LevelOK {
		level = LevelWarn
	}
	if len(report.Failing()) > 0 && level == LevelOK {
		level = LevelWarn
	}

	return groups, Check{
		Name:   "site:groups",
		Level:  level,
		Detail: fmt.Sprintf("%d groups, %d with lessons, %d dated day columns", len(groups), withLessons, dated),
		Hints:  hints,
	}
}

func checkTeachers(client *http.Client, url string) Check {
	if strings.TrimSpace(url) == "" {
		return Check{Name: "site:teachers", Level: LevelSkip, Detail: "parser.endpoints.timetable_teacher is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: err.Error()}
	}

	parsed := parser.NewTeacherParser(doc)
	teachers, err := parsed.Run()
	report := parsed.Report()

	if err != nil {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: "parser: " + err.Error()}
	}
	if len(teachers) == 0 {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: url + ": the page produced no teachers", Hints: probeHints(report)}
	}

	dated, bad, unordered, withLessons := dayQuality(teacherDays(teachers))
	hints := probeHints(report)
	level := LevelOK

	if dated == 0 {
		level = LevelFail
		hints = append(hints, "error: no day column carried a dd.MM.yyyy date")
	}
	if len(bad) > 0 {
		level = LevelFail
		hints = append(hints, "error: day labels are not dd.MM.yyyy: "+strings.Join(limit(bad, 3), "; "))
	}
	if len(unordered) > 0 && level == LevelOK {
		level = LevelWarn
		hints = append(hints, "warning: days out of order: "+strings.Join(limit(unordered, 3), "; "))
	}
	if withLessons == 0 {
		level = LevelFail
		hints = append(hints, "error: no teacher had a single lesson")
	}
	if len(report.Failing()) > 0 && level == LevelOK {
		level = LevelWarn
	}

	return Check{
		Name:   "site:teachers",
		Level:  level,
		Detail: fmt.Sprintf("%d teachers, %d with lessons, %d dated day columns", len(teachers), withLessons, dated),
		Hints:  hints,
	}
}

func checkCalls(client *http.Client, url string, now time.Time, staleDays int) Check {
	if strings.TrimSpace(url) == "" {
		return Check{Name: "site:calls", Level: LevelSkip, Detail: "parser.endpoints.bell_schedule is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return Check{Name: "site:calls", Level: LevelFail, Detail: err.Error()}
	}

	variants, report := parser.ParseCallsVariants(doc)
	hints := probeHints(report)
	if len(variants) == 0 || len(variants[0].Schedule.Weekdays) == 0 {
		hints = append(hints, "error: no bell schedule slot was recognised")
		return Check{Name: "site:calls", Level: LevelFail, Detail: url + ": the page produced no bell schedule", Hints: hints}
	}

	weekdays := variants[0].Schedule.Weekdays
	saturday := variants[0].Schedule.Saturday

	problems := append(slotProblems("site weekdays", weekdays), slotProblems("site saturday", saturday)...)

	level := LevelOK
	if len(problems) > 0 {
		level = LevelFail
		for _, problem := range problems {
			hints = append(hints, "error: "+problem)
		}
	}
	if level == LevelOK && (len(report.Failing()) > 0 || len(report.Advisories()) > 0) {
		level = LevelWarn
	}
	if len(saturday) == 0 {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: the site published no saturday slots")
	}

	detail := fmt.Sprintf("%d variant(s), %d weekday slots, %d saturday slots", len(variants), len(weekdays), len(saturday))
	if updated := parser.ParseCallsUpdatedAt(doc); updated.At != 0 {
		declared := time.UnixMilli(updated.At).UTC()
		label := declared.Format("02.01.2006")
		detail += ", dated " + label
		switch age := now.Sub(declared); {
		case age < -24*time.Hour:
			if level == LevelOK {
				level = LevelWarn
			}
			hints = append(hints, "warning: the bell schedule page is dated in the future: "+label)
		case age > time.Duration(staleDays)*24*time.Hour:
			if level == LevelOK {
				level = LevelWarn
			}
			hints = append(hints, fmt.Sprintf("warning: the bell schedule page is dated %s, %d days old (limit %d days)", label, int(age.Hours()/24), staleDays))
		}
	}
	if len(variants) > 1 {
		names := make([]string, 0, len(variants))
		for _, variant := range variants {
			if strings.TrimSpace(variant.Name) != "" {
				names = append(names, variant.Name)
			}
		}
		if len(names) > 0 {
			detail += " (" + strings.Join(names, ", ") + ")"
		}
	}

	return Check{Name: "site:calls", Level: level, Detail: detail, Hints: hints}
}
