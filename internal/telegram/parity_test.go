package telegram

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parity"
)

func TestParityWithTypeScriptSurface(t *testing.T) {
	fixture, err := parity.LoadSurface(filepath.Join("testdata", "parity", "ts_surface.json"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	allow, err := parity.LoadAllowlist(filepath.Join("testdata", "parity", "known_differences.json"))
	if err != nil {
		t.Fatalf("load allowlist: %v", err)
	}
	locale, err := parity.LoadLocale(filepath.Join("..", "i18n", "locales", "ru.json"))
	if err != nil {
		t.Fatalf("load locale: %v", err)
	}
	buttons, err := parity.GoButtonLiterals(".", locale)
	if err != nil {
		t.Fatalf("extract button literals: %v", err)
	}

	got := parity.Surface{
		Commands:  SurfaceOfCommands(),
		Callbacks: SurfaceOfCallbacks(),
		Buttons:   buttons,
	}

	diffs := parity.Compare(fixture, got, allow)
	corpus := parity.GoTextCorpus([]string{".", "../formatter", "../notification", "../calendar"}, locale)
	diffs = append(diffs, parity.CompareTexts(fixture.Texts, corpus, allow)...)
	if len(diffs) == 0 {
		return
	}

	var lines []string
	for _, diff := range diffs {
		lines = append(lines, fmt.Sprintf("%s/%s: %q", diff.Section, diff.Kind, diff.Value))
	}
	t.Fatalf("Telegram surface diverged from the TypeScript bot:\n%s\n\n"+
		"Either bring the Go bot back in line, or document the difference in\n"+
		"testdata/parity/known_differences.json and regenerate the fixture with\n"+
		"go run ./scripts/paritycheck -update",
		strings.Join(lines, "\n"))
}

func TestSurfaceIsNotEmpty(t *testing.T) {
	if len(SurfaceOfCommands()) == 0 {
		t.Error("no commands registered")
	}
	if len(SurfaceOfCallbacks()) == 0 {
		t.Error("no callbacks registered")
	}
	if len(SurfaceOfButtons()) == 0 {
		t.Error("no keyboard buttons found")
	}
}
