package timetable

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func marshalJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func parseRaw(t *testing.T, data string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertGroupRoundTrip(t *testing.T, data string) GroupSchedule {
	t.Helper()
	schedule, err := GroupScheduleFromJSON([]byte(data))
	if err != nil {
		t.Fatalf("GroupScheduleFromJSON() = %v", err)
	}
	if err := schedule.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if got, want := marshalJSON(t, schedule.ToAnyMap()), marshalJSON(t, parseRaw(t, data)); got != want {
		t.Errorf("round trip drifted:\n got: %s\nwant: %s", got, want)
	}
	return schedule
}

func TestGroupRoundTripKeepsEveryShape(t *testing.T) {
	assertGroupRoundTrip(t, `{"group":"100","lastNoticedDay":5,"days":[
		{"day":"07.09.2026","lessons":[
			{"lesson":"Математика","type":"Лек","teacher":"Иванов И.И.","cabinet":"101"},
			[{"subgroup":1,"lesson":"Английский","teacher":"А","cabinet":"10"},{"subgroup":2,"lesson":"Английский","teacher":"Б","cabinet":"20"}]
		]},
		{"day":"08.09.2026","lessons":[]}
	]}`)
}

func TestGroupRoundTripKeepsUnknownKeys(t *testing.T) {
	schedule := assertGroupRoundTrip(t, `{"group":"100","days":[
		{"day":"07.09.2026","lessons":[{"lesson":"Математика","time":"08:00 - 08:45"}]}
	]}`)
	if schedule.Days[0].Lessons[0].Extra["time"] != "08:00 - 08:45" {
		t.Errorf("extras = %+v, want the time key", schedule.Days[0].Lessons[0].Extra)
	}
}

func TestGroupRoundTripNormalizesLegacyLabels(t *testing.T) {
	schedule, err := GroupScheduleFromJSON([]byte(`{"group":"100","days":[{"day":"Понедельник, 31.08.2026","lessons":[]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if schedule.Days[0].Date != "31.08.2026" {
		t.Errorf("date = %q, want 31.08.2026", schedule.Days[0].Date)
	}
}

func TestGroupNumberingFollowsSlotPositions(t *testing.T) {
	schedule, err := GroupScheduleFromJSON([]byte(`{"group":"100","days":[{"day":"07.09.2026","lessons":[
		{"lesson":"А"},
		[{"lesson":"Б1"},{"lesson":"Б2"}],
		{"lesson":"В"}
	]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	nums := []int{}
	for _, lesson := range schedule.Days[0].Lessons {
		nums = append(nums, lesson.Num)
	}
	want := []int{1, 2, 2, 3}
	if len(nums) != len(want) {
		t.Fatalf("nums = %v, want %v", nums, want)
	}
	for i := range want {
		if nums[i] != want[i] {
			t.Fatalf("nums = %v, want %v", nums, want)
		}
	}
	if schedule.Days[0].Lessons[1].Subgroup != 0 || schedule.Days[0].Lessons[2].Subgroup != 0 {
		t.Errorf("array lessons without subgroup marks must default to 0: %+v", schedule.Days[0].Lessons[1:3])
	}
}

func TestTeacherRoundTripKeepsGroupRefs(t *testing.T) {
	data := `{"teacher":"Иванов И.И.","days":[{"day":"07.09.2026","lessons":[
		{"lesson":"Математика","type":"Лек","subgroup":1,"group":"100","cabinet":"101"}
	]}]}`
	schedule, err := TeacherScheduleFromJSON([]byte(data))
	if err != nil {
		t.Fatalf("TeacherScheduleFromJSON() = %v", err)
	}
	if err := schedule.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if got, want := marshalJSON(t, schedule.ToAnyMap()), marshalJSON(t, parseRaw(t, data)); got != want {
		t.Errorf("round trip drifted:\n got: %s\nwant: %s", got, want)
	}
	if schedule.Days[0].Lessons[0].Group != "100" {
		t.Errorf("group = %q, want 100", schedule.Days[0].Lessons[0].Group)
	}
}

func TestScheduleFromJSONRejectsGarbage(t *testing.T) {
	for _, data := range []string{
		`not json`,
		`{"group":"100","days":"yesterday"}`,
		`{"group":"100","days":[42]}`,
		`{"group":"100","days":[{"day":"07.09.2026","lessons":[42]}]}`,
	} {
		if _, err := GroupScheduleFromJSON([]byte(data)); err == nil {
			t.Errorf("GroupScheduleFromJSON(%s) = nil, want an error", data)
		}
	}
	if _, err := TeacherScheduleFromJSON([]byte(`{"teacher":"X","days":[42]}`)); err == nil {
		t.Error("TeacherScheduleFromJSON() = nil, want an error")
	}
}

func TestFromModelGroupStructs(t *testing.T) {
	subgroup := 1
	lessonType := "Лек"
	teacher := "Иванов И.И."
	cabinet := "101"
	group := &model.Group{
		Group:       "100",
		LastNoticed: 7,
		Days: []model.GroupDay{{Day: "07.09.2026", Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика", Type: &lessonType, Teacher: &teacher, Cabinet: &cabinet},
			[]*model.GroupLessonExplain{
				{Subgroup: &subgroup, Lesson: "Английский"},
				{Lesson: "Немецкий"},
			},
		}}},
	}
	schedule, err := FromModelGroup(group)
	if err != nil {
		t.Fatalf("FromModelGroup() = %v", err)
	}
	if schedule.Group != "100" || schedule.LastNoticed != 7 {
		t.Errorf("schedule = %+v, want name 100 and notice 7", schedule)
	}
	if len(schedule.Days[0].Lessons) != 3 {
		t.Fatalf("lessons = %+v, want 3", schedule.Days[0].Lessons)
	}
	if schedule.Days[0].Lessons[0].Num != 1 || schedule.Days[0].Lessons[2].Num != 2 {
		t.Errorf("nums = %d, %d, want 1 and 2",
			schedule.Days[0].Lessons[0].Num, schedule.Days[0].Lessons[2].Num)
	}
	if schedule.Days[0].Lessons[1].Subgroup != 1 {
		t.Errorf("subgroup = %d, want 1", schedule.Days[0].Lessons[1].Subgroup)
	}
	if err := schedule.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	raw := schedule.ToAnyMap()
	lessons, _ := raw["days"].([]any)[0].(map[string]any)["lessons"].([]any)
	if len(lessons) != 2 {
		t.Fatalf("reverse lessons = %+v, want 2 slots", lessons)
	}
	array, ok := lessons[1].([]any)
	if !ok || len(array) != 2 {
		t.Errorf("second slot = %+v, want the subgroup array back", lessons[1])
	}
	if sub, ok := array[0].(map[string]any)["subgroup"].(float64); !ok || sub != 1 {
		t.Errorf("subgroup in reverse map = %v, want float64 1", array[0])
	}
	if !strings.Contains(marshalJSON(t, raw), `"lastNoticedDay":7`) {
		t.Errorf("reverse map lost lastNoticedDay: %s", marshalJSON(t, raw))
	}
}

func TestFromModelTeacherStructs(t *testing.T) {
	teacher := &model.Teacher{
		Teacher: "Иванов И.И.",
		Days: []model.TeacherDay{{Day: "07.09.2026", Lessons: []model.TeacherLesson{
			{Lesson: "Математика", Group: "100"},
		}}},
	}
	schedule, err := FromModelTeacher(teacher)
	if err != nil {
		t.Fatalf("FromModelTeacher() = %v", err)
	}
	if schedule.Days[0].Lessons[0].Group != "100" || schedule.Days[0].Lessons[0].Num != 1 {
		t.Errorf("lesson = %+v, want group 100 num 1", schedule.Days[0].Lessons[0])
	}
}
