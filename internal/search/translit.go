package search

import "strings"

var ruToEn = map[rune]string{
	'а': "a",
	'б': "b",
	'в': "v",
	'г': "g",
	'д': "d",
	'е': "e",
	'ё': "yo",
	'ж': "zh",
	'з': "z",
	'и': "i",
	'й': "y",
	'к': "k",
	'л': "l",
	'м': "m",
	'н': "n",
	'о': "o",
	'п': "p",
	'р': "r",
	'с': "s",
	'т': "t",
	'у': "u",
	'ф': "f",
	'х': "kh",
	'ц': "ts",
	'ч': "ch",
	'ш': "sh",
	'щ': "shch",
	'ъ': "",
	'ы': "y",
	'ь': "",
	'э': "e",
	'ю': "yu",
	'я': "ya",
}

func ToLatin(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if latin, ok := ruToEn[r]; ok {
			out.WriteString(latin)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

var enToRu = map[rune]rune{
	'a': 'а',
	'b': 'б',
	'c': 'ц',
	'd': 'д',
	'e': 'е',
	'f': 'ф',
	'g': 'г',
	'h': 'х',
	'i': 'и',
	'j': 'й',
	'k': 'к',
	'l': 'л',
	'm': 'м',
	'n': 'н',
	'o': 'о',
	'p': 'п',
	'q': 'к',
	'r': 'р',
	's': 'с',
	't': 'т',
	'u': 'у',
	'v': 'в',
	'w': 'в',
	'x': 'х',
	'y': 'ы',
	'z': 'з',
}

func ToCyrillic(s string) string {
	runes := []rune(strings.ToLower(s))
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	for i < len(runes) {
		if rep, size, ok := matchDigraph(runes[i:]); ok {
			out.WriteRune(rep)
			i += size
			continue
		}
		if rep, ok := enToRu[runes[i]]; ok {
			out.WriteRune(rep)
		} else {
			out.WriteRune(runes[i])
		}
		i++
	}
	return out.String()
}

func matchDigraph(r []rune) (rune, int, bool) {
	if len(r) >= 4 && r[0] == 's' && r[1] == 'h' && r[2] == 'c' && r[3] == 'h' {
		return 'щ', 4, true
	}
	if len(r) >= 3 && r[0] == 's' && r[1] == 'c' && r[2] == 'h' {
		return 'щ', 3, true
	}
	if len(r) < 2 {
		return 0, 0, false
	}
	switch r[0] {
	case 'z':
		if r[1] == 'h' {
			return 'ж', 2, true
		}
	case 'k':
		if r[1] == 'h' {
			return 'х', 2, true
		}
	case 't':
		if r[1] == 's' {
			return 'ц', 2, true
		}
	case 'c':
		if r[1] == 'h' {
			return 'ч', 2, true
		}
	case 's':
		if r[1] == 'h' {
			return 'ш', 2, true
		}
	case 'y':
		if r[1] == 'u' {
			return 'ю', 2, true
		}
		if r[1] == 'a' {
			return 'я', 2, true
		}
		if r[1] == 'o' {
			return 'ё', 2, true
		}
	case 'j':
		if r[1] == 'u' {
			return 'ю', 2, true
		}
		if r[1] == 'a' {
			return 'я', 2, true
		}
	}
	return 0, 0, false
}
