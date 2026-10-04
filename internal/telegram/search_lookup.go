package telegram

import (
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/search"
)

func (b *Bot) searchAliases(userID int64) []search.Alias {
	if b.aliasRepo == nil {
		return nil
	}
	stored, err := b.aliasRepo.List(userID)
	if err != nil {
		return nil
	}
	out := make([]search.Alias, 0, 2*len(stored))
	for _, a := range stored {
		if a.Key == "" || a.Value == "" {
			continue
		}
		out = append(out, search.Alias{Key: a.Key, Value: a.Value}, search.Alias{Key: a.Value, Value: a.Key})
	}
	return out
}

const minGroupLookupScore = 1.5

func (b *Bot) lookupGroup(input string, userID int64) (string, bool) {
	groups := b.cache.GetGroups()
	results := search.SearchGroups(input, sortedKeys(groups), b.searchAliases(userID), 5)
	for _, r := range results {
		if r.Score < minGroupLookupScore {
			continue
		}
		if _, ok := groups[r.Name]; ok {
			return r.Name, true
		}
	}
	return "", false
}

func matchTeacherList(input string, candidates map[string]any, fullNames map[string]string, aliases []search.Alias) ([]string, bool) {
	const matchLimit = 5
	results := search.SearchTeachers(input, sortedKeys(candidates), aliases, matchLimit+2)
	var matched []string
	for _, r := range results {
		if _, ok := candidates[r.Name]; ok {
			matched = append(matched, r.Name)
		}
	}
	if len(matched) > 0 && len(results) > 0 && results[0].Score == 3 && results[0].Name == matched[0] {
		return matched[:1], false
	}
	for _, key := range sortedKeys(fullNames) {
		if len(matched) > matchLimit {
			break
		}
		if containsString(matched, key) {
			continue
		}
		if !strings.Contains(strings.ToLower(fullNames[key]), strings.ToLower(input)) {
			continue
		}
		matched = append(matched, key)
	}
	return matched, len(matched) > matchLimit
}
