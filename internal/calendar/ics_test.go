package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var (
	weekdayCalls = [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
		{{"10:50", "11:35"}, {"11:55", "12:40"}},
	}
	saturdayCalls = [][2][2]string{
		{{"08:00", "08:45"}, {"08:55", "09:40"}},
	}
)

func fixedBuilder(weekdays, saturday [][2][2]string, weekNumber int) *ICSBuilder {
	b := NewICSBuilder(weekdays, saturday, weekNumber)
	b.now = time.Date(2025, 9, 1, 12, 30, 45, 0, time.Local)
	return b
}

func TestICSBuilderMatchesOldFormat(t *testing.T) {
	lessonType := "лекция"
	teacher := "Иванов А.А."
	cabinet := "101"

	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{
				Lesson:  "Математика",
				Type:    &lessonType,
				Teacher: &teacher,
				Cabinet: &cabinet,
			},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	if b.EventCount() != 1 {
		t.Fatalf("expected 1 event, got %d", b.EventCount())
	}

	ics := b.Build()
	lines := strings.Split(ics, "\r\n")

	head := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//MGKE Timetable Bot//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
	}
	for i, want := range head {
		if lines[i] != want {
			t.Errorf("line %d: want %q, got %q", i, want, lines[i])
		}
	}

	uid := lines[6]
	if !strings.HasPrefix(uid, "UID:") || len(strings.TrimPrefix(uid, "UID:")) != 64 {
		t.Errorf("expected sha256 uid, got %q", uid)
	}
	if lines[7] != "DTSTAMP:20250901T123045" {
		t.Errorf("unexpected dtstamp %q", lines[7])
	}
	if lines[8] != "DTSTART:20250901T090000" {
		t.Errorf("lesson must start at the first bell time, got %q", lines[8])
	}
	if lines[9] != "DTEND:20250901T104000" {
		t.Errorf("lesson must end at the second bell end, got %q", lines[9])
	}
	if lines[10] != "SUMMARY:Математика (лекция)" {
		t.Errorf("unexpected summary %q", lines[10])
	}
	if lines[11] != "DESCRIPTION:Преподаватель: Иванов А.А.\\nКабинет: 101" {
		t.Errorf("unexpected description %q", lines[11])
	}
	if lines[12] != "LOCATION:101" {
		t.Errorf("unexpected location %q", lines[12])
	}
	if lines[len(lines)-2] != "END:VEVENT" || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Errorf("unexpected tail %q", lines[len(lines)-2:])
	}
	if strings.Contains(ics, "VK:") || strings.Contains(ics, "Viber") {
		t.Error("ics must not carry messenger adverts")
	}
}

func TestICSBuilderSaturdayUsesSaturdayCalls(t *testing.T) {
	day := model.GroupDay{
		Day:     "06.09.2025",
		Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "История"}},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	ics := b.Build()
	if !strings.Contains(ics, "DTSTART:20250906T080000") {
		t.Errorf("saturday lesson must use the saturday bell schedule, got %s", ics)
	}
	if !strings.Contains(ics, "DTEND:20250906T094000") {
		t.Errorf("saturday lesson must end with the saturday schedule, got %s", ics)
	}
	if !strings.Contains(ics, "SUMMARY:История") {
		t.Errorf("summary must stay the bare lesson when it has no type, got %s", ics)
	}
	if strings.Contains(ics, "DESCRIPTION:") {
		t.Errorf("lesson without details must not get a description, got %s", ics)
	}
}

func TestICSBuilderElectivesBecomeSubgroupEvents(t *testing.T) {
	first, second := 1, 2
	cabinetA, cabinetB := "201", "202"

	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			[]*model.GroupLessonExplain{
				{Lesson: "Английский", Subgroup: &first, Cabinet: &cabinetA},
				{Lesson: "Английский", Subgroup: &second, Cabinet: &cabinetB},
			},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 8)
	b.AddGroupDay(day, "63")

	if b.EventCount() != 2 {
		t.Fatalf("expected one event per subgroup, got %d", b.EventCount())
	}

	ics := b.Build()
	if !strings.Contains(ics, "SUMMARY:Английский\\, подгр. 1") {
		t.Errorf("expected subgroup 1 in the summary, got %s", ics)
	}
	if !strings.Contains(ics, "SUMMARY:Английский\\, подгр. 2") {
		t.Errorf("expected subgroup 2 in the summary, got %s", ics)
	}
	if strings.Count(ics, "BEGIN:VEVENT") != 2 {
		t.Errorf("expected two VEVENT blocks, got %s", ics)
	}
}

