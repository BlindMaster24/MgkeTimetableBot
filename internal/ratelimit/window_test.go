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

func TestWaitReturnsAtOnceWhenTheWindowHasRoom(t *testing.T) {
	w := New(2)
	started := time.Now()
	w.Wait()
	w.Wait()
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("two free slots must not sleep, took %s", elapsed)
	}
}

func TestWaitSleepsUntilTheOldestSlotLeavesTheWindow(t *testing.T) {
	w := New(1)
	w.Reserve(time.Now())

	started := time.Now()
	w.Wait()
	elapsed := time.Since(started)

	if elapsed < 900*time.Millisecond {
		t.Fatalf("Wait returned after %s, the window was still full", elapsed)
	}
	if elapsed > 2500*time.Millisecond {
		t.Fatalf("Wait slept for %s, it must wake once the slot expires", elapsed)
	}
}
