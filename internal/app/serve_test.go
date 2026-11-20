package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServeStopsOnCancelAndSavesTheCache(t *testing.T) {
	r := newParseRuntime(t)
	r.cache.SetGroups(t.Context(), map[string]any{
		"100": map[string]any{"days": []any{}},
	}, "hash")

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	done := make(chan int, 1)
	go func() { done <- r.serve(ctx) }()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve returned %d, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop after the context was cancelled")
	}

	if _, err := os.Stat(filepath.Join(r.cfg.ResolvedCacheDir(), "groups.json")); err != nil {
		t.Fatalf("shutdown did not save the cache: %v", err)
	}
}
