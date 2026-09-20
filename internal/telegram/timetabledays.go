package telegram

import (
	"encoding/json"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

type daySource uint8

const (
	daysFromCache daySource = iota
	daysFromArchive
)

type archiveResult uint8

const (
	archiveAnswered archiveResult = iota
	archiveUnavailable
	archiveFailed
)

type scheduleTarget struct {
	typeName               string
	value                  string
	notSelectedKey         string
	notExistsKey           string
	hintField              string
	hintMap                func(*Bot) map[string]any
	rolloverWhenPastHidden bool
}

func scheduleTargetFor(chat *Chat) (scheduleTarget, bool) {
	switch chat.Mode {
	case ModeStudent, ModeParent:
		return scheduleTarget{
			typeName:               "group",
			value:                  chat.Group,
			notSelectedKey:         "group_not_selected",
			notExistsKey:           "group_not_exists",
			hintField:              "Group",
			hintMap:                func(b *Bot) map[string]any { return b.cache.GetGroups() },
			rolloverWhenPastHidden: true,
		}, true
	case ModeTeacher:
		return scheduleTarget{
			typeName:       "teacher",
			value:          chat.Teacher,
			notSelectedKey: "teacher_not_selected",
			notExistsKey:   "teacher_not_exists",
			hintField:      "Teacher",
			hintMap:        func(b *Bot) map[string]any { return b.cache.GetTeachers() },
		}, true
	}
	return scheduleTarget{}, false
}

func (b *Bot) formatDays(chat *Chat, typeName, value string, days []map[string]any, opts formatter.FormatOptions) string {
	if typeName == "teacher" {
		return formatter.GetByIndex(chat.Formatter).FormatTeacherFull(value, days, opts)
	}
	return formatter.GetByIndex(chat.Formatter).FormatGroupFull(value, days, opts)
}

func (b *Bot) cacheEntry(typeName, value string) (any, bool) {
	if typeName == "teacher" {
		data, ok := b.cache.GetTeachers()[value]
		return data, ok
	}
	data, ok := b.cache.GetGroups()[value]
	return data, ok
}

func (b *Bot) hasCachedValue(typeName, value string) bool {
	_, ok := b.cacheEntry(typeName, value)
	return ok
}

func (b *Bot) cacheDaysForRange(typeName, value string, minIdx, maxIdx int) []map[string]any {
	data, ok := b.cacheEntry(typeName, value)
	if !ok {
		return nil
	}
	return extractDaysFromRange(data, minIdx, maxIdx)
}

func (b *Bot) archiveDaysForRange(typeName, value string, minIdx, maxIdx int) ([]map[string]any, archiveResult) {
	if b.archive == nil {
		return nil, archiveUnavailable
	}

	if typeName == "teacher" {
		days, err := b.archive.TeacherDaysByRange(int64(minIdx), int64(maxIdx), value)
		if err != nil {
			return nil, archiveFailed
		}
		return daysToMaps(days), archiveAnswered
	}

	days, err := b.archive.GroupDaysByRange(int64(minIdx), int64(maxIdx), value)
	if err != nil {
		return nil, archiveFailed
	}
	return daysToMaps(days), archiveAnswered
}

func (b *Bot) daysForRange(source daySource, typeName, value string, minIdx, maxIdx int) []map[string]any {
	if source == daysFromArchive {
		days, result := b.archiveDaysForRange(typeName, value, minIdx, maxIdx)
		switch result {
		case archiveAnswered:
			return days
		case archiveFailed:
			return nil
		}
	}
	return b.cacheDaysForRange(typeName, value, minIdx, maxIdx)
}

func (b *Bot) weekDays(source daySource, typeName, value string, week utils.WeekIndex) []map[string]any {
	minIdx, maxIdx := week.WeekDayIndexRange()
	return b.daysForRange(source, typeName, value, minIdx, maxIdx)
}

func daysToMaps[T any](days []T) []map[string]any {
	var result []map[string]any
	for _, d := range days {
		raw, err := json.Marshal(d)
		if err != nil {
			continue
		}
		var day map[string]any
		if err := json.Unmarshal(raw, &day); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"day":     day["day"],
			"lessons": day["lessons"],
		})
	}
	return result
}

func extractDaysFromRange(data any, minIdx, maxIdx int) []map[string]any {
	allDays := extractDays(data)
	if len(allDays) == 0 {
		return nil
	}

	var result []map[string]any
	for _, day := range allDays {
		dateStr, _ := day["day"].(string)
		t, err := time.Parse("02.01.2006", dateStr)
		if err != nil {
			continue
		}
		idx := utils.DayIndexFromDate(t)
		if idx >= minIdx && idx <= maxIdx {
			result = append(result, day)
		}
	}
	return result
}
