package parity

import (
	"reflect"
	"testing"
)

func TestStringLiteralsDecodesEscapes(t *testing.T) {
	src := `const a = '\u041F\u0440\u0438\u0432\u0435\u0442';` + "\n" +
		"const b = '\\uD83D\\uDCCA Показать';\n" +
		"const c = `🧾 Лимит строк: ${value}`;\n" +
		"const d = \"with 'quotes' inside\";\n"

	got := StringLiterals(src)
	want := []string{"Привет", "📊 Показать", "🧾 Лимит строк: ${value}", "with 'quotes' inside"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("literals mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestStripCommentsKeepsStrings(t *testing.T) {
	src := "// text: 'comment'\n" +
		"/* text: 'block' */\n" +
		"const keep = 'value';\n" +
		"const url = 'http://example.com';\n"

	got := StringLiterals(StripComments(src))
	want := []string{"value", "http://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("literals mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestNormalizeButton(t *testing.T) {
	cases := map[string]string{
		`✅ Кнопка "📄 На день"`:           `Кнопка "📄 На день"`,
		`🚫 Кнопка "📄 На день"`:           `Кнопка "📄 На день"`,
		"🔈 Оповещение о звонках: Да":     "Оповещение о звонках",
		"🔇 Оповещение о звонках: Нет":    "Оповещение о звонках",
		"🧾 Лимит строк: 25":              "🧾 Лимит строк",
		"🧾 Лимит строк: ${diffMaxLines}": "🧾 Лимит строк",
		"📝 Стуктурированный (выбран)":    "📝 Стуктурированный",
		"❌ %s → %s": "→",
	}
	for input, want := range cases {
		if got := NormalizeButton(input); got != want {
			t.Errorf("NormalizeButton(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsButtonLabelDropsPunctuation(t *testing.T) {
	for _, text := range []string{",", "→", "  ", "123"} {
		if IsButtonLabel(text) {
			t.Errorf("%q should not count as a button label", text)
		}
	}
	if !IsButtonLabel("Начать") {
		t.Error("«Начать» should count as a button label")
	}
}

func TestCompareReportsAndAllowsDifferences(t *testing.T) {
	ts := Surface{Commands: []string{"day", "eval"}, Buttons: []string{"Начать"}}
	got := Surface{Commands: []string{"/Day", "/start"}, Buttons: []string{"Начать"}}

	diffs := Compare(ts, got, Allowlist{})
	want := []Diff{
		{Section: SectionCommands, Kind: KindExtraInGo, Value: "start"},
		{Section: SectionCommands, Kind: KindMissingInGo, Value: "eval"},
	}
	if len(diffs) != len(want) {
		t.Fatalf("diffs = %+v, want %+v", diffs, want)
	}
	for _, expected := range want {
		found := false
		for _, diff := range diffs {
			if diff == expected {
				found = true
			}
		}
		if !found {
			t.Errorf("missing diff %+v in %+v", expected, diffs)
		}
	}

	allow := Allowlist{Decisions: []Decision{
		{Section: SectionCommands, Kind: KindExtraInGo, Value: "start", Reason: "test"},
		{Section: SectionCommands, Kind: KindMissingInGo, Value: "eval", Reason: "test"},
	}}
	if remaining := Compare(ts, got, allow); len(remaining) != 0 {
		t.Errorf("allowlisted differences still reported: %+v", remaining)
	}
}

func TestCompareNormalizesCommandsAndCallbacks(t *testing.T) {
	ts := Surface{Commands: []string{"flushCache", "day"}, Callbacks: []string{"answer", "timetable"}}
	got := Surface{Commands: []string{"/flushcache", "/day"}, Callbacks: []string{"answer:", "timetable_g:"}}

	if diffs := Compare(ts, got, Allowlist{}); len(diffs) != 0 {
		t.Errorf("case and prefix differences should normalize away: %+v", diffs)
	}
}
