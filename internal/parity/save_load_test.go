package parity

import (
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"testing"
)

func parseExpr(t *testing.T, source string) ast.Expr {
	t.Helper()
	expr, err := parser.ParseExpr(source)
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	return expr
}

func TestSaveSurfaceRoundTripsThroughTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "surface.json")
	surface := Surface{
		Commands:  []string{"start", "week"},
		Callbacks: []string{"menu", "week"},
		Buttons:   []string{"Отмена", "Отмена"},
		Texts:     []string{"Готово", "Готово"},
	}

	if err := SaveSurface(path, surface); err != nil {
		t.Fatalf("SaveSurface: %v", err)
	}
	loaded, err := LoadSurface(path)
	if err != nil {
		t.Fatalf("LoadSurface: %v", err)
	}
	if len(loaded.Commands) != 2 || len(loaded.Buttons) != 1 {
		t.Fatalf("loaded = %+v, want the saved surface with duplicates dropped", loaded)
	}

	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSurface(path); err == nil {
		t.Fatal("a broken surface file must be an error")
	}
}

func TestSaveSurfaceReportsUnwritablePaths(t *testing.T) {
	dir := t.TempDir()
	if err := SaveSurface(dir, Surface{}); err == nil {
		t.Fatal("saving onto a directory must fail")
	}
}

func TestLoadLocaleTrimsTheDotPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ru.json")
	body := `{".button": ".Отмена", "plain": "Готово"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	locale, err := LoadLocale(path)
	if err != nil {
		t.Fatalf("LoadLocale: %v", err)
	}
	if locale["button"] != "Отмена" || locale["plain"] != "Готово" {
		t.Fatalf("locale = %v, want the keys and values without their dots", locale)
	}

	if _, err := LoadLocale(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("a missing locale must be an error")
	}
}

func TestSkipLiteralAndBlockStopSafely(t *testing.T) {
	if got := skipLiteral(`"a\"b" tail`, 0); got != 6 {
		t.Fatalf("skipLiteral = %d, want past the escaped quote", got)
	}
	if got := skipLiteral(`"unterminated`, 0); got != len(`"unterminated`) {
		t.Fatalf("an unterminated literal must stop at the end, got %d", got)
	}
	if got := skipBlock("/* still open", 0); got != len("/* still open") {
		t.Fatalf("an unterminated block must stop at the end, got %d", got)
	}
}

func TestKeyboardTypesAreRecognised(t *testing.T) {
	cases := map[string]bool{
		"telego.KeyboardButton{}":         true,
		"telego.InlineKeyboardButton{}":   true,
		"[]telego.InlineKeyboardButton{}": true,
		"[][]telego.KeyboardButton{}":     true,
		"*telego.KeyboardButton{}":        true,
		"telego.OtherButton{}":            false,
		"string":                          false,
	}
	for source, want := range cases {
		expr := parseExpr(t, source)
		composite, ok := expr.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if got := isKeyboardType(composite.Type); got != want {
			t.Errorf("isKeyboardType(%s) = %v, want %v", source, got, want)
		}
	}
	if isKeyboardType(nil) {
		t.Fatal("a missing type is not a keyboard")
	}
}

func TestKeyboardLabelsLookThroughHelpers(t *testing.T) {
	locale := map[string]string{"cancel": "Отмена"}
	cases := []struct {
		source string
		want   []string
	}{
		{`telego.KeyboardButton{Text: "Отмена"}`, []string{"Отмена"}},
		{`telego.KeyboardButton{Text: "a" + "b"}`, []string{"a", "b"}},
		{`telego.KeyboardButton{Text: ("скобки")}`, []string{"скобки"}},
		{`telego.KeyboardButton{Text: loc("cancel")}`, []string{"Отмена"}},
		{`telego.KeyboardButton{Text: fmt.Sprintf("%s", "шаблон")}`, []string{"%s"}},
		{`telego.KeyboardButton{Text: noYesSmile("key", "Да")}`, []string{"Да"}},
		{`telego.KeyboardButton{Text: 42}`, nil},
	}
	for _, tc := range cases {
		literals := keyboardLabels(parseExpr(t, tc.source).(*ast.CompositeLit), locale)
		if len(literals) != len(tc.want) {
			t.Fatalf("%s = %v, want %v", tc.source, literals, tc.want)
		}
		for i := range tc.want {
			if literals[i] != tc.want[i] {
				t.Fatalf("%s = %v, want %v", tc.source, literals, tc.want)
			}
		}
	}
}
