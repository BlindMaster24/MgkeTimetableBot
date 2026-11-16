package tests

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocaleKeysAreUniqueAndUsed(t *testing.T) {
	localePath := filepath.Join("..", "internal", "i18n", "locales", "ru.json")
	file, err := os.Open(localePath)
	if err != nil {
		t.Fatalf("open the locale file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("read the locale object: %v", err)
	}

	keys := make(map[string]bool)
	var duplicates []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatalf("read a locale key: %v", err)
		}
		key, ok := token.(string)
		if !ok {
			t.Fatalf("unexpected locale key token: %v", token)
		}
		if keys[key] {
			duplicates = append(duplicates, key)
		}
		keys[key] = true
		if _, err := decoder.Token(); err != nil {
			t.Fatalf("skip the value of %s: %v", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil && err != io.EOF {
		t.Fatalf("close the locale object: %v", err)
	}
	if len(duplicates) > 0 {
		t.Fatalf("the locale file repeats keys: %v", duplicates)
	}

	corpus := goSourceCorpus(t)
	for key := range keys {
		if strings.HasPrefix(key, "weekday_") {
			continue
		}
		if !strings.Contains(corpus, `"`+key+`"`) {
			t.Errorf("locale key %q is not referenced by any Go source", key)
		}
	}
}

func goSourceCorpus(t *testing.T) string {
	t.Helper()

	var builder strings.Builder
	root := filepath.Join("..", "internal")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builder.Write(data)
		builder.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go sources: %v", err)
	}
	return builder.String()
}
