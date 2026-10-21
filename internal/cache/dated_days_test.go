package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCacheFile(t *testing.T, dir, name, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCacheDropsUndatedDaysFromLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	writeCacheFile(t, dir, "groups.json", `{"timetable":{"100":{"group":"100","days":[{"day":"Понедельник","lessons":[]},{"day":"31.08.2026","lessons":[]}]}},"update":1}`)
	writeCacheFile(t, dir, "teachers.json", `{"timetable":{"Иванов И.И.":{"teacher":"Иванов И.И.","days":[{"day":"Вторник","lessons":[]},{"day":"01.09.2026","lessons":[]}]}},"update":1}`)

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	group := c.Groups.Timetable["100"]
	if len(group.Days) != 1 || group.Days[0].Date != "31.08.2026" {
		t.Fatalf("group days = %+v, want only the dated day", group.Days)
	}
	if err := group.Validate(); err != nil {
		t.Errorf("the typed group still carries an invalid day: %v", err)
	}

	teacher := c.Teachers.Timetable["Иванов И.И."]
	if len(teacher.Days) != 1 || teacher.Days[0].Date != "01.09.2026" {
		t.Fatalf("teacher days = %+v, want only the dated day", teacher.Days)
	}

	entry, ok := c.GetGroups()["100"].(map[string]any)
	if !ok {
		t.Fatalf("raw group view = %v", c.GetGroups()["100"])
	}
	if days, _ := entry["days"].([]any); len(days) != 1 {
		t.Fatalf("raw days = %+v, want the undated day removed", days)
	}
}

func TestSetGroupsDropsUndatedDays(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetGroups(t.Context(), map[string]any{
		"100": map[string]any{
			"group": "100",
			"days": []any{
				map[string]any{"day": "Понедельник", "lessons": []any{}},
				map[string]any{"day": "31.08.2026", "lessons": []any{}},
			},
		},
	}, "hash")

	entry, ok := c.GetGroups()["100"].(map[string]any)
	if !ok {
		t.Fatalf("raw group view = %v", c.GetGroups()["100"])
	}
	days, _ := entry["days"].([]any)
	if len(days) != 1 {
		t.Fatalf("raw days = %+v, want only the dated day", days)
	}
	if day, _ := days[0].(map[string]any); day["day"] != "31.08.2026" {
		t.Fatalf("kept day = %v, want 31.08.2026", days[0])
	}

	typed := c.Groups.Timetable["100"]
	if len(typed.Days) != 1 || typed.Days[0].Date != "31.08.2026" {
		t.Fatalf("typed days = %+v, want only the dated day", typed.Days)
	}
}
