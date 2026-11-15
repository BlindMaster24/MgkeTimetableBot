package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryDocLinksToItsOtherLanguage(t *testing.T) {
	pairs := [][2]string{
		{"README.md", "README.en.md"},
		{"docs/google-calendar.md", "docs/google-calendar.en.md"},
	}

	for _, pair := range pairs {
		for i, path := range pair {
			data, err := os.ReadFile(filepath.Join("..", path))
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			head := string(data)
			if len(head) > 2000 {
				head = head[:2000]
			}
			if !strings.Contains(head, filepath.Base(pair[1-i])) {
				t.Errorf("%s must link to %s at the top", path, pair[1-i])
			}
		}
	}
}
