package search

import (
	"strings"
	"testing"
)

func FuzzSearchStaysOrderedAndBounded(f *testing.F) {
	f.Add("63тп", "63ТП\n63ТП-2\nАгрономия", 5)
	f.Add("иван", "Иванов И.И.\nПетров П.П.\nмой препод=Иванов И.И.", 3)
	f.Add("любимая группа", "Т-123\nИТ-456\nлюбимая группа=Т-123", 5)
	f.Add("ivnaov", "Иванов И.И.\nАгрономия\nИТ-456", 7)
	f.Add("", "", 0)

	f.Fuzz(func(t *testing.T, query string, blob string, limit int) {
		names, aliases := fuzzCandidates(blob)

		first := SearchGroups(query, names, aliases, limit)
		checkSearchResults(t, query, first, names, aliases, limit)
		checkSearchResults(t, query, SearchTeachers(query, names, aliases, limit), names, aliases, limit)

		if second := SearchGroups(query, names, aliases, limit); !sameResults(first, second) {
			t.Fatalf("Search(%q) is not deterministic: %+v then %+v", query, first, second)
		}

		checkNormalized(t, query)
		for _, name := range names {
			checkNormalized(t, name)
		}
		for _, alias := range aliases {
			checkNormalized(t, alias.Key)
			checkNormalized(t, alias.Value)
		}
	})
}

func fuzzCandidates(blob string) ([]string, []Alias) {
	var names []string
	var aliases []Alias

	for _, line := range strings.Split(blob, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, isAlias := strings.Cut(line, "=")
		if !isAlias {
			names = append(names, line)
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		aliases = append(aliases, Alias{Key: key, Value: value})
	}

	return names, aliases
}

func sameResults(first, second []Result) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}

func checkSearchResults(t *testing.T, query string, results []Result, names []string, aliases []Alias, limit int) {
	t.Helper()

	if limit > 0 && len(results) > limit {
		t.Fatalf("Search(%q) returned %d results for limit %d", query, len(results), limit)
	}

	known := make(map[string]bool, len(names)+len(aliases))
	for _, name := range names {
		known[name] = true
	}
	aliasKeys := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		known[alias.Value] = true
		aliasKeys[alias.Key] = true
	}

	seen := make(map[string]bool, len(results))
	for i, result := range results {
		if result.Score <= 0 || result.Score > 3 {
			t.Fatalf("Search(%q) scored %q at %v", query, result.Name, result.Score)
		}
		if seen[result.Name] {
			t.Fatalf("Search(%q) returned %q twice: %+v", query, result.Name, results)
		}
		seen[result.Name] = true
		if !known[result.Name] {
			t.Fatalf("Search(%q) invented the name %q", query, result.Name)
		}
		if result.ViaAlias != "" && !aliasKeys[result.ViaAlias] {
			t.Fatalf("Search(%q) reported the unknown alias %q", query, result.ViaAlias)
		}
		if i == 0 {
			continue
		}
		previous := results[i-1]
		if previous.Score < result.Score {
			t.Fatalf("Search(%q) results are not ordered by score: %+v", query, results)
		}
		if previous.Score == result.Score && previous.Name > result.Name {
			t.Fatalf("Search(%q) results are not ordered by name on a tie: %+v", query, results)
		}
	}
}

func checkNormalized(t *testing.T, value string) {
	t.Helper()

	once := Normalize(value)
	if twice := Normalize(once); twice != once {
		t.Fatalf("Normalize(%q) is not idempotent: %q then %q", value, once, twice)
	}
	if once != strings.TrimSpace(once) {
		t.Fatalf("Normalize(%q) = %q keeps surrounding spaces", value, once)
	}
	if once != strings.Join(strings.Fields(once), " ") {
		t.Fatalf("Normalize(%q) = %q keeps double spaces", value, once)
	}
}
