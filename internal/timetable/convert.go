package timetable

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var dateLabelRe = regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)

var groupLessonKeys = []string{"lesson", "type", "teacher", "cabinet", "comment", "subgroup"}
var teacherLessonKeys = []string{"lesson", "type", "subgroup", "group", "cabinet", "comment"}
var dayKeys = []string{"day", "lessons"}
var groupEntryKeys = []string{"group", "days", "lastNoticedDay"}
var teacherEntryKeys = []string{"teacher", "days", "lastNoticedDay"}

func normalizeDateLabel(value string) string {
	if match := dateLabelRe.FindString(value); match != "" {
		return match
	}
	return strings.TrimSpace(value)
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func textPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func subgroupPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func splitExtras(raw map[string]any, known []string) map[string]any {
	keep := make(map[string]bool, len(known))
	for _, key := range known {
		keep[key] = true
	}
	var extra map[string]any
	for key, value := range raw {
		if !keep[key] {
			if extra == nil {
				extra = make(map[string]any)
			}
			extra[key] = value
		}
	}
	return extra
}

func mergeExtras(base, extra map[string]any) {
	for key, value := range extra {
		if _, taken := base[key]; !taken {
			base[key] = value
		}
	}
}

func slotNumbers(counts []int) []int {
	nums := make([]int, 0, 64)
	for slot, count := range counts {
		for i := 0; i < count; i++ {
			nums = append(nums, slot+1)
		}
	}
	return nums
}

func groupDayFromRaw(dayRaw map[string]any) (Day, error) {
	label, _ := dayRaw["day"].(string)
	day := Day{Date: normalizeDateLabel(label), Extra: splitExtras(dayRaw, dayKeys)}
	rawLessons, _ := dayRaw["lessons"].([]any)
	if dayRaw["lessons"] != nil && rawLessons == nil {
		return Day{}, fmt.Errorf("day %q holds lessons of unknown shape", label)
	}
	counts := make([]int, 0, len(rawLessons))
	flat := make([]map[string]any, 0, len(rawLessons))
	for _, item := range rawLessons {
		if item == nil {
			continue
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return Day{}, fmt.Errorf("day %q: %v", label, err)
		}
		var single map[string]any
		var multi []any
		if err := json.Unmarshal(encoded, &single); err == nil && single != nil {
			flat = append(flat, single)
			counts = append(counts, 1)
			continue
		}
		if err := json.Unmarshal(encoded, &multi); err != nil {
			return Day{}, fmt.Errorf("day %q holds a lesson of unknown shape", label)
		}
		slot := 0
		for _, sub := range multi {
			slotMap, ok := sub.(map[string]any)
			if !ok {
				return Day{}, fmt.Errorf("day %q holds a lesson of unknown shape", label)
			}
			flat = append(flat, slotMap)
			slot++
		}
		counts = append(counts, slot)
	}
	nums := slotNumbers(counts)
	for i, raw := range flat {
		var explain model.GroupLessonExplain
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &explain); err != nil {
			return Day{}, fmt.Errorf("day %q: %v", label, err)
		}
		var subgroup int
		if explain.Subgroup != nil {
			subgroup = *explain.Subgroup
		}
		day.Lessons = append(day.Lessons, Lesson{
			Num:      nums[i],
			Subject:  explain.Lesson,
			Type:     text(explain.Type),
			Teacher:  text(explain.Teacher),
			Room:     text(explain.Cabinet),
			Comment:  text(explain.Comment),
			Subgroup: subgroup,
			Extra:    splitExtras(raw, groupLessonKeys),
		})
	}
	return day, nil
}

