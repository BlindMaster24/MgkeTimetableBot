package search

import "testing"

func TestNormalizeFoldsCaseSpacesAndYo(t *testing.T) {
	cases := map[string]string{
		"Ёлка":             "елка",
		"ИВАНОВ И.И.":      "иванов и.и.",
		"  привет   мир  ": "привет мир",
		"Т–123":            "т-123",
		"Т—123":            "т-123",
		"Т―123":            "т-123",
		"":                 "",
		"   ":              "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompactFormStripsSeparators(t *testing.T) {
	if got := compactForm("т-123"); got != "т123" {
		t.Errorf("compactForm = %q, want %q", got, "т123")
	}
	if got := compactForm("иванов и.и."); got != "ивановии" {
		t.Errorf("compactForm = %q, want %q", got, "ивановии")
	}
}

func TestTrigramsHandleShortInputs(t *testing.T) {
	if grams := trigrams("а"); len(grams) != 1 || grams[0] != "а" {
		t.Errorf("trigrams(а) = %q", grams)
	}
	if grams := trigrams("аб"); len(grams) != 1 {
		t.Errorf("trigrams(аб) = %q", grams)
	}
	if grams := trigrams("abcd"); len(grams) != 2 || grams[0] != "abc" || grams[1] != "bcd" {
		t.Errorf("trigrams(abcd) = %q", grams)
	}
}

func TestDiceScoresOverlap(t *testing.T) {
	full := dice(trigrams("ivanov"), trigrams("ivanov"))
	if full != 1 {
		t.Errorf("dice identical = %v, want 1", full)
	}
	if got := dice(nil, trigrams("a")); got != 0 {
		t.Errorf("dice nil = %v, want 0", got)
	}
	if got := dice(trigrams("a"), nil); got != 0 {
		t.Errorf("dice nil = %v, want 0", got)
	}
	if got := dice(nil, nil); got != 0 {
		t.Errorf("dice empty = %v, want 0", got)
	}
	partial := dice(trigrams("ivanov"), trigrams("ivanox"))
	if partial <= 0 || partial >= 1 {
		t.Errorf("dice partial = %v, want between 0 and 1", partial)
	}
}

func TestLevSimScoresEdits(t *testing.T) {
	if got := levSim("ivanov", "ivanov"); got != 1 {
		t.Errorf("levSim identical = %v, want 1", got)
	}
	if got := levSim("", ""); got != 0 {
		t.Errorf("levSim empty = %v, want 0", got)
	}
	if got := levSim("", "abc"); got != 0 {
		t.Errorf("levSim one empty = %v, want 0", got)
	}
	if got := levSim("abc", ""); got != 0 {
		t.Errorf("levSim one empty = %v, want 0", got)
	}
	if got := levSim("kitten", "sitting"); got < 0.57 || got > 0.58 {
		t.Errorf("levSim kitten/sitting = %v, want about 0.571", got)
	}
}

func TestFuzzyScorePrefersBestSignal(t *testing.T) {
	if got := fuzzyScore("ivanov", "ivanov"); got != 1 {
		t.Errorf("fuzzyScore identical = %v, want 1", got)
	}
	if got := fuzzyScore("ivnaov", "ivanov"); got < 0.66 || got > 0.67 {
		t.Errorf("fuzzyScore typo = %v, want about 0.667", got)
	}
	if got := fuzzyScore("aaaa", "bbbb"); got != 0 {
		t.Errorf("fuzzyScore disjoint = %v, want 0", got)
	}
}