func TestICSBuilderZeroSubgroupHasNoSuffix(t *testing.T) {
	zero := 0
	two := 2

	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			[]*model.GroupLessonExplain{
				{Lesson: "Физкультура", Subgroup: &zero},
				{Lesson: "Физкультура", Subgroup: &two},
			},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	ics := b.Build()
	if !strings.Contains(ics, "SUMMARY:Физкультура\r\n") {
		t.Errorf("subgroup 0 must not add a suffix, got %s", ics)
	}
	if !strings.Contains(ics, "SUMMARY:Физкультура\\, подгр. 2") {
		t.Errorf("non zero subgroup must add a suffix, got %s", ics)
	}
}

func TestICSBuilderTeacherDay(t *testing.T) {
	b := fixedBuilder(weekdayCalls, saturdayCalls, 3)

	day := model.TeacherDay{
		Day: "01.09.2025",
		Lessons: []model.TeacherLesson{
			{Lesson: "Физика", Group: "63"},
		},
	}

	b.AddTeacherDay(day, "Иванов")

	if b.EventCount() != 1 {
		t.Fatalf("expected 1 event, got %d", b.EventCount())
	}

	ics := b.Build()
	if !strings.Contains(ics, "SUMMARY:Физика") {
		t.Errorf("unexpected summary %s", ics)
	}
	if !strings.Contains(ics, "DESCRIPTION:Группа: 63") {
		t.Errorf("teacher events must name the group, got %s", ics)
	}
	if strings.Contains(ics, "LOCATION:") {
		t.Errorf("teacher event without a cabinet must not set a location, got %s", ics)
	}
}

func TestICSBuilderSkipsLessonsWithoutBellSlot(t *testing.T) {
	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика"},
			&model.GroupLessonExplain{Lesson: "Физика"},
		},
	}

	b := fixedBuilder(weekdayCalls[:1], saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	if b.EventCount() != 1 {
		t.Fatalf("lessons beyond the bell schedule must be skipped, got %d", b.EventCount())
	}
	if !strings.Contains(b.Build(), "SUMMARY:Математика") {
		t.Error("expected the first lesson to stay")
	}
}

func TestICSBuilderSkipsEmptyLessons(t *testing.T) {
	empty := ""
	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			nil,
			&model.GroupLessonExplain{Lesson: empty},
			[]*model.GroupLessonExplain{nil, {Lesson: empty}},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	if b.EventCount() != 0 {
		t.Errorf("expected no events, got %d", b.EventCount())
	}
	if b.Build() != strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//MGKE Timetable Bot//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"END:VCALENDAR",
	}, "\r\n") {
		t.Errorf("empty calendar must keep the old header, got %s", b.Build())
	}
}

func TestICSBuilderUIDDependsOnEventIdentity(t *testing.T) {
	cabinetA, cabinetB := "101", "102"

	build := func(cabinet string) string {
		day := model.GroupDay{
			Day:     "01.09.2025",
			Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "Математика", Cabinet: &cabinet}},
		}
		b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
		b.AddGroupDay(day, "63")
		for _, line := range strings.Split(b.Build(), "\r\n") {
			if strings.HasPrefix(line, "UID:") {
				return line
			}
		}
		return ""
	}

	first := build(cabinetA)
	if first == "" {
		t.Fatal("expected a uid")
	}
	if first != build(cabinetA) {
		t.Error("the same event must keep the same uid between builds")
	}
	if first == build(cabinetB) {
		t.Error("a different cabinet must produce a different uid")
	}
	if first == build("") {
		t.Error("a missing cabinet must produce a different uid")
	}
}

func TestICSBuilderSkipsInvalidDates(t *testing.T) {
	day := model.GroupDay{
		Day:     "not-a-date",
		Lessons: []model.GroupLesson{&model.GroupLessonExplain{Lesson: "Математика"}},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")
	b.AddTeacherDay(model.TeacherDay{
		Day:     "not-a-date",
		Lessons: []model.TeacherLesson{{Lesson: "Математика", Group: "63"}},
	}, "Иванов")

	if b.EventCount() != 0 {
		t.Errorf("expected no events for an unparsable date, got %d", b.EventCount())
	}
}

func TestICSBuilderEscapesText(t *testing.T) {
	lessonType := "лекция"
	teacher := "Иванов, А.А.; доцент"
	cabinet := "101/2"

	day := model.GroupDay{
		Day: "01.09.2025",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика", Type: &lessonType, Teacher: &teacher, Cabinet: &cabinet},
		},
	}

	b := fixedBuilder(weekdayCalls, saturdayCalls, 1)
	b.AddGroupDay(day, "63")

	ics := b.Build()
	if !strings.Contains(ics, "DESCRIPTION:Преподаватель: Иванов\\, А.А.\\; доцент\\nКабинет: 101/2") {
		t.Errorf("expected escaped commas and semicolons, got %s", ics)
	}
}
