package rooms

import (
	"regexp"
	"sort"
	"strings"
)

var (
	cabinetPrefixRe  = regexp.MustCompile(`(?i)^каб\.?\s*`)
	roomDecorationRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
)

func Key(room string) string {
	key := strings.TrimSpace(room)
	key = cabinetPrefixRe.ReplaceAllString(key, "")
	key = roomDecorationRe.ReplaceAllString(key, "")
	key = strings.Join(strings.Fields(key), "")
	return strings.ToLower(key)
}

func Universe(list []string) map[string]string {
	labels := make(map[string]string, len(list))
	for _, room := range list {
		label := strings.TrimSpace(room)
		if label == "" {
			continue
		}
		key := Key(label)
		if key == "" {
			continue
		}
		if known, ok := labels[key]; !ok || preferred(label, known) {
			labels[key] = label
		}
	}
	return labels
}

func Free(universe map[string]string, occupied map[string]bool) []string {
	free := make([]string, 0, len(universe))
	for key, label := range universe {
		if !occupied[key] {
			free = append(free, label)
		}
	}
	sort.Strings(free)
	return free
}

func preferred(candidate, current string) bool {
	if len(candidate) != len(current) {
		return len(candidate) < len(current)
	}
	return candidate < current
}
