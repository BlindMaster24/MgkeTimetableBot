package cache

import (
	"encoding/json"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

type GroupTimetable map[string]timetable.GroupSchedule

type TeacherTimetable map[string]timetable.TeacherSchedule

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
	if err := json.Unmarshal(data, &typed); err == nil && plausibleGroups(typed) {
		for name, schedule := range typed {
			if schedule.Group == "" {
				schedule.Group = name
				typed[name] = schedule
			}
		}
		*t = typed
		return nil
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
	if err := json.Unmarshal(data, &typed); err == nil && plausibleTeachers(typed) {
		for name, schedule := range typed {
			if schedule.Teacher == "" {
				schedule.Teacher = name
				typed[name] = schedule
			}
		}
		*t = typed
		return nil
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
		if schedule.Teacher == "" {
			schedule.Teacher = name
		}
		converted[name] = schedule
	}
	return converted
}
