package reminder

import (
	"math/rand"
	"testing"
	"time"
)

func TestJitterWithoutSourceStaysStill(t *testing.T) {
	start := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	if got := Jitter(time.Minute, nil); got != 0 {
		t.Errorf("Jitter = %v, want 0 without source", got)
	}
	if got := ReminderTime(start, 10*time.Minute, time.Minute, nil); got != start.Add(-10*time.Minute) {
		t.Errorf("ReminderTime = %v, want start minus lead", got)
	}
}

func TestJitterWithoutWindowStaysStill(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	if got := Jitter(0, r); got != 0 {
		t.Errorf("Jitter = %v, want 0 without window", got)
	}
	if got := Jitter(-time.Second, r); got != 0 {
		t.Errorf("Jitter = %v, want 0 for negative window", got)
	}
}

func TestJitterIsDeterministicAndBounded(t *testing.T) {
	window := 5 * time.Minute
	first := Jitter(window, rand.New(rand.NewSource(7)))
	second := Jitter(window, rand.New(rand.NewSource(7)))
	if first != second {
		t.Fatalf("first = %v, second = %v, want identical", first, second)
	}
	if first < 0 || first >= window {
		t.Fatalf("Jitter = %v, want within [0, %v)", first, window)
	}
}

func TestReminderTimeAddsBoundedJitter(t *testing.T) {
	start := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	lead := 10 * time.Minute
	window := 5 * time.Minute
	got := ReminderTime(start, lead, window, rand.New(rand.NewSource(3)))
	base := start.Add(-lead)
	if got.Before(base) || !got.Before(base.Add(window)) {
		t.Fatalf("ReminderTime = %v, want within [%v, %v)", got, base, base.Add(window))
	}
}
