package parity

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SectionTexts = "texts"

func MessageTexts(sources map[string]string) []string {
	var values []string
	for _, src := range sources {
		src = StripComments(src)
		values = append(values, StringLiterals(src)...)
	}
	return normalizeTexts(values)
}

func normalizeTexts(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !isMessageText(value) || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isMessageText(value string) bool {
	if len([]rune(value)) < 8 {
		return false
	}
	if strings.ContainsAny(value, "${}") {
		return false
	}
	if strings.HasPrefix(value, "(") || strings.HasPrefix(value, ":") {
		return false
	}
	return hasCyrillic(value)
}

func hasCyrillic(value string) bool {
	for _, r := range value {
		if r >= 0x410 && r <= 0x44F || r == 0x401 || r == 0x451 {
			return true
		}
	}
	return false
}

func GoTextCorpus(dirs []string, locale map[string]string) string {
	var parts []string
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			parts = append(parts, StringLiterals(StripComments(string(data)))...)
			return nil
		})
	}
	for _, value := range locale {
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n")
}

func CompareTexts(ts []string, corpus string, allow Allowlist) []Diff {
	var diffs []Diff
	for _, value := range normalizeTexts(ts) {
		if strings.Contains(corpus, value) || allow.allows(SectionTexts, KindMissingInGo, value) {
			continue
		}
		diffs = append(diffs, Diff{Section: SectionTexts, Kind: KindMissingInGo, Value: value})
	}
	return diffs
}

func CompareSets(section string, ts, got []string, allow Allowlist) []Diff {
	return compareSets(section, ts, got, allow)
}
