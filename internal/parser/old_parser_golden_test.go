package parser

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func readGoldenGzip(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "old_parser", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gunzip %s: %v", path, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func goldenDocument(t *testing.T, name string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(readGoldenGzip(t, name)))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

func canonicalValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return generic
}

func firstDifference(want, got any, path string) string {
	if reflect.TypeOf(want) != reflect.TypeOf(got) {
		return fmt.Sprintf("%s: type %T vs %T", path, want, got)
	}
	switch expected := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: map vs %T", path, got)
		}
		for key, expectedValue := range expected {
			actualValue, present := actual[key]
			if !present {
				return fmt.Sprintf("%s.%s: missing", path, key)
			}
			if diff := firstDifference(expectedValue, actualValue, path+"."+key); diff != "" {
				return diff
			}
		}
		for key := range actual {
			if _, present := expected[key]; !present {
				return fmt.Sprintf("%s.%s: unexpected", path, key)
			}
		}
	case []any:
		actual, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: list vs %T", path, got)
		}
		if len(expected) != len(actual) {
			return fmt.Sprintf("%s: length %d vs %d", path, len(expected), len(actual))
		}
		for i := range expected {
			if diff := firstDifference(expected[i], actual[i], fmt.Sprintf("%s[%d]", path, i)); diff != "" {
				return diff
			}
		}
	default:
		if !reflect.DeepEqual(want, got) {
			return fmt.Sprintf("%s: %#v vs %#v", path, want, got)
		}
	}
	return ""
}

func assertMatchesOldParser(t *testing.T, goldenName string, parsed any, expectedEntries int) {
	t.Helper()

	want := canonicalValue(t, decodeGolden(t, goldenName))
	got := canonicalValue(t, parsed)

	entries, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("parsed value is %T", got)
	}
	if len(entries) != expectedEntries {
		t.Fatalf("%s: parsed %d entries, want %d", goldenName, len(entries), expectedEntries)
	}

	if diff := firstDifference(want, got, ""); diff != "" {
		t.Fatalf("%s differs from the old TypeScript parser: %s", goldenName, diff)
	}
}

func decodeGolden(t *testing.T, name string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(readGoldenGzip(t, name), &value); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return value
}

func TestGroupParserMatchesOldParserOnLivePage(t *testing.T) {
	doc := goldenDocument(t, "groups.html.gz")

	groups, _ := NewGroupParser(doc).Run()

	out := map[string]any{}
	for name, group := range groups {
		days := make([]any, 0, len(group.Days))
		for _, day := range group.Days {
			days = append(days, map[string]any{"day": day.Day, "lessons": day.Lessons})
		}
		out[name] = map[string]any{"group": group.Group, "days": days}
	}

	assertMatchesOldParser(t, "groups.golden.json.gz", out, 35)
}

func TestTeacherParserMatchesOldParserOnLivePage(t *testing.T) {
	doc := goldenDocument(t, "teachers.html.gz")

	teachers, _ := NewTeacherParser(doc).Run()

	out := map[string]any{}
	for name, teacher := range teachers {
		days := make([]any, 0, len(teacher.Days))
		for _, day := range teacher.Days {
			days = append(days, map[string]any{"day": day.Day, "lessons": day.Lessons})
		}
		out[name] = map[string]any{"teacher": teacher.Teacher, "days": days}
	}

	assertMatchesOldParser(t, "teachers.golden.json.gz", out, 82)
}
