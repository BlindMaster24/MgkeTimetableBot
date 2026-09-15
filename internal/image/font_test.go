package image

import (
	"os"
	"path/filepath"
	"testing"
)

func sysFontOrSkip(t *testing.T) (string, []byte) {
	t.Helper()

	path := findFont()
	if path == "" {
		t.Skip("no font available on this system")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return path, data
}

func TestFontLoadsRejectsFilesThatAreNotFonts(t *testing.T) {
	dir := t.TempDir()

	broken := filepath.Join(dir, "broken.ttf")
	if err := os.WriteFile(broken, []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}

	if fontLoads(broken) {
		t.Error("a file that is not a font must not pass the check")
	}
	if fontLoads(dir) {
		t.Error("a directory must not pass the font check")
	}
	if fontLoads(filepath.Join(dir, "missing.ttf")) {
		t.Error("a missing file must not pass the font check")
	}
}

func TestFirstFontInPicksTheFontFromTheDirectory(t *testing.T) {
	_, data := sysFontOrSkip(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "Sample.ttf")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := firstFontIn(dir); got != target {
		t.Errorf("firstFontIn() = %q, want %q", got, target)
	}
}

func TestFirstFontInSkipsEmojiFonts(t *testing.T) {
	_, data := sysFontOrSkip(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Apple Color Emoji.ttf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "Sample.ttf")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := firstFontIn(dir); got != target {
		t.Errorf("firstFontIn() = %q, want it to skip the emoji font and return %q", got, target)
	}
}

func TestFindFontOnlyReturnsLoadableFonts(t *testing.T) {
	path := findFont()
	if path == "" {
		t.Skip("no font available on this system")
	}

	if !fontLoads(path) {
		t.Errorf("findFont() = %q, but the renderer cannot load it", path)
	}
	if cached := findFont(); cached != path {
		t.Errorf("findFont() is not stable: %q then %q", path, cached)
	}
}
