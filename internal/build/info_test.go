package build

import (
	"runtime"
	"strings"
	"testing"
)

func TestNewFillsMissingBuildFields(t *testing.T) {
	info := New("", "  ", "")

	for name, got := range map[string]string{
		"version": info.Version,
		"commit":  info.Commit,
		"date":    info.Date,
	} {
		if got != unknownVersion && got != unknownCommit && got != unknownDate {
			t.Fatalf("%s = %q, want one of the unknown markers", name, got)
		}
	}
	if info.Version != unknownVersion || info.Commit != unknownCommit || info.Date != unknownDate {
		t.Fatalf("info = %+v", info)
	}
	if info.Go != runtime.Version() || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		t.Fatalf("the runtime fields must come from the toolchain: %+v", info)
	}
}

func TestShortCommitKeepsTheFirstSevenCharacters(t *testing.T) {
	cases := map[string]string{
		"abcdef1234567890": "abcdef1",
		"abcdef1":          "abcdef1",
		"abc":              "abc",
		unknownCommit:      unknownCommit,
		" abcdef1234567 ":  "abcdef1",
	}
	for commit, want := range cases {
		if got := (Info{Commit: commit}).ShortCommit(); got != want {
			t.Errorf("ShortCommit(%q) = %q, want %q", commit, got, want)
		}
	}
}

func TestSummaryAndBuiltAt(t *testing.T) {
	info := Info{Version: "1.2.3", Commit: "abcdef1234567890", Date: "2026-11-16T12:00:00Z"}
	if got, want := info.Summary(), "1.2.3 (abcdef1, 2026-11-16T12:00:00Z)"; got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
	if got := info.BuiltAt(); got != "2026-11-16T12:00:00Z" {
		t.Fatalf("BuiltAt() = %q", got)
	}
	if got := (Info{Date: "not a date"}).BuiltAt(); got != "" {
		t.Fatalf("an unparsable date must render empty, got %q", got)
	}
	if got := (Info{Date: "2026-11-16T15:00:00+03:00"}).BuiltAt(); !strings.HasSuffix(got, "Z") {
		t.Fatalf("BuiltAt must be UTC, got %q", got)
	}
}
