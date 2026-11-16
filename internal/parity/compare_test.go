package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareSetsReportsBothDirections(t *testing.T) {
	ts := []string{"a", "b"}
	got := []string{"b", "c"}

	diffs := CompareSets("buttons", ts, got, Allowlist{})
	if len(diffs) != 2 {
		t.Fatalf("diffs = %+v, want one missing and one extra", diffs)
	}
	if diffs[0].Kind != KindMissingInGo || diffs[0].Value != "a" {
		t.Fatalf("first diff = %+v", diffs[0])
	}
	if diffs[1].Kind != KindExtraInGo || diffs[1].Value != "c" {
		t.Fatalf("second diff = %+v", diffs[1])
	}

	allow := Allowlist{Decisions: []Decision{
		{Section: "buttons", Kind: KindMissingInGo, Value: "a", Reason: "old button removed"},
		{Section: "buttons", Kind: KindExtraInGo, Value: "c", Reason: "new button"},
	}}
	if diffs := CompareSets("buttons", ts, got, allow); len(diffs) != 0 {
		t.Fatalf("an allowed decision must silence the diff: %+v", diffs)
	}
}

func TestCompareTextsMatchesSubstringsAndAllowlist(t *testing.T) {
	corpus := "Добро пожаловать в бот\nРасписание на неделю"

	if diffs := CompareTexts([]string{"Добро пожаловать"}, corpus, Allowlist{}); len(diffs) != 0 {
		t.Fatalf("a text inside the corpus must pass: %+v", diffs)
	}

	diffs := CompareTexts([]string{"Потерянный текст"}, corpus, Allowlist{})
	if len(diffs) != 1 || diffs[0].Kind != KindMissingInGo || diffs[0].Value != "Потерянный текст" {
		t.Fatalf("diffs = %+v", diffs)
	}

	allow := Allowlist{Decisions: []Decision{{Section: SectionTexts, Kind: KindMissingInGo, Value: "Потерянный текст", Reason: "dropped wording"}}}
	if diffs := CompareTexts([]string{"Потерянный текст"}, corpus, allow); len(diffs) != 0 {
		t.Fatalf("the allowlist must silence the text: %+v", diffs)
	}
}

func TestLoadSurfaceAndAllowlist(t *testing.T) {
	dir := t.TempDir()
	surfacePath := filepath.Join(dir, "surface.json")
	if err := os.WriteFile(surfacePath, []byte(`{"commands":["/start"],"callbacks":["cb"],"buttons":["Кнопка"],"texts":["Текст"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	surface, err := LoadSurface(surfacePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(surface.Commands) != 1 || surface.Commands[0] != "/start" || surface.Texts[0] != "Текст" {
		t.Fatalf("surface = %+v", surface)
	}

	missing, err := LoadAllowlist(filepath.Join(dir, "absent.json"))
	if err != nil || len(missing.Decisions) != 0 {
		t.Fatalf("a missing allowlist must be empty, got %+v, %v", missing, err)
	}

	allowPath := filepath.Join(dir, "known.json")
	if err := os.WriteFile(allowPath, []byte(`{"decisions":[{"section":"texts","kind":"missing_in_go","value":"x","reason":"kept"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAllowlist(allowPath); err != nil {
		t.Fatal(err)
	}

	noReason := filepath.Join(dir, "noreason.json")
	if err := os.WriteFile(noReason, []byte(`{"decisions":[{"section":"texts","kind":"missing_in_go","value":"x"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAllowlist(noReason); err == nil || !strings.Contains(err.Error(), "has no reason") {
		t.Fatalf("a decision without a reason must be rejected, got %v", err)
	}
}

func TestMessageTextsExtractsLiteralTexts(t *testing.T) {
	source := `const x = "Добро пожаловать";
const y = "";
// "Комментарий пропал"
`
	texts := MessageTexts(map[string]string{"a.ts": source, "b.ts": `const z = "До встречи"`})

	joined := strings.Join(texts, "|")
	if !strings.Contains(joined, "Добро пожаловать") || !strings.Contains(joined, "До встречи") {
		t.Fatalf("texts = %q", texts)
	}
	if strings.Contains(joined, "Комментарий пропал") {
		t.Fatalf("comments must be stripped first: %q", texts)
	}
}
