package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const offlineConfig = `telegram:
  token: "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
parser:
  endpoints:
    timetable_group: "http://127.0.0.1:1/groups"
    timetable_teacher: "http://127.0.0.1:1/teachers"
    bell_schedule: "http://127.0.0.1:1/calls"
    team:
      - "http://127.0.0.1:1/team"
`

func writeOfflineConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(offlineConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunReportsABrokenConfigInsteadOfPanicking(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"), &out, logger.New("error", nil))
	if err == nil {
		t.Fatal("a missing config file must be an error, not a panic")
	}
}

func TestRunCountsTheFailedSourcesOffline(t *testing.T) {
	var out bytes.Buffer
	issues, err := run(context.Background(), writeOfflineConfig(t), &out, logger.New("error", nil))
	if err != nil {
		t.Fatalf("an unreachable site is an audit finding, not an error: %v", err)
	}
	if issues < 3 {
		t.Fatalf("issues = %d, want at least one per unreachable source; output:\n%s", issues, out.String())
	}
	if strings.Contains(out.String(), "all invariants hold") {
		t.Fatalf("a failed audit must not claim that everything holds:\n%s", out.String())
	}
	if want := "FAILED with " + strconv.Itoa(issues) + " issues"; !strings.Contains(out.String(), want) {
		t.Errorf("output does not report %q:\n%s", want, out.String())
	}
	for _, want := range []string{"timetable error:", "team error:", "issue: active calls schedule is empty"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not mention %q:\n%s", want, out.String())
		}
	}
}