func teacherDayFromRaw(dayRaw map[string]any) (Day, error) {
	label, _ := dayRaw["day"].(string)
	day := Day{Date: normalizeDateLabel(label), Extra: splitExtras(dayRaw, dayKeys)}
	rawLessons, _ := dayRaw["lessons"].([]any)
	if dayRaw["lessons"] != nil && rawLessons == nil {
		return Day{}, fmt.Errorf("day %q holds lessons of unknown shape", label)
	}
	for _, item := range rawLessons {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var explain model.TeacherLessonExplain
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &explain); err != nil {
			return Day{}, fmt.Errorf("day %q: %v", label, err)
		}
		var subgroup int
		if explain.Subgroup != nil {
			subgroup = *explain.Subgroup
		}
		day.Lessons = append(day.Lessons, Lesson{
			Num:      len(day.Lessons) + 1,
			Subject:  explain.Lesson,
			Type:     text(explain.Type),
			Group:    explain.Group,
			Room:     text(explain.Cabinet),
			Comment:  text(explain.Comment),
			Subgroup: subgroup,
			Extra:    splitExtras(raw, teacherLessonKeys),
		})
	}
	return day, nil
}

func FromGroupDay(day model.GroupDay) (Day, error) {
	encoded, err := json.Marshal(day)
	if err != nil {
		return Day{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return Day{}, err
	}
	return groupDayFromRaw(raw)
}

func FromTeacherDay(day model.TeacherDay) (Day, error) {
	encoded, err := json.Marshal(day)
	if err != nil {
		return Day{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return Day{}, err
	}
	return teacherDayFromRaw(raw)
}

func GroupScheduleFromJSON(data []byte) (GroupSchedule, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return GroupSchedule{}, err
	}
	name, _ := raw["group"].(string)
	schedule := GroupSchedule{Group: name, Extra: splitExtras(raw, groupEntryKeys)}
	if noticed, ok := raw["lastNoticedDay"].(float64); ok {
		schedule.LastNoticed = int64(noticed)
	}
	rawDays, _ := raw["days"].([]any)
	if raw["days"] != nil && rawDays == nil {
		return GroupSchedule{}, fmt.Errorf("group %q holds days of unknown shape", name)
	}
	for _, item := range rawDays {
		dayRaw, ok := item.(map[string]any)
		if !ok {
			return GroupSchedule{}, fmt.Errorf("group %q holds a day of unknown shape", name)
		}
		day, err := groupDayFromRaw(dayRaw)
		if err != nil {
			return GroupSchedule{}, err
		}
		schedule.Days = append(schedule.Days, day)
	}
	return schedule, nil
}

func TeacherScheduleFromJSON(data []byte) (TeacherSchedule, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return TeacherSchedule{}, err
	}
	name, _ := raw["teacher"].(string)
	schedule := TeacherSchedule{Teacher: name, Extra: splitExtras(raw, teacherEntryKeys)}
	if noticed, ok := raw["lastNoticedDay"].(float64); ok {
		schedule.LastNoticed = int64(noticed)
	}
	rawDays, _ := raw["days"].([]any)
	if raw["days"] != nil && rawDays == nil {
		return TeacherSchedule{}, fmt.Errorf("teacher %q holds days of unknown shape", name)
	}
	for _, item := range rawDays {
		dayRaw, ok := item.(map[string]any)
		if !ok {
			return TeacherSchedule{}, fmt.Errorf("teacher %q holds a day of unknown shape", name)
		}
		day, err := teacherDayFromRaw(dayRaw)
		if err != nil {
			return TeacherSchedule{}, err
		}
		schedule.Days = append(schedule.Days, day)
	}
	return schedule, nil
}

func FromModelGroup(group *model.Group) (GroupSchedule, error) {
	if group == nil {
		return GroupSchedule{}, fmt.Errorf("group schedule is nil")
	}
	encoded, err := json.Marshal(group)
	if err != nil {
		return GroupSchedule{}, err
	}
	schedule, err := GroupScheduleFromJSON(encoded)
	if err != nil {
		return GroupSchedule{}, err
	}
	schedule.LastNoticed = group.LastNoticed
	return schedule, nil
}

func FromModelTeacher(teacher *model.Teacher) (TeacherSchedule, error) {
	if teacher == nil {
		return TeacherSchedule{}, fmt.Errorf("teacher schedule is nil")
	}
	encoded, err := json.Marshal(teacher)
	if err != nil {
		return TeacherSchedule{}, err
	}
	schedule, err := TeacherScheduleFromJSON(encoded)
	if err != nil {
		return TeacherSchedule{}, err
	}
	schedule.LastNoticed = teacher.LastNoticed
	return schedule, nil
}

