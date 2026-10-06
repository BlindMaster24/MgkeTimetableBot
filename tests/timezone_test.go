package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var processTimezoneAssignment = regexp.MustCompile(`\btime\.Local\s*=[^=]`)

var reminderFixtureFile = filepath.Join("..", "internal", "notification", "reminders_test.go")

func TestReminderFixturesReadTheClockOnlyInTheirAnchor(t *testing.T) {
	data, err := os.ReadFile(reminderFixtureFile)
	if err != nil {
		t.Fatalf("read %s: %v", reminderFixtureFile, err)
	}

	lines := strings.Split(string(data), "\n")
	start, end := clockAnchorRange(lines)
	if start < 0 {
		t.Fatalf("%s lost its reminderMoment anchor; keep one named anchor so fixtures cannot drift with the hour", reminderFixtureFile)
	}

	anchored := false
	for number, line := range lines {
		if !strings.Contains(line, "time.Now(") {
			continue
		}
		if number >= start && number <= end {
			anchored = true
			continue
		}
		t.Errorf("%s:%d reads the clock outside reminderMoment; a fixture seeded from the raw clock goes silent when the suite runs near midnight", reminderFixtureFile, number+1)
	}
	if !anchored {
		t.Errorf("%s no longer reads the clock inside reminderMoment; the anchor must stay the single place that touches it", reminderFixtureFile)
	}
}

func clockAnchorRange(lines []string) (int, int) {
	start := -1
	for number, line := range lines {
		if start < 0 {
			if strings.HasPrefix(line, "func reminderMoment(") {
				start = number
			}
			continue
		}
		if line == "}" {
			return start, number
		}
	}
	return -1, -1
}

func TestSourcesNeverRewriteTheProcessTimezone(t *testing.T) {
	roots := []string{
		filepath.Join("..", "cmd"),
		filepath.Join("..", "internal"),
		filepath.Join("..", "tests"),
		filepath.Join("..", "scripts"),
	}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(data), "\n") {
				if processTimezoneAssignment.MatchString(line) {
					t.Errorf("%s:%d assigns time.Local; a test must inject the clock instead, otherwise it races with every goroutine that calls time.Now()", path, number+1)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
