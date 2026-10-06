package rooms

import (
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

func fuzzSeeds() []string {
	return []string{
		"101",
		"202",
		"каб. 101",
		"каб.101",
		"101 (к)",
		"каб. 101 (левый корпус)",
		"3-212 (к)",
		"Каб 2-206",
		"кабкаб101",
		"каб. каб. 101",
		"ка б",
		"КАБ 101",
		"101 (к) (л)",
		"\xb0",
		"101 202",
		"каб.",
		"(к)",
		"",
		"   ",
	}
}

func fuzzContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func checkKey(t *testing.T, value string) {
	t.Helper()

	key := Key(value)
	if key != strings.TrimSpace(key) {
		t.Fatalf("Key(%q) = %q, want a trimmed key", value, key)
	}
	if key != strings.ToLower(key) {
		t.Fatalf("Key(%q) = %q, want a lower-cased key", value, key)
	}
	if strings.ContainsAny(key, " \t\r\n") {
		t.Fatalf("Key(%q) = %q, want every space collapsed", value, key)
	}
	if utf8.RuneCountInString(key) > utf8.RuneCountInString(strings.TrimSpace(value)) {
		t.Fatalf("Key(%q) = %q, want no more runes than its input", value, key)
	}
	if !utf8.ValidString(key) {
		t.Fatalf("Key(%q) = %q, want valid UTF-8", value, key)
	}
	if second := Key(key); second != key {
		t.Fatalf("Key(%q) = %q is not a fixpoint: Key(%q) = %q", value, key, key, second)
	}
	if Key(key) != Key(Key(value)) {
		t.Fatalf("Key(%q) is not deterministic", value)
	}
}

func checkUniverse(t *testing.T, inputs []string) map[string]string {
	t.Helper()

	trimmed := make([]string, 0, len(inputs))
	distinct := map[string]bool{}
	for _, input := range inputs {
		trimmed = append(trimmed, strings.TrimSpace(input))
		if key := Key(input); key != "" {
			distinct[key] = true
		}
	}

	universe := Universe(inputs)
	if len(universe) != len(distinct) {
		t.Fatalf("Universe(%q) holds %d labels, want one per distinct non-empty key (%d)", inputs, len(universe), len(distinct))
	}
	for key, label := range universe {
		if key == "" {
			t.Fatalf("Universe(%q) stored an empty key", inputs)
		}
		if key != Key(label) {
			t.Fatalf("Universe(%q) stored label %q under key %q, want %q", inputs, label, key, Key(label))
		}
		if !fuzzContains(trimmed, label) {
			t.Fatalf("Universe(%q) invented the label %q", inputs, label)
		}
	}

	repeated := Universe(inputs)
	if len(repeated) != len(universe) {
		t.Fatalf("Universe(%q) is not deterministic", inputs)
	}
	for key, label := range universe {
		if repeated[key] != label {
			t.Fatalf("Universe(%q) picked %q for key %q on the second call, want %q", inputs, repeated[key], key, label)
		}
	}

	return universe
}

func checkFree(t *testing.T, universe map[string]string) {
	t.Helper()

	want := make([]string, 0, len(universe))
	for _, label := range universe {
		want = append(want, label)
	}
	sort.Strings(want)

	all := Free(universe, nil)
	if !sort.StringsAreSorted(all) {
		t.Fatalf("Free returned %q, want a sorted list", all)
	}
	if len(all) != len(want) {
		t.Fatalf("Free returned %q, want every label of the universe %q", all, want)
	}
	for i, label := range want {
		if all[i] != label {
			t.Fatalf("Free returned %q, want %q", all, want)
		}
	}

	occupied := map[string]bool{}
	for key := range universe {
		occupied[key] = true
	}
	if busy := Free(universe, occupied); len(busy) != 0 {
		t.Fatalf("Free returned %q while every room is busy", busy)
	}
}

func FuzzKeyIsStableAndUniverseKeepsOneLabel(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed, "каб. 101", "101 (к)")
	}
	f.Add("каб101", "каб. 101", "303 (спортзал)")
	f.Add("каб. 101 (левый корпус)", "каб.101", "101")

	f.Fuzz(func(t *testing.T, first string, second string, third string) {
		inputs := []string{first, second, third}
		for _, value := range inputs {
			checkKey(t, value)
		}
		if Key("") != "" {
			t.Fatal("Key of an empty label must stay empty")
		}

		checkFree(t, checkUniverse(t, inputs))
	})
}
