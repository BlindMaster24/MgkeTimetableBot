package v2

import (
	"math/rand"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

type ValidationResult struct {
	OK     bool
	Errors []string
}

func ValidateGroups(groups model.Groups, maxLessons int, sampleSize int) ValidationResult {
	entries := make([]*model.Group, 0, len(groups))
	for _, v := range groups {
		entries = append(entries, v)
	}

	if len(entries) == 0 {
		return ValidationResult{OK: false, Errors: []string{"empty groups"}}
	}

	sampled := pickSampleGroups(entries, sampleSize)
	var errors []string
	hasLessons := false

	for _, entry := range sampled {
		entryErrors := validateGroupEntry(entry, maxLessons)
		if len(entryErrors) > 0 {
			errors = append(errors, entry.Group+": "+strings.Join(entryErrors, ", "))
		}
		if !hasLessons {
			for _, d := range entry.Days {
				if len(d.Lessons) > 0 {
					hasLessons = true
					break
				}
			}
		}
	}

	if !hasLessons {
		errors = append(errors, "no lessons in sample")
	}

	return ValidationResult{OK: len(errors) == 0, Errors: errors}
}

func ValidateTeachers(teachers model.Teachers, maxLessons int, sampleSize int) ValidationResult {
	entries := make([]*model.Teacher, 0, len(teachers))
	for _, v := range teachers {
		entries = append(entries, v)
	}

	if len(entries) == 0 {
		return ValidationResult{OK: false, Errors: []string{"empty teachers"}}
	}

	sampled := pickSampleTeachers(entries, sampleSize)
	var errors []string
	hasLessons := false

	for _, entry := range sampled {
		entryErrors := validateTeacherEntry(entry, maxLessons)
		if len(entryErrors) > 0 {
			errors = append(errors, entry.Teacher+": "+strings.Join(entryErrors, ", "))
		}
		if !hasLessons {
			for _, d := range entry.Days {
				if len(d.Lessons) > 0 {
					hasLessons = true
					break
				}
			}
		}
	}

	if !hasLessons {
		errors = append(errors, "no lessons in sample")
	}

	return ValidationResult{OK: len(errors) == 0, Errors: errors}
}

func validateGroupEntry(entry *model.Group, maxLessons int) []string {
	return validateDays(entry.Days, maxLessons, func(d model.GroupDay) string { return d.Day }, func(d model.GroupDay) int { return len(d.Lessons) })
}

func validateTeacherEntry(entry *model.Teacher, maxLessons int) []string {
	return validateDays(entry.Days, maxLessons, func(d model.TeacherDay) string { return d.Day }, func(d model.TeacherDay) int { return len(d.Lessons) })
}

func validateDays[T any](days []T, maxLessons int, getDay func(T) string, getLessonCount func(T) int) []string {
	if len(days) == 0 {
		return []string{"no days"}
	}

	var errors []string
	seen := make(map[string]bool)

	for _, d := range days {
		dayStr := getDay(d)
		if dayStr == "" {
			errors = append(errors, "empty day")
			continue
		}
		if seen[dayStr] {
			errors = append(errors, "duplicate day "+dayStr)
		}
		seen[dayStr] = true
		if getLessonCount(d) > maxLessons {
			errors = append(errors, "too many lessons "+dayStr)
		}
	}

	return errors
}

func pickSampleGroups(items []*model.Group, size int) []*model.Group {
	if size <= 0 || len(items) <= size {
		return items
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return items[:size]
}

func pickSampleTeachers(items []*model.Teacher, size int) []*model.Teacher {
	if size <= 0 || len(items) <= size {
		return items
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return items[:size]
}
