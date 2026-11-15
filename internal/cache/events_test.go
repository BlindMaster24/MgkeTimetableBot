package cache

import "testing"

func TestLastNoticedUpdateKeepsAHandedOutViewIntact(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetGroups(t.Context(), map[string]any{
		"100": map[string]any{"group": "100", "days": []any{}},
	}, "groups-hash")
	c.SetTeachers(t.Context(), map[string]any{
		"Иванов И.И.": map[string]any{"teacher": "Иванов И.И.", "days": []any{}},
	}, "teachers-hash")

	groups := c.GetGroups()
	teachers := c.GetTeachers()

	c.SetLastNoticedDay(KindGroups, "100", 42)
	c.SetLastNoticedDay(KindTeachers, "Иванов И.И.", 43)

	if entry, _ := groups["100"].(map[string]any); entry["lastNoticedDay"] != nil {
		t.Fatalf("a group view handed out earlier was mutated: %v", entry["lastNoticedDay"])
	}
	if entry, _ := teachers["Иванов И.И."].(map[string]any); entry["lastNoticedDay"] != nil {
		t.Fatalf("a teacher view handed out earlier was mutated: %v", entry["lastNoticedDay"])
	}
	if got := c.LastNoticedDay(KindGroups, "100"); got != 42 {
		t.Fatalf("the stored group mark = %d, want 42", got)
	}
	if got := c.LastNoticedDay(KindTeachers, "Иванов И.И."); got != 43 {
		t.Fatalf("the stored teacher mark = %d, want 43", got)
	}
}
