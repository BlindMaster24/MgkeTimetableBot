package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsAMissingConfig(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"-config", filepath.Join(t.TempDir(), "missing.yaml")}, &out)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for a missing config", code)
	}
}

func TestRunRejectsAnUnknownFlag(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"-nope"}, &out); code != 2 {
		t.Fatalf("exit code = %d, want 2 for a broken flag", code)
	}
	if !strings.Contains(out.String(), "flag provided but not defined") {
		t.Fatalf("the flag set must explain the mistake, got %q", out.String())
	}
}
