package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, dir, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	target := "groups.json"
	if name == "legacy_teachers.json" {
		target = "teachers.json"
	}
	if err := os.WriteFile(filepath.Join(dir, target), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func marshalValue(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLegacyFilesLoadIntoTypedSchedules(t *testing.T) {
	dir := t.TempDir()
	loadFixture(t, dir, "legacy_groups.json")
	loadFixture(t, dir, "legacy_teachers.json")

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	for name, schedule := range c.Groups.Timetable {
		if schedule.Group == "" {
			t.Errorf("group %q lost its name", name)
		}
		if err := schedule.Validate(); err != nil {
			t.Errorf("group %q: %v", name, err)
		}
	}
	for name, schedule := range c.Teachers.Timetable {
		if schedule.Teacher == "" {
			t.Errorf("teacher %q lost its name", name)
		}
		if err := schedule.Validate(); err != nil {
			t.Errorf("teacher %q: %v", name, err)
		}
	}

	slot := c.Groups.Timetable["100"].Days[0].Lessons
	nums := []int{slot[0].Num, slot[1].Num, slot[2].Num}
	if nums[0] != 1 || nums[1] != 2 || nums[2] != 2 {
		t.Errorf("slot numbers = %v, want [1 2 2]", nums)
	}
	if slot[0].Extra["time"] != "09:00 - 09:45" {
		t.Errorf("extras = %+v, want the time key kept", slot[0].Extra)
	}
	if c.Groups.Timetable["100"].LastNoticed != 3 {
		t.Errorf("lastNoticed = %d, want 3", c.Groups.Timetable["100"].LastNoticed)
	}
	if c.Teachers.Timetable["Иванов И.И."].Days[0].Lessons[0].Group != "100" {
		t.Errorf("teacher lesson lost its group: %+v", c.Teachers.Timetable["Иванов И.И."].Days[0].Lessons[0])
	}
}

func TestLegacyFilesKeepTheirVisibleShape(t *testing.T) {
	dir := t.TempDir()
	loadFixture(t, dir, "legacy_groups.json")
	loadFixture(t, dir, "legacy_teachers.json")

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	groups := c.GetGroups()
	entry, ok := groups["100"].(map[string]any)
	if !ok {
		t.Fatalf("groups[100] = %v, want an entry", groups["100"])
	}
	days, _ := entry["days"].([]any)
	if len(days) != 2 {
		t.Fatalf("group 100 days = %+v, want 2", entry)
	}
	first, _ := days[0].(map[string]any)
	lessons, _ := first["lessons"].([]any)
	if len(lessons) != 2 {
		t.Fatalf("first day lessons = %+v, want single plus subgroup array", first)
	}
	array, ok := lessons[1].([]any)
	if !ok || len(array) != 2 {
		t.Fatalf("second slot = %+v, want the subgroup array", lessons[1])
	}
	single, _ := lessons[0].(map[string]any)
	if single["lesson"] != "Математика" || single["time"] != "09:00 - 09:45" {
		t.Errorf("single lesson = %v, want the explain shape with extras", single)
	}
	if _, ok := single["num"]; ok {
		t.Errorf("typed fields leaked into the view: %v", single)
	}
	if _, ok := single["subject"]; ok {
		t.Errorf("typed fields leaked into the view: %v", single)
	}

	secondEntry, _ := groups["101"].(map[string]any)
	secondDays, _ := secondEntry["days"].([]any)
	secondDay, _ := secondDays[0].(map[string]any)
	if secondDay["day"] != "31.08.2026" {
		t.Errorf("legacy label survived as %v, want 31.08.2026", secondDay["day"])
	}

	teachers := c.GetTeachers()
	teacher, _ := teachers["Иванов И.И."].(map[string]any)
	teacherDays, _ := teacher["days"].([]any)
	teacherDay, _ := teacherDays[0].(map[string]any)
	teacherLessons, _ := teacherDay["lessons"].([]any)
	teacherLesson, _ := teacherLessons[0].(map[string]any)
	if teacherLesson["group"] != "100" || teacherLesson["cabinet"] != "101" {
		t.Errorf("teacher lesson = %v, want group and cabinet", teacherLesson)
	}
}

func TestTypedSaveReloadsStably(t *testing.T) {
	dir := t.TempDir()
	loadFixture(t, dir, "legacy_groups.json")
	loadFixture(t, dir, "legacy_teachers.json")

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	beforeGroups := marshalValue(t, c.GetGroups())
	beforeTeachers := marshalValue(t, c.GetTeachers())

	if err := c.Save(t.Context()); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := marshalValue(t, reloaded.GetGroups()); got != beforeGroups {
		t.Errorf("groups drifted across save:\n got: %s\nwant: %s", got, beforeGroups)
	}
	if got := marshalValue(t, reloaded.GetTeachers()); got != beforeTeachers {
		t.Errorf("teachers drifted across save:\n got: %s\nwant: %s", got, beforeTeachers)
	}
	for name, schedule := range reloaded.Groups.Timetable {
		if err := schedule.Validate(); err != nil {
			t.Errorf("reloaded group %q: %v", name, err)
		}
	}
}
