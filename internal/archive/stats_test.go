package archive

import (
	"testing"
)

func TestStatsCountsRows(t *testing.T) {
	repo := newTestRepo(t)

	empty, err := repo.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if empty != (Stats{}) {
		t.Fatalf("empty stats = %+v, want zeros", empty)
	}

	groups := map[string]any{
		"100": map[string]any{
			"days": []any{
				map[string]any{"day": "07.09.2026", "lessons": []any{
					map[string]any{"lesson": "Математика", "cabinet": "101"},
				}},
				map[string]any{"day": "08.09.2026", "lessons": []any{
					map[string]any{"lesson": "Физика", "cabinet": "202"},
				}},
			},
		},
	}
	teachers := map[string]any{
		"Иванов И.И.": map[string]any{
			"days": []any{
				map[string]any{"day": "07.09.2026", "lessons": []any{
					map[string]any{"lesson": "Математика", "group": "100"},
				}},
			},
		},
	}
	if err := repo.SyncFromCache(groups, teachers); err != nil {
		t.Fatalf("sync: %v", err)
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 3 || stats.WithGroup != 2 || stats.WithTeacher != 1 {
		t.Fatalf("stats = %+v, want total 3 with 2 groups and 1 teacher", stats)
	}
}
