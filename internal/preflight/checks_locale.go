package preflight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func checkLocale(root string) Check {
	if root == "" {
		return Check{Name: "locale", Level: LevelSkip, Detail: "repository sources not found, locale check skipped"}
	}

	path := filepath.Join(root, localePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Check{Name: "locale", Level: LevelFail, Detail: fmt.Sprintf("%s: %v", localePath, err)}
	}

	var messages map[string]string
	if err := json.Unmarshal(raw, &messages); err != nil {
		return Check{Name: "locale", Level: LevelFail, Detail: localePath + " is not a flat string map", Hints: []string{"error: " + err.Error()}}
	}

	var empty []string
	for key, value := range messages {
		if strings.TrimSpace(value) == "" {
			empty = append(empty, key)
		}
	}
	sort.Strings(empty)

	missing, used := missingLocaleKeys(root, messages)
	if len(missing) > 0 {
		return Check{
			Name:   "locale",
			Level:  LevelFail,
			Detail: fmt.Sprintf("%s: %d of %d keys referenced by the bot are missing", localePath, len(missing), used),
			Hints:  []string{"missing: " + strings.Join(missing, ", ")},
		}
	}

	level := LevelOK
	var hints []string
	if len(empty) > 0 {
		level = LevelWarn
		hints = append(hints, "warning: empty values: "+strings.Join(empty, ", "))
	}
	return Check{
		Name:   "locale",
		Level:  level,
		Detail: fmt.Sprintf("%d keys, %d referenced by the bot all present", len(messages), used),
		Hints:  hints,
	}
}

func missingLocaleKeys(root string, messages map[string]string) ([]string, int) {
	used := make(map[string]bool)
	for _, dir := range []string{telegramSource, noticeSource} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			source, err := os.ReadFile(filepath.Join(root, dir, entry.Name()))
			if err != nil {
				continue
			}
			for _, match := range localeKeyRe.FindAllSubmatch(source, -1) {
				used[string(match[1])] = true
			}
		}
	}

	var missing []string
	for key := range used {
		if _, ok := messages[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing, len(used)
}
