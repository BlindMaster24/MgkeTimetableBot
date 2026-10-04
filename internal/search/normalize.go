package search

import "strings"

func Normalize(s string) string {
	lower := strings.ToLower(s)
	lower = strings.ReplaceAll(lower, "ё", "е")
	for _, dash := range []string{"—", "–", "―"} {
		lower = strings.ReplaceAll(lower, dash, "-")
	}
	return strings.Join(strings.Fields(lower), " ")
}

func compactForm(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return strings.ReplaceAll(s, ".", "")
}

func trigrams(s string) []string {
	runes := []rune(s)
	if len(runes) < 3 {
		return []string{s}
	}
	out := make([]string, 0, len(runes)-2)
	for i := 0; i+3 <= len(runes); i++ {
		out = append(out, string(runes[i:i+3]))
	}
	return out
}

func dice(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	counts := make(map[string]int, len(a))
	for _, gram := range a {
		counts[gram]++
	}
	inter := 0
	for _, gram := range b {
		if counts[gram] > 0 {
			counts[gram]--
			inter++
		}
	}
	return 2 * float64(inter) / float64(len(a)+len(b))
}

func distance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			best := prev[j] + 1
			if ins := curr[j-1] + 1; ins < best {
				best = ins
			}
			if sub := prev[j-1] + cost; sub < best {
				best = sub
			}
			curr[j] = best
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func levSim(a, b string) float64 {
	ar := []rune(a)
	br := []rune(b)
	longest := len(ar)
	if len(br) > longest {
		longest = len(br)
	}
	if longest == 0 {
		return 0
	}
	return 1 - float64(distance(ar, br))/float64(longest)
}

func fuzzyScore(a, b string) float64 {
	best := dice(trigrams(a), trigrams(b))
	if sim := levSim(a, b); sim > best {
		best = sim
	}
	return best
}
