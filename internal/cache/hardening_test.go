package cache

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(t.Context(), map[string]any{
		"100": map[string]any{"group": "100", "days": []any{
			map[string]any{"day": "07.09.2026", "lessons": []any{
				map[string]any{"lesson": "Математика"},
			}},
		}},
	}, "hash")
	if err := c.Save(t.Context()); err != nil {
		t.Fatal(err)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("save left temp files: %v", leftovers)
	}
	for _, name := range []string{"groups.json", "teachers.json", "team.json", "calls.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
			t.Errorf("%s mode = %o, want 644", name, info.Mode().Perm())
		}
	}

	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.GetGroups()) != 1 {
		t.Errorf("reloaded groups = %d, want 1", len(reloaded.GetGroups()))
	}
}

func TestCancelledContextSkipsCacheWork(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.SetGroups(ctx, map[string]any{"100": map[string]any{"group": "100"}}, "hash")
	if len(c.GetGroups()) != 0 {
		t.Errorf("cancelled SetGroups stored %d entries, want 0", len(c.GetGroups()))
	}
	if err := c.Save(ctx); err == nil {
		t.Error("cancelled Save() = nil, want the context error")
	} else if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("cancelled Save() = %v, want context canceled", err)
	}
	if events := c.DrainEvents(ctx); len(events) != 0 {
		t.Errorf("cancelled DrainEvents() = %d events, want none", len(events))
	}
	if changes := c.DrainDayChanges(ctx); len(changes) != 0 {
		t.Errorf("cancelled DrainDayChanges() = %d changes, want none", len(changes))
	}
}

func TestConcurrentCacheUse(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := string(rune('A' + n))
			for j := 0; j < 50; j++ {
				c.SetGroups(ctx, map[string]any{name: map[string]any{"group": name}}, "hash")
				c.SetCallsPreferSite(j%2 == 0)
				c.GetGroups()
				c.DrainEvents(ctx)
				if err := c.Save(ctx); err != nil {
					t.Errorf("concurrent Save() = %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.GetGroups()) == 0 {
		t.Error("concurrent use left no groups behind")
	}
}
