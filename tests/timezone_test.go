package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var processTimezoneAssignment = regexp.MustCompile(`\btime\.Local\s*=[^=]`)

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
