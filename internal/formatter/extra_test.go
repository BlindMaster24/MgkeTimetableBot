package formatter

import (
	"strings"
	"testing"
)

func TestLabelKeysCoversTheOptions(t *testing.T) {
	keys := LabelKeys()
	if len(keys) == 0 {
		t.Fatal("LabelKeys must list the labels the formatter asks for")
	}
	seen := map[string]bool{}
	opts := FormatOptions{Labels: map[string]string{}}
	for _, key := range keys {
		if key == "" || seen[key] {
			t.Fatalf("label keys = %v, want no blanks or duplicates", keys)
		}
		seen[key] = true
		opts.Labels[key] = key + ":%s"
	}
	if !strings.Contains(opts.formatLabel("no_timetable", "x"), ":x") {
		t.Fatal("every listed key must be a real label the formatter formats")
	}
}

func TestFormatSecondsWritesEveryUnit(t *testing.T) {
	if got := FormatSeconds(-5); got != "0" {
		t.Fatalf("a negative duration = %q, want zero", got)
	}
	if got := FormatSeconds(65); got != "1 5" {
		t.Fatalf("FormatSeconds(65) = %q, want '1 5'", got)
	}
	if got := formatSeconds(65, []string{"s", "m", "h", "d", "y"}); got != "1 m 5 s" {
		t.Fatalf("formatSeconds(65, suffixes) = %q, want '1 m 5 s'", got)
	}
	if got := formatSeconds(3665, []string{"s", "m", "h", "d", "y"}); got != "1 h 1 m 5 s" {
		t.Fatalf("formatSeconds(3665) = %q, want '1 h 1 m 5 s'", got)
	}
	if got := formatSeconds(86400, []string{"s", "m", "h", "d", "y"}); !strings.HasPrefix(got, "1 d") {
		t.Fatalf("formatSeconds(86400) = %q, want the day unit first", got)
	}
}

func TestOldDashFillsTheEmptyCabinet(t *testing.T) {
	if got := oldDash(""); got != "-" {
		t.Fatalf("oldDash(\"\") = %q, want a dash", got)
	}
	if got := oldDash("3-209"); got != "3-209" {
		t.Fatalf("oldDash keeps a real cabinet, got %q", got)
	}
}
