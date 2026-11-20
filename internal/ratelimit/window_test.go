package ratelimit

import (
	"testing"
	"time"
)

func TestWindowKeepsABurstInsideTheSecond(t *testing.T) {
	w := New(DefaultPerSecond)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	for i := 0; i < DefaultPerSecond; i++ {
		if delay := w.Reserve(now); delay != 0 {
			t.Fatalf("call %d inside the limit must not wait, got %s", i+1, delay)
		}
	}

	if delay := w.Reserve(now); delay <= 0 {
		t.Fatalf("the call over the limit must wait, got %s", delay)
	}

	if delay := w.Reserve(now.Add(time.Second)); delay != 0 {
		t.Fatalf("a second later the window must be free, got %s", delay)
	}
}

func TestWindowHonoursACustomLimit(t *testing.T) {
	w := New(2)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	if delay := w.Reserve(now); delay != 0 {
		t.Fatalf("first call: %s", delay)
	}
	if delay := w.Reserve(now.Add(10 * time.Millisecond)); delay != 0 {
		t.Fatalf("second call: %s", delay)
	}

	delay := w.Reserve(now.Add(20 * time.Millisecond))
	if want := 980 * time.Millisecond; delay != want {
		t.Fatalf("third call must wait %s, got %s", want, delay)
	}
}

func TestZeroWindowUsesTheDefault(t *testing.T) {
	var w Window
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	for i := 0; i < DefaultPerSecond; i++ {
		if delay := w.Reserve(now); delay != 0 {
			t.Fatalf("call %d: %s", i+1, delay)
		}
	}
	if delay := w.Reserve(now); delay <= 0 {
		t.Fatal("a zero window must fall back to the default limit")
	}
}
