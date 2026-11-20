package tests

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cyrillicLiteralRe matches a Cyrillic letter anywhere in a line. Comments are
// skipped by the scanner, so a documented name never trips it.
var cyrillicLiteralRe = regexp.MustCompile(`[А-Яа-яЁё]`)

// loadCyrillicAllowlist reads tests/testdata/cyrillic_literals.txt and returns
// the paths that may keep their literals together with the reason.
func loadCyrillicAllowlist(t *testing.T) map[string]string {
	t.Helper()

	file, err := os.Open(filepath.Join(repoRoot(t), "tests", "testdata", "cyrillic_literals.txt"))
	if err != nil {
		t.Fatalf("open the allowlist: %v", err)
	}
	defer file.Close()

	allowed := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, reason, ok := strings.Cut(line, " — ")
		if !ok || strings.TrimSpace(reason) == "" {
			t.Fatalf("allowlist line %q has no reason", line)
		}
		allowed[filepath.ToSlash(strings.TrimSpace(path))] = strings.TrimSpace(reason)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read the allowlist: %v", err)
	}
	return allowed
}

// repoRoot walks up from the test package until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// goFilesUnder walks the source tree and yields the non-test Go files.
func goFilesUnder(t *testing.T, roots ...string) []string {
	t.Helper()

	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "old" || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, filepath.ToSlash(path))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return files
}

// TestUserFacingTextLivesOnlyInTheLocale keeps the code and the locale apart:
// every user-facing string belongs in internal/i18n/locales/ru.json, and a Go
// file may hold Cyrillic only when the allowlist explains why.
func TestUserFacingTextLivesOnlyInTheLocale(t *testing.T) {
	allowed := loadCyrillicAllowlist(t)

	root := repoRoot(t)
	for _, path := range goFilesUnder(t, filepath.Join(root, "internal"), filepath.Join(root, "cmd"), filepath.Join(root, "scripts")) {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relative path of %s: %v", path, err)
		}
		relative = filepath.ToSlash(relative)
		if _, ok := allowed[relative]; ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for number, line := range strings.Split(string(data), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			if cyrillicLiteralRe.MatchString(code) {
				t.Errorf("%s:%d holds Cyrillic text in code: %q\n"+
					"user-facing text belongs in internal/i18n/locales/ru.json; "+
					"if this is data, add the file to tests/testdata/cyrillic_literals.txt with a reason",
					relative, number+1, code)
			}
		}
	}
}
