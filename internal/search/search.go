package search

import (
	"sort"
	"strings"
)

type Alias struct {
	Key   string
	Value string
}

type Result struct {
	Name     string
	Score    float64
	ViaAlias string
}

func SearchGroups(query string, groups []string, aliases []Alias, limit int) []Result {
	return search(query, groups, aliases, limit)
}

func SearchTeachers(query string, teachers []string, aliases []Alias, limit int) []Result {
	return search(query, teachers, aliases, limit)
}

func search(query string, names []string, aliases []Alias, limit int) []Result {
	q := Normalize(query)
	if q == "" {
		return nil
	}
	qLat := ToLatin(q)
	if qLat == "" {
		qLat = q
	}
	qCompact := compactForm(q)
	ranked := make(map[string]Result)
	keep := func(display, via string, score float64) {
		if prev, ok := ranked[display]; !ok || score > prev.Score {
			ranked[display] = Result{Name: display, Score: score, ViaAlias: via}
		}
	}
	for _, name := range names {
		display := strings.TrimSpace(name)
		if display == "" {
			continue
		}
		norm := Normalize(display)
		if score, ok := scoreCandidate(q, qLat, qCompact, norm, latinOf(norm), compactForm(norm)); ok {
			keep(display, "", score)
		}
	}
	for _, alias := range aliases {
		display := strings.TrimSpace(alias.Value)
		if display == "" || strings.TrimSpace(alias.Key) == "" {
			continue
		}
		norm := Normalize(display)
		best := -1.0
		via := ""
		if score, ok := scoreCandidate(q, qLat, qCompact, norm, latinOf(norm), compactForm(norm)); ok {
			best = score
		}
		if keyNorm := Normalize(alias.Key); keyNorm != "" {
			if score, ok := scoreCandidate(q, qLat, qCompact, keyNorm, latinOf(keyNorm), compactForm(keyNorm)); ok && score > best {
				best = score
				via = alias.Key
			}
		}
		if best >= 0 {
			keep(display, via, best)
		}
	}
	out := make([]Result, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func latinOf(norm string) string {
	if lat := ToLatin(norm); lat != "" {
		return lat
	}
	return norm
}

func scoreCandidate(q, qLat, qCompact, norm, lat, compact string) (float64, bool) {
	if norm == q || lat == qLat {
		return 3, true
	}
	if qCompact != "" && compact == qCompact {
		return 2.5, true
	}
	if strings.HasPrefix(norm, q) || strings.HasPrefix(lat, qLat) {
		return 2, true
	}
	if tokenPrefix(norm, q) || tokenPrefix(lat, qLat) {
		return 1.5, true
	}
	if strings.Contains(norm, q) || strings.Contains(lat, qLat) {
		return 1.5, true
	}
	best := fuzzyScore(q, norm)
	if other := fuzzyScore(qLat, lat); other > best {
		best = other
	}
	if best < 0.3 {
		return 0, false
	}
	return best, true
}

func tokenPrefix(s, q string) bool {
	for _, token := range strings.Fields(s) {
		if strings.HasPrefix(token, q) {
			return true
		}
	}
	return false
}
