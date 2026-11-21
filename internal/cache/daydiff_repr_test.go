package cache

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

// parserDay is the shape the parser hands over: lesson/cabinet keys and a
// nested array for a pair of subgroups.
func parserDay(date string) map[string]any {
	return map[string]any{
		"day": date,
		"lessons": []any{
			map[string]any{"lesson": "География", "type": "Лек", "teacher": "Поклад Т. И.", "cabinet": "603", "comment": nil},
			[]any{
				map[string]any{"lesson": "ФЗКиЗ", "subgroup": 1.0, "type": "Лек", "teacher": "Прокопович М. Е.", "cabinet": nil, "comment": nil},
				map[string]any{"lesson": "ФЗКиЗ", "subgroup": 2.0, "type": "Лек", "teacher": "Усикова Л. Н.", "cabinet": nil, "comment": nil},
			},
		},
	}
}

// typedDay is the same day as the cache file stores it: one row per subgroup
// and the typed key names.
func typedDay(date string) map[string]any {
	return map[string]any{
		"day": date,
		"lessons": []any{
			map[string]any{"num": 1, "subject": "География", "type": "Лек", "teacher": "Поклад Т. И.", "room": "603"},
			map[string]any{"num": 2, "subject": "ФЗКиЗ", "subgroup": 1, "type": "Лек", "teacher": "Прокопович М. Е."},
			map[string]any{"num": 2, "subject": "ФЗКиЗ", "subgroup": 2, "type": "Лек", "teacher": "Усикова Л. Н."},
		},
	}
}

func TestTheSameDayInTwoShapesIsNotAChange(t *testing.T) {
	date := utils.SiteNow().AddDate(0, 0, 1).Format("02.01.2006")

	if dayLessonsChanged(typedDay(date), parserDay(date)) {
		t.Fatal("a day stored by the cache must equal the parser's own shape")
	}
}

func TestARealChangeStillCounts(t *testing.T) {
	date := utils.SiteNow().AddDate(0, 0, 1).Format("02.01.2006")

	changed := parserDay(date)
	changed["lessons"].([]any)[0].(map[string]any)["teacher"] = "Другой П. П."
	if !dayLessonsChanged(typedDay(date), changed) {
		t.Fatal("a different teacher must count as a change")
	}

	moved := parserDay(date)
	moved["lessons"] = []any{
		map[string]any{"lesson": "География", "type": "Лек", "teacher": "Поклад Т. И.", "cabinet": "603", "comment": nil},
	}
	if !dayLessonsChanged(typedDay(date), moved) {
		t.Fatal("a lost subgroup pair must count as a change")
	}
}

func TestARestartDoesNotReportAChange(t *testing.T) {
	date := utils.SiteNow().Format("02.01.2006")
	ctx := context.Background()
	dir := t.TempDir()

	entries := func() map[string]any {
		return map[string]any{"100": map[string]any{"group": "100", "days": []any{parserDay(date)}}}
	}

	first, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	first.SetGroups(ctx, entries(), "hash")
	if err := first.Save(ctx); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	second, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	second.SetGroups(ctx, entries(), "hash")
	if events := second.DrainEvents(ctx); len(events) != 0 {
		t.Fatalf("a restart re-read the same day as %d change events: %+v", len(events), events)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the check took %s", elapsed)
	}
}

func TestSavedCacheKeepsTheParserShape(t *testing.T) {
	date := utils.SiteNow().Format("02.01.2006")
	ctx := context.Background()
	dir := t.TempDir()

	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(ctx, map[string]any{"100": map[string]any{"group": "100", "days": []any{parserDay(date)}}}, "hash")
	if err := c.Save(ctx); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "groups.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{`"lesson": "География"`, `"cabinet": "603"`} {
		if !bytes.Contains(raw, []byte(wanted)) {
			t.Fatalf("the cache file must keep the parser shape, %q is missing:\n%s", wanted, raw)
		}
	}
	for _, unwanted := range []string{`"subject":`, `"room":`} {
		if bytes.Contains(raw, []byte(unwanted)) {
			t.Fatalf("the cache file still holds the typed view (%q):\n%s", unwanted, raw)
		}
	}
}
