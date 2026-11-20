package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// latinOnly matches a value that holds Latin letters and punctuation but no
// Cyrillic at all: that is what an untranslated message looks like.
var latinOnly = regexp.MustCompile(`^[A-Za-z0-9 .,:;!?'"()/+\-–—]*[A-Za-z]{4,}[A-Za-z0-9 .,:;!?'"()/+\-–—]*$`)

// latinValuesAllowed are the ru.json values that stay Latin on purpose: the
// admin key envelope fields and a console section header.
var latinValuesAllowed = map[string]bool{
	"api_create_id":  true,
	"api_create_iv":  true,
	"webhook_header": true,
}

func TestTheRussianLocaleIsRussian(t *testing.T) {
	root := repoRoot(t)

	data, err := os.ReadFile(filepath.Join(root, "internal", "i18n", "locales", "ru.json"))
	if err != nil {
		t.Fatalf("read the locale: %v", err)
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse the locale: %v", err)
	}

	var untranslated []string
	for key, value := range values {
		if latinValuesAllowed[key] {
			continue
		}
		if latinOnly.MatchString(value) {
			untranslated = append(untranslated, key+" = "+value)
		}
	}
	sort.Strings(untranslated)
	for _, line := range untranslated {
		t.Errorf("the Russian locale holds an English value: %s", line)
	}
}
