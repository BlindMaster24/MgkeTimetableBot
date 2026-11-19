package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseBoolUnderstandsEverySpelling(t *testing.T) {
	for _, raw := range []string{"1", "t", "TRUE", "yes", "Y", "on", "enable", " enabled "} {
		ok, err := parseBool(raw)
		if err != nil || !ok {
			t.Errorf("parseBool(%q) = %v, %v; want true", raw, ok, err)
		}
	}
	for _, raw := range []string{"0", "f", "False", "no", "N", "off", "disable", "disabled", ""} {
		ok, err := parseBool(raw)
		if err != nil || ok {
			t.Errorf("parseBool(%q) = %v, %v; want false", raw, ok, err)
		}
	}
	if _, err := parseBool("maybe"); err == nil || !strings.Contains(err.Error(), "expected a boolean") {
		t.Fatalf("an unknown word must be rejected, got %v", err)
	}
}

func TestSetEnvValueReportsBadNumbers(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		raw  string
		want string
	}{
		{"int", reflect.TypeOf(int(0)), "abc", "expected an integer"},
		{"uint", reflect.TypeOf(uint(0)), "-1", "expected a positive integer"},
		{"float", reflect.TypeOf(float64(0)), "abc", "expected a number"},
	}
	for _, tc := range cases {
		value := reflect.New(tc.typ).Elem()
		err := setEnvValue(value, tc.raw)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestSnakeUpperHandlesCamelAndAlreadySnake(t *testing.T) {
	cases := map[string]string{
		"diff_max_lines": "DIFF_MAX_LINES",
		"DiffMaxLines":   "DIFF_MAX_LINES",
		"PORT":           "PORT",
		"Port":           "PORT",
	}
	for in, want := range cases {
		if got := snakeUpper(in); got != want {
			t.Errorf("snakeUpper(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestYAMLNameFallsBackToTheFieldName(t *testing.T) {
	type sample struct {
		WithoutTag string
		Dashed     string `yaml:"-"`
		WithOption string `yaml:"with_option,omitempty"`
		Plain      string `yaml:"plain"`
	}
	typ := reflect.TypeOf(sample{})

	cases := map[string]string{
		"WithoutTag": "WITHOUTTAG",
		"Dashed":     "DASHED",
		"WithOption": "with_option",
		"Plain":      "plain",
	}
	for fieldName, want := range cases {
		field, _ := typ.FieldByName(fieldName)
		if got := yamlName(field); got != want {
			t.Errorf("yamlName(%q) = %q, want %q", fieldName, got, want)
		}
	}
}

func TestHasEnvBelowWalksStructsAndPointers(t *testing.T) {
	type inner struct {
		Value string `yaml:"value"`
	}
	type outer struct {
		Name  string `yaml:"name"`
		Inner inner  `yaml:"inner"`
		Ptr   *inner `yaml:"ptr"`
	}
	typ := reflect.TypeOf(outer{})

	lookup := func(names ...string) Lookup {
		return func(key string) (string, bool) {
			for _, name := range names {
				if key == name {
					return "value", true
				}
			}
			return "", false
		}
	}

	if hasEnvBelow(typ, "APP", lookup("APP_NAME")) != true {
		t.Error("a direct field must be found")
	}
	if hasEnvBelow(typ, "APP", lookup("APP_INNER_VALUE")) != true {
		t.Error("a nested struct field must be found")
	}
	if hasEnvBelow(typ, "APP", lookup("APP_PTR_VALUE")) != true {
		t.Error("a nested pointer struct field must be found")
	}
	if hasEnvBelow(typ, "APP", lookup("OTHER")) {
		t.Error("an unknown prefix must not match")
	}
}
