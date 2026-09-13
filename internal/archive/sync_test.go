package archive

import (
	"path/filepath"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestSyncFromCacheEmpty(t *testing.T) {
	repo := newTestRepo(t)
	if err := repo.SyncFromCache(map[string]any{}, map[string]any{}); err != nil {
		t.Fatalf("sync empty cache: %v", err)
	}
	bounds, err := repo.DayIndexBounds()
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Max != 0 || bounds.Min != 0 {
		t.Fatalf("expected empty bounds, got min=%d max=%d", bounds.Min, bounds.Max)
	}
}

func TestSyncFromCachePopulatesArchive(t *testing.T) {
	repo := newTestRepo(t)

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

	days, err := repo.GroupDaysByRange(DateToDayIndex("07.09.2026"), DateToDayIndex("08.09.2026"), "100")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("expected 2 group days, got %d", len(days))
	}
	if days[0].Day != "07.09.2026" || days[1].Day != "08.09.2026" {
		t.Fatalf("unexpected days: %s, %s", days[0].Day, days[1].Day)
	}
	if len(days[0].Lessons) != 1 {
		t.Fatalf("expected 1 lesson on first day, got %d", len(days[0].Lessons))
	}

	teacherDays, err := repo.TeacherDaysByRange(DateToDayIndex("07.09.2026"), DateToDayIndex("07.09.2026"), "Иванов И.И.")
	if err != nil {
		t.Fatal(err)
	}
	if len(teacherDays) != 1 {
		t.Fatalf("expected 1 teacher day, got %d", len(teacherDays))
	}
}

func TestSyncFromCacheSkipsWhenNotNewer(t *testing.T) {
	repo := newTestRepo(t)

	entries := []AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": "07.09.2026", "lessons": []any{}}},
	}
	if err := repo.AppendDays(entries); err != nil {
		t.Fatal(err)
	}

	groups := map[string]any{
		"100": map[string]any{
			"days": []any{map[string]any{"day": "07.09.2026", "lessons": []any{}}},
		},
	}
	if err := repo.SyncFromCache(groups, map[string]any{}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	bounds, err := repo.DayIndexBounds()
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Max != DateToDayIndex("07.09.2026") {
		t.Fatalf("unexpected max day bound: %d", bounds.Max)
	}
}

func strPtr(s string) *string { return &s }

func TestGroupDayRoundTrip(t *testing.T) {
	repo := newTestRepo(t)

	day := &model.GroupDay{
		Day: "07.09.2026",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика", Cabinet: strPtr("101")},
		},
	}
	if err := repo.AppendDays([]AppendDay{{Type: "group", Value: "100", Day: day}}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GroupDay(DateToDayIndex("07.09.2026"), "100")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected day to exist")
	}
	if got.Day != "07.09.2026" || len(got.Lessons) != 1 {
		t.Fatalf("unexpected day: %+v", got)
	}
}

func TestTeacherDayRoundTrip(t *testing.T) {
	repo := newTestRepo(t)

	day := &model.TeacherDay{
		Day: "07.09.2026",
		Lessons: []model.TeacherLesson{
			&model.TeacherLessonExplain{Lesson: "Математика", Group: "100"},
		},
	}
	if err := repo.AppendDays([]AppendDay{{Type: "teacher", Value: "Иванов И.И.", Day: day}}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.TeacherDay(DateToDayIndex("07.09.2026"), "Иванов И.И.")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected day to exist")
	}
	if got.Day != "07.09.2026" || len(got.Lessons) != 1 {
		t.Fatalf("unexpected day: %+v", got)
	}
}