func (l Lesson) groupLessonMap() map[string]any {
	lesson := map[string]any{"lesson": l.Subject}
	if l.Type != "" {
		lesson["type"] = l.Type
	}
	if l.Teacher != "" {
		lesson["teacher"] = l.Teacher
	}
	if l.Room != "" {
		lesson["cabinet"] = l.Room
	}
	if l.Comment != "" {
		lesson["comment"] = l.Comment
	}
	if l.Subgroup != 0 {
		lesson["subgroup"] = float64(l.Subgroup)
	}
	mergeExtras(lesson, l.Extra)
	return lesson
}

func (l Lesson) teacherLessonMap() map[string]any {
	lesson := map[string]any{"lesson": l.Subject}
	if l.Type != "" {
		lesson["type"] = l.Type
	}
	if l.Subgroup != 0 {
		lesson["subgroup"] = float64(l.Subgroup)
	}
	if l.Group != "" {
		lesson["group"] = l.Group
	}
	if l.Room != "" {
		lesson["cabinet"] = l.Room
	}
	if l.Comment != "" {
		lesson["comment"] = l.Comment
	}
	mergeExtras(lesson, l.Extra)
	return lesson
}

func (d Day) groupDayMap() map[string]any {
	day := map[string]any{"day": d.Date}
	lessons := make([]any, 0, len(d.Lessons))
	for i := 0; i < len(d.Lessons); {
		j := i
		for j < len(d.Lessons) && d.Lessons[j].Num == d.Lessons[i].Num {
			j++
		}
		slot := d.Lessons[i:j]
		if len(slot) == 1 {
			lessons = append(lessons, slot[0].groupLessonMap())
		} else {
			array := make([]any, 0, len(slot))
			for _, lesson := range slot {
				array = append(array, lesson.groupLessonMap())
			}
			lessons = append(lessons, array)
		}
		i = j
	}
	day["lessons"] = lessons
	mergeExtras(day, d.Extra)
	return day
}

func (d Day) teacherDayMap() map[string]any {
	day := map[string]any{"day": d.Date}
	lessons := make([]any, 0, len(d.Lessons))
	for _, lesson := range d.Lessons {
		lessons = append(lessons, lesson.teacherLessonMap())
	}
	day["lessons"] = lessons
	mergeExtras(day, d.Extra)
	return day
}

type GroupSchedule struct {
	Group       string         `json:"group"`
	Days        []Day          `json:"days"`
	LastNoticed int64          `json:"lastNoticedDay,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type TeacherSchedule struct {
	Teacher     string         `json:"teacher"`
	Days        []Day          `json:"days"`
	LastNoticed int64          `json:"lastNoticedDay,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

func (s GroupSchedule) ToAnyMap() map[string]any {
	entry := map[string]any{"group": s.Group}
	if len(s.Days) > 0 {
		days := make([]any, 0, len(s.Days))
		for _, day := range s.Days {
			days = append(days, day.groupDayMap())
		}
		entry["days"] = days
	}
	if s.LastNoticed != 0 {
		entry["lastNoticedDay"] = float64(s.LastNoticed)
	}
	mergeExtras(entry, s.Extra)
	return entry
}

func (s TeacherSchedule) ToAnyMap() map[string]any {
	entry := map[string]any{"teacher": s.Teacher}
	if len(s.Days) > 0 {
		days := make([]any, 0, len(s.Days))
		for _, day := range s.Days {
			days = append(days, day.teacherDayMap())
		}
		entry["days"] = days
	}
	if s.LastNoticed != 0 {
		entry["lastNoticedDay"] = float64(s.LastNoticed)
	}
	mergeExtras(entry, s.Extra)
	return entry
}

func (s GroupSchedule) Validate() error {
	for i := range s.Days {
		if err := s.Days[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s TeacherSchedule) Validate() error {
	for i := range s.Days {
		if err := s.Days[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}
