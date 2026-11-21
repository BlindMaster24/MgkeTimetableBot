package google

import (
	"testing"
	"time"
)

func TestParseClockUsesTheMinskZone(t *testing.T) {
	day := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	got, err := parseClock(day, "10:00")
	if err != nil {
		t.Fatalf("parseClock: %v", err)
	}
	want := time.Date(2026, 9, 7, 10, 0, 0, 0, minsk)
	if !got.Equal(want) {
		t.Fatalf("parseClock = %s, want %s", got, want)
	}
	if _, err := parseClock(day, "9.00"); err == nil {
		t.Fatal("a malformed clock must be an error")
	}
}

func TestSubgroupPrefixAndValue(t *testing.T) {
	zero, two := 0, 2
	if got := subgroupPrefix(nil); got != "" {
		t.Fatalf("prefix(nil) = %q, want empty", got)
	}
	if got := subgroupPrefix(&zero); got != "" {
		t.Fatalf("prefix(0) = %q, want empty", got)
	}
	if got := subgroupPrefix(&two); got != "2. " {
		t.Fatalf("prefix(2) = %q, want '2. '", got)
	}
	if got := subgroupValue(nil); got != "" {
		t.Fatalf("value(nil) = %q, want empty", got)
	}
	if got := subgroupValue(&zero); got != "0" {
		t.Fatalf("value(0) = %q, want 0", got)
	}
	if got := subgroupValue(&two); got != "2" {
		t.Fatalf("value(2) = %q, want 2", got)
	}
}
