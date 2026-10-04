package search

import "testing"

func TestToLatinCoversAlphabet(t *testing.T) {
	cases := map[string]string{
		"а":           "a",
		"б":           "b",
		"в":           "v",
		"г":           "g",
		"д":           "d",
		"е":           "e",
		"ё":           "yo",
		"ж":           "zh",
		"з":           "z",
		"и":           "i",
		"й":           "y",
		"к":           "k",
		"л":           "l",
		"м":           "m",
		"н":           "n",
		"о":           "o",
		"п":           "p",
		"р":           "r",
		"с":           "s",
		"т":           "t",
		"у":           "u",
		"ф":           "f",
		"х":           "kh",
		"ц":           "ts",
		"ч":           "ch",
		"ш":           "sh",
		"щ":           "shch",
		"ъ":           "",
		"ы":           "y",
		"ь":           "",
		"э":           "e",
		"ю":           "yu",
		"я":           "ya",
		"А":           "a",
		"Ё":           "yo",
		"Щ":           "shch",
		"Иванов":      "ivanov",
		"Щукин":       "shchukin",
		"Юрьев":       "yurev",
		"hello 123-+": "hello 123-+",
		"":            "",
	}
	for in, want := range cases {
		if got := ToLatin(in); got != want {
			t.Errorf("ToLatin(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToCyrillicCoversDigraphs(t *testing.T) {
	cases := map[string]string{
		"shch":   "щ",
		"SHCH":   "щ",
		"sch":    "щ",
		"zh":     "ж",
		"kh":     "х",
		"ts":     "ц",
		"ch":     "ч",
		"sh":     "ш",
		"yu":     "ю",
		"ya":     "я",
		"yo":     "ё",
		"ju":     "ю",
		"ja":     "я",
		"ivanov": "иванов",
		"IVANOV": "иванов",
		"y":      "ы",
		"j":      "й",
		"h":      "х",
		"x":      "х",
		"w":      "в",
		"q":      "к",
		"c":      "ц",
		"sham":   "шам",
		"bus":    "бус",
		"za":     "за",
		"ka":     "ка",
		"tx":     "тх",
		"cq":     "цк",
		"yi":     "ыи",
		"jy":     "йы",
		"aaaa":   "аааа",
		"abc":    "абц",
		"иванов": "иванов",
		"123-+":  "123-+",
		"":       "",
		"s":      "с",
	}
	for in, want := range cases {
		if got := ToCyrillic(in); got != want {
			t.Errorf("ToCyrillic(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslitRoundTripLosslessNames(t *testing.T) {
	for _, name := range []string{"иванов", "петров", "сидоров", "мама", "кот"} {
		if got := ToCyrillic(ToLatin(name)); got != name {
			t.Errorf("round trip %q = %q", name, got)
		}
	}
}
