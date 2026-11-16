package main

import (
	"testing"
)

func TestAuditEntriesFindsEveryKindOfTrouble(t *testing.T) {
	entries := map[string]any{
		"empty": map[string]any{},
		"bad-date": map[string]any{"days": []any{
			map[string]any{"day": "31.02.2026", "lessons": []any{}},
		}},
		"day-lessons": map[string]any{"days": []any{
			map[string]any{"day": "07.09.2026", "lessons": []any{}},
			map[string]any{"day": "08.09.2026", "lessons": []any{
				map[string]any{"lesson": "Математика(лек)", "group": "100"},
				map[string]any{"lesson": "Физика", "group": "100-2"},
				map[string]any{"lesson": "Химия", "group": "100"},
				[]any{map[string]any{"lesson": "Информатика"}},
				[]any{map[string]any{"lesson": "Биология", "subgroup": float64(1)}},
				"мусор",
			}},
		}},
	}

	result := auditEntries("group", entries)

	if result.Entries != 3 {
		t.Fatalf("entries = %d", result.Entries)
	}
	if result.EmptyDays != 1 {
		t.Fatalf("empty days = %d", result.EmptyDays)
	}
	if result.BadDates != 1 {
		t.Fatalf("bad dates = %d", result.BadDates)
	}
	if result.LessonlessDays != 1 {
		t.Fatalf("lessonless days = %d", result.LessonlessDays)
	}
	// (лек) in the subject, the dash in the group, the subgroup without a
	// number and the foreign lesson shape.
	if result.BadLessons != 4 {
		t.Fatalf("bad lessons = %d: %v", result.BadLessons, result.Notes)
	}
	if result.Issues() != 6 {
		t.Fatalf("issues = %d", result.Issues())
	}
	if result.DayCounts[2] != 1 || result.DayCounts[1] != 1 {
		t.Fatalf("day counts = %v", result.DayCounts)
	}
	if result.EmptyWeekdays["Monday"] != 1 {
		t.Fatalf("empty weekdays = %v", result.EmptyWeekdays)
	}
}

func TestAuditEntriesIsQuietOnAHealthyDump(t *testing.T) {
	entries := map[string]any{
		"100": map[string]any{"days": []any{
			map[string]any{"day": "07.09.2026", "lessons": []any{
				map[string]any{"lesson": "Математика", "group": "100"},
				[]any{map[string]any{"lesson": "Физика", "subgroup": float64(1)}},
			}},
		}},
	}

	result := auditEntries("group", entries)
	if result.Issues() != 0 {
		t.Fatalf("a healthy dump must pass, got %+v", result)
	}
	if len(result.Notes) != 0 {
		t.Fatalf("a healthy dump must print nothing, got %v", result.Notes)
	}
}
