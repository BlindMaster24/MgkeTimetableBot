package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	groups := map[string]any{
		"63": map[string]any{"group": "63"},
	}
	c.SetGroups(groups, "hash123")

	teachers := map[string]any{
		"Ivanov": map[string]any{"teacher": "Ivanov"},
	}
	c.SetTeachers(teachers, "hash456")

	c.SetTeam(map[string]string{"63": "63TP"}, []string{"teamhash"})

	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	c2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	g2 := c2.GetGroups()
	if len(g2) != 1 {
		t.Errorf("expected 1 group, got %d", len(g2))
	}
	if _, ok := g2["63"]; !ok {
		t.Error("expected group 63")
	}

	te := c2.GetTeachers()
	if len(te) != 1 {
		t.Errorf("expected 1 teacher, got %d", len(te))
	}

	team := c2.GetTeamNames()
	if team["63"] != "63TP" {
		t.Errorf("expected team name 63TP, got %s", team["63"])
	}
}

func TestCacheStats(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	c.RecordHit()
	c.RecordHit()
	c.RecordMiss()

	c.SetGroups(map[string]any{"A": 1, "B": 2}, "h")
	c.SetTeachers(map[string]any{"T1": 1}, "h2")

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.GroupsCount != 2 {
		t.Errorf("expected 2 groups, got %d", stats.GroupsCount)
	}
	if stats.TeachersCount != 1 {
		t.Errorf("expected 1 teacher, got %d", stats.TeachersCount)
	}
	if stats.GroupsHash != "h" {
		t.Errorf("expected hash h, got %s", stats.GroupsHash)
	}
}

func TestCacheConcurrent(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				c.SetGroups(map[string]any{string(rune('A' + n)): j}, "hash")
				c.GetGroups()
				c.RecordHit()
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := c.Stats()
	if stats.Hits != 1000 {
		t.Errorf("expected 1000 hits, got %d", stats.Hits)
	}
}

func TestCacheLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "groups.json"), []byte("not json"), 0644)

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	groups := c.GetGroups()
	if len(groups) != 0 {
		t.Error("expected empty groups after corrupt file")
	}

	if _, err := os.Stat(filepath.Join(dir, "groups.json")); err == nil {
		t.Error("expected corrupt file to be deleted")
	}
}

func TestCacheSetCalls(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	site := Schedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}},
		},
		Saturday: [][2][2]string{
			{{"09:00", "09:45"}},
		},
	}
	manual := Schedule{}

	c.SetCalls(site, manual, "site")

	calls := c.GetCalls()
	if calls.Active.Source != "site" {
		t.Errorf("expected source site, got %s", calls.Active.Source)
	}
	if len(calls.Active.Schedule.Weekdays) != 1 {
		t.Errorf("expected 1 weekday slot, got %d", len(calls.Active.Schedule.Weekdays))
	}
}

func TestCacheSuccessUpdate(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !c.Stats().SuccessUpdate {
		t.Error("expected successUpdate true by default")
	}

	c.SetSuccessUpdate(false)
	if c.Stats().SuccessUpdate {
		t.Error("expected successUpdate false after set")
	}
}

func TestCacheTeachersSaveLoad(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	c.SetTeachers(map[string]any{"T1": "data1", "T2": "data2"}, "thash")
	c.Save()

	c2, _ := New(dir)
	te := c2.GetTeachers()
	if len(te) != 2 {
		t.Errorf("expected 2 teachers, got %d", len(te))
	}
	if te["T1"] != "data1" {
		t.Errorf("expected teacher T1 data, got %v", te["T1"])
	}
}
