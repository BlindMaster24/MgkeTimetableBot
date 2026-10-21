package cache

import (
	"encoding/json"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type GroupTimetable map[string]timetable.GroupSchedule

type TeacherTimetable map[string]timetable.TeacherSchedule

func datedDays(days []timetable.Day) []timetable.Day {
	if len(days) == 0 {
		return days
	}

	kept := make([]timetable.Day, 0, len(days))
	for _, day := range days {
		dated, ok := day.Dated()
		if !ok {
			continue
		}
		kept = append(kept, dated)
	}
	return kept
}

func onlyDatedEntries(entries map[string]any) map[string]any {
	sanitized := make(map[string]any, len(entries))

	for name, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			sanitized[name] = value
			continue
		}
		rawDays, ok := entry["days"].([]any)
		if !ok {
			sanitized[name] = value
			continue
		}

		days := make([]any, 0, len(rawDays))
		modified := false
		for _, raw := range rawDays {
			day, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			label, _ := day["day"].(string)
			dated, ok := timetable.Day{Date: label}.Dated()
			if !ok {
				continue
			}
			if dated.Date == strings.TrimSpace(label) {
				days = append(days, raw)
				continue
			}
			copiedDay := make(map[string]any, len(day))
			for key, item := range day {
				copiedDay[key] = item
			}
			copiedDay["day"] = dated.Date
			days = append(days, copiedDay)
			modified = true
		}
		if len(days) == len(rawDays) && !modified {
			sanitized[name] = value
			continue
		}

		copied := make(map[string]any, len(entry))
		for key, item := range entry {
			copied[key] = item
		}
		copied["days"] = days
		sanitized[name] = copied
	}

	return sanitized
}

func plausibleGroups(schedules map[string]timetable.GroupSchedule) bool {
	for _, schedule := range schedules {
		if err := schedule.Validate(); err != nil {
			return false
		}
	}
	return true
}

func plausibleTeachers(schedules map[string]timetable.TeacherSchedule) bool {
	for _, schedule := range schedules {
		if err := schedule.Validate(); err != nil {
			return false
		}
	}
	return true
}

func (t *GroupTimetable) UnmarshalJSON(data []byte) error {
	var typed map[string]timetable.GroupSchedule
	if err := json.Unmarshal(data, &typed); err == nil {
		for name, schedule := range typed {
			schedule.Days = datedDays(schedule.Days)
			if schedule.Group == "" {
				schedule.Group = name
			}
			typed[name] = schedule
		}
		if plausibleGroups(typed) {
			*t = typed
			return nil
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	converted := make(GroupTimetable, len(raw))
	for name, entry := range raw {
		schedule, err := timetable.GroupScheduleFromJSON(entry)
		if err != nil {
			return err
		}
		schedule.Days = datedDays(schedule.Days)
		if schedule.Group == "" {
			schedule.Group = name
		}
		converted[name] = schedule
	}
	*t = converted
	return nil
}

func (t *TeacherTimetable) UnmarshalJSON(data []byte) error {
	var typed map[string]timetable.TeacherSchedule
	if err := json.Unmarshal(data, &typed); err == nil {
		for name, schedule := range typed {
			schedule.Days = datedDays(schedule.Days)
			if schedule.Teacher == "" {
				schedule.Teacher = name
			}
			typed[name] = schedule
		}
		if plausibleTeachers(typed) {
			*t = typed
			return nil
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	converted := make(TeacherTimetable, len(raw))
	for name, entry := range raw {
		schedule, err := timetable.TeacherScheduleFromJSON(entry)
		if err != nil {
			return err
		}
		schedule.Days = datedDays(schedule.Days)
		if schedule.Teacher == "" {
			schedule.Teacher = name
		}
		converted[name] = schedule
	}
	*t = converted
	return nil
}

func (t GroupTimetable) AnyView() map[string]any {
	view := make(map[string]any, len(t))
	for name, schedule := range t {
		view[name] = schedule.ToAnyMap()
	}
	return view
}

func (t TeacherTimetable) AnyView() map[string]any {
	view := make(map[string]any, len(t))
	for name, schedule := range t {
		view[name] = schedule.ToAnyMap()
	}
	return view
}

func GroupTimetableFromAny(data map[string]any) GroupTimetable {
	converted := make(GroupTimetable, len(data))
	for name, value := range data {
		encoded, err := json.Marshal(value)
		if err != nil {
			continue
		}
		schedule, err := timetable.GroupScheduleFromJSON(encoded)
		if err != nil {
			continue
		}
		schedule.Days = datedDays(schedule.Days)
		if schedule.Group == "" {
			schedule.Group = name
		}
		converted[name] = schedule
	}
	return converted
}

func TeacherTimetableFromAny(data map[string]any) TeacherTimetable {
	converted := make(TeacherTimetable, len(data))
	for name, value := range data {
		encoded, err := json.Marshal(value)
		if err != nil {
			continue
		}
		schedule, err := timetable.TeacherScheduleFromJSON(encoded)
		if err != nil {
			continue
		}
		schedule.Days = datedDays(schedule.Days)
		if schedule.Teacher == "" {
			schedule.Teacher = name
		}
		converted[name] = schedule
	}
	return converted
}
