package cache

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func mergedCache(t *testing.T) *RaspCache {
	t.Helper()

	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func weekEntry(day string, lessons ...any) map[string]any {
	list := make([]any, 0, len(lessons))
	list = append(list, lessons...)
	return map[string]any{"days": []any{map[string]any{"day": day, "lessons": list}}}
}

func TestSetGroupsKeepsEntriesMissingFromTheNewDump(t *testing.T) {
	c := mergedCache(t)
	today := utils.SiteNow().Format("02.01.2006")

	c.SetGroups(t.Context(), map[string]any{
		"100": weekEntry(today, map[string]any{"lesson": "Математика"}),
		"200": weekEntry(today, map[string]any{"lesson": "Физика"}),
	}, "hash1")
	c.DrainEvents(t.Context())

	c.SetGroups(t.Context(), map[string]any{
		"100": weekEntry(today, map[string]any{"lesson": "История"}),
	}, "hash2")

	groups := c.GetGroups()
	if _, ok := groups["200"]; !ok {
		t.Fatal("a group missing from the new dump must keep its archived days")
	}
	entry, _ := groups["200"].(map[string]any)
	days, _ := entry["days"].([]any)
	if len(days) != 1 {
		t.Fatalf("kept days = %v, want the old one", days)
	}
}

func TestSetGroupsSkipsValuesThatAreNotMaps(t *testing.T) {
	c := mergedCache(t)
	today := utils.SiteNow().Format("02.01.2006")

	c.SetGroups(t.Context(), map[string]any{
		"100": "not an entry",
		"200": weekEntry(today, map[string]any{"lesson": "Математика"}),
	}, "hash")

	groups := c.GetGroups()
	if _, ok := groups["200"]; !ok {
		t.Fatal("a healthy entry must survive next to the broken one")
	}
	// The malformed value is stored verbatim (the stats count it), but it must
	// never be read as a day entry.
	if typed := c.Groups.Timetable; len(typed) != 1 {
		t.Fatalf("typed groups = %v, want only the healthy one", typed)
	}
}

func TestSetGroupsWithdrawsAnEntryFromAFutureWeek(t *testing.T) {
	c := mergedCache(t)
	future := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(utils.SiteNow()).Value() + 1)
	day := future.FirstDayDate().Format("02.01.2006")

	c.SetGroups(t.Context(), map[string]any{
		"63": weekEntry(day, map[string]any{"lesson": "Математика"}),
		"64": weekEntry(day, map[string]any{"lesson": "Физика"}),
	}, "hash1")
	c.DrainEvents(t.Context())

	// The site dropped the future week for group 64.
	today := utils.SiteNow().Format("02.01.2006")
	c.SetGroups(t.Context(), map[string]any{
		"63": weekEntry(day, map[string]any{"lesson": "Математика"}),
		"64": weekEntry(today, map[string]any{"lesson": "Физика"}),
	}, "hash2")

	var withdrawn *WeekEvent
	for _, event := range c.DrainEvents(t.Context()) {
		if event.Week != nil && event.Week.Withdrawn {
			withdrawn = event.Week
		}
	}
	if withdrawn == nil {
		t.Fatal("a future week that shrank must produce a withdrawal event")
	}
	if len(withdrawn.Entries) != 1 || withdrawn.Entries[0] != "64" {
		t.Fatalf("withdrawn entries = %v, want [64]", withdrawn.Entries)
	}
	if withdrawn.Kind != KindGroups {
		t.Fatalf("kind = %q, want groups", withdrawn.Kind)
	}
}
