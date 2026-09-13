package telegram

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func TestKeyboardLayoutsGolden(t *testing.T) {
	layouts := SurfaceLayouts()
	if len(layouts) == 0 {
		t.Fatal("no keyboard layouts produced")
	}

	got := renderLayouts(layouts)
	path := filepath.Join("testdata", "keyboard_layouts.golden")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v (run go test ./internal/telegram -update)", err)
	}
	if got != string(want) {
		t.Errorf("keyboard layouts changed; review the diff and run:\n  go test ./internal/telegram -update\n\n%s",
			firstDifference(string(want), got))
	}
}

func TestEveryKeyboardBuilderRenders(t *testing.T) {
	builders := map[string]bool{}
	for _, layout := range SurfaceLayouts() {
		builders[layout.Builder] = true
	}
	for _, expected := range []string{
		"replyMainMenu",
		"replySettingsMain",
		"replySettingsButtons",
		"replySettingsNotice",
		"replySettingsView",
		"replySettingsDiff",
		"replySettingsDiffAdvanced",
		"replySettingsFormatters",
		"replySettingsSchedules",
		"replySettingsAliases",
		"replySubscriptionsMenu",
		"replySelectMode",
		"replyCancel",
		"replyStartButton",
		"withCancelButton",
		"weekControlKeyboard",
	} {
		if !builders[expected] {
			t.Errorf("keyboard builder %q rendered nothing", expected)
		}
	}
}

func renderLayouts(layouts []Layout) string {
	var b strings.Builder
	for _, layout := range layouts {
		b.WriteString("=== ")
		b.WriteString(layout.Builder)
		b.WriteString(" ===\n")
		for _, row := range layout.Rows {
			b.WriteString(strings.Join(row, " | "))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var wantLine, gotLine string
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n  want: " + wantLine + "\n  got:  " + gotLine
		}
	}
	return "no line difference found"
}
