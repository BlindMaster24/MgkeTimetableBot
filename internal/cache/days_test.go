package cache

import (
	"testing"
)

func TestTimetableDaysFindsToday(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(t.Context(), map[string]any{
		"63": map[string]any{"days": []any{
			map[string]any{"day": "07.09.2026", "lessons": []any{
				map[string]any{"lesson": "Math"},
			}},
		}},
	}, "hash1")
	c.SetTeachers(t.Context(), map[string]any{
		"Ivanov": map[string]any{"days": []any{
			map[string]any{"day": "07.09.2026", "lessons": []any{
				map[string]any{"lesson": "Math", "group": "63"},
			}},
		}},
	}, "hash2")

	groups := c.TimetableDays(KindGroups, "07.09.2026")
	if day, ok := groups["63"]; !ok || len(day.Lessons) != 1 || day.Lessons[0].Subject != "Math" {
		t.Fatalf("groups = %+v, want Math for 63", groups)
	}
	teachers := c.TimetableDays(KindTeachers, "07.09.2026")
	if day, ok := teachers["Ivanov"]; !ok || len(day.Lessons) != 1 {
		t.Fatalf("teachers = %+v, want one lesson for Ivanov", teachers)
	}
	if other := c.TimetableDays(KindGroups, "08.09.2026"); len(other) != 0 {
		t.Fatalf("other date = %+v, want none", other)
	}
	if unknown := c.TimetableDays("unknown", "07.09.2026"); len(unknown) != 0 {
		t.Fatalf("unknown kind = %+v, want none", unknown)
	}
}

func TestTimetableDaysEmptyCacheStaysSilent(t *testing.T) {
	empty := &RaspCache{}
	if days := empty.TimetableDays(KindGroups, "07.09.2026"); len(days) != 0 {
		t.Fatalf("groups = %+v, want none", days)
	}
	if days := empty.TimetableDays(KindTeachers, "07.09.2026"); len(days) != 0 {
		t.Fatalf("teachers = %+v, want none", days)
	}
}
