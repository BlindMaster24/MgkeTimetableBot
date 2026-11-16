package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func TestConfigStampFollowsTheFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("telegram:\n  token: a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	first := configStamp(path)
	if len(first) != 8 {
		t.Fatalf("stamp %q must be eight characters", first)
	}
	if again := configStamp(path); again != first {
		t.Fatalf("the same file must give the same stamp: %q then %q", first, again)
	}

	if err := os.WriteFile(path, []byte("telegram:\n  token: b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed := configStamp(path); changed == first {
		t.Fatal("a changed file must change the stamp")
	}

	if missing := configStamp(filepath.Join(dir, "absent.yaml")); missing != "" {
		t.Fatalf("a missing file must give an empty stamp, got %q", missing)
	}
}

func TestGoogleDayChangesMapsTheKinds(t *testing.T) {
	changes := []cache.DayChange{
		{Kind: cache.KindGroups, Value: "100", Date: "07.09.2026"},
		{Kind: cache.KindTeachers, Value: "Иванов И.И.", Date: "08.09.2026"},
	}

	got := googleDayChanges(changes)
	if len(got) != 2 {
		t.Fatalf("changes = %+v", got)
	}
	if got[0].Type != "group" || got[0].Value != "100" || got[0].Date != "07.09.2026" {
		t.Fatalf("the group change = %+v", got[0])
	}
	if got[1].Type != "teacher" || got[1].Value != "Иванов И.И." {
		t.Fatalf("the teacher change = %+v", got[1])
	}

	if empty := googleDayChanges(nil); len(empty) != 0 {
		t.Fatalf("no changes must map to nothing, got %+v", empty)
	}
}

func TestAbsolutePathResolvesRelativePaths(t *testing.T) {
	abs := absolutePath("config.yaml")
	if !filepath.IsAbs(abs) {
		t.Fatalf("absolutePath returned %q", abs)
	}
	if !strings.HasSuffix(abs, string(filepath.Separator)+"config.yaml") {
		t.Fatalf("absolutePath lost the file name: %q", abs)
	}

	already := filepath.Join(t.TempDir(), "config.yaml")
	if got := absolutePath(already); got != already {
		t.Fatalf("an absolute path must stay as it is: %q", got)
	}
}
