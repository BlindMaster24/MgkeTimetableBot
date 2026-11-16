package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeEscape(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		want   string
		nextAt int
	}{
		{"newline", `\n`, "\n", 2},
		{"tab", `\t`, "\t", 2},
		{"hex", `\x41`, "A", 4},
		{"broken hex", `\xZZ`, "x", 2},
		{"unicode", `\u0041`, "A", 6},
		{"braced unicode", `\u{1F600}`, "😀", 9},
		{"broken braced", `\u{zz}`, "u", 2},
		{"surrogate pair", `\uD83D\uDE00`, "😀", 12},
		{"unknown escape", `\q`, "q", 2},
		{"trailing backslash", `\`, `\`, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, next := decodeEscape(tc.src, 0)
			if got != tc.want || next != tc.nextAt {
				t.Fatalf("decodeEscape(%q) = %q, %d; want %q, %d", tc.src, got, next, tc.want, tc.nextAt)
			}
		})
	}
}

func TestGoTextCorpusReadsSourcesAndLocale(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bot.go"), []byte("package bot\n\nvar t = \"Литерал\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bot_test.go"), []byte("package bot\n\nvar t = \"Скрыто\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	corpus := GoTextCorpus([]string{dir}, map[string]string{"key": "Из локали"})
	for _, want := range []string{"Литерал", "Из локали"} {
		if !strings.Contains(corpus, want) {
			t.Fatalf("corpus misses %q: %q", want, corpus)
		}
	}
	if strings.Contains(corpus, "Скрыто") {
		t.Fatalf("test files must stay out of the corpus: %q", corpus)
	}
}

func TestGoButtonLiteralsFindsEveryLabelShape(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture

func row(b *Bot) []telego.KeyboardButton {
	return []telego.KeyboardButton{
		{Text: "Отмена"},
		{Text: b.loc("cancel")},
		{Text: noYesSmile("yes_no", "Да")},
		{Text: "a" + "b"},
		{Text: Sprintf(caption, "x")},
	}
}

func nested() [][]telego.KeyboardButton {
	return [][]telego.KeyboardButton{{telego.KeyboardButton{Text: "Внутри"}}}
}
`
	if err := os.WriteFile(filepath.Join(dir, "buttons.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	labels, err := GoButtonLiterals(dir, map[string]string{"cancel": "Отмена из локали"})
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(labels, "\n")
	for _, want := range []string{"Отмена", "Отмена из локали", "Да", "a", "b", "Внутри"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("labels miss %q: %q", want, labels)
		}
	}
}
