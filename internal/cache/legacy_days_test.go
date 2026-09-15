package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheNormalizesLegacyDayLabels(t *testing.T) {
	dir := t.TempDir()

	legacy := `{"timetable":{"100":{"group":"100","days":[{"day":"Понедельник, 31.08.2026","lessons":[]}]},"101":{"group":"101","days":[{"day":"Вторник, 01.09.2026","lessons":[]}]}},"update":1}`
	if err := os.WriteFile(filepath.Join(dir, "groups.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	teachers := `{"timetable":{"Иванов И.И.":{"teacher":"Иванов И.И.","days":[{"day":"Среда, 02.09.2026","lessons":[]}]}},"update":1}`
	if err := os.WriteFile(filepath.Join(dir, "teachers.json"), []byte(teachers), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertFirstDay(t, c.GetGroups(), "100", "31.08.2026")
	assertFirstDay(t, c.GetGroups(), "101", "01.09.2026")
	assertFirstDay(t, c.GetTeachers(), "Иванов И.И.", "02.09.2026")
}

func TestCacheKeepsPlainDates(t *testing.T) {
	dir := t.TempDir()

	groups := `{"timetable":{"100":{"group":"100","days":[{"day":"31.08.2026","lessons":[]}]}},"update":1}`
	if err := os.WriteFile(filepath.Join(dir, "groups.json"), []byte(groups), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertFirstDay(t, c.GetGroups(), "100", "31.08.2026")
}

func assertFirstDay(t *testing.T, entries map[string]any, value, want string) {
	t.Helper()

	record, ok := entries[value].(map[string]any)
	if !ok {
		t.Fatalf("%s: expected an entry", value)
	}
	days, ok := record["days"].([]any)
	if !ok || len(days) == 0 {
		t.Fatalf("%s: expected days", value)
	}
	day, _ := days[0].(map[string]any)
	if day["day"] != want {
		t.Errorf("%s: day = %v, want %v", value, day["day"], want)
	}
}
