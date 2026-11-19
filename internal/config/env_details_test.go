package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestTimezoneDefaultsToMinskAndValidatesItsName(t *testing.T) {
	cfg := &Config{}
	if got := cfg.ResolvedTimezone(); got != DefaultTimezone {
		t.Fatalf("ResolvedTimezone = %q, want %q", got, DefaultTimezone)
	}
	if loc := cfg.Location(); loc.String() != DefaultTimezone {
		t.Fatalf("Location = %q, want %q", loc, DefaultTimezone)
	}
	if _, offset := time.Date(2026, 9, 7, 12, 0, 0, 0, cfg.Location()).Zone(); offset != 3*60*60 {
		t.Fatalf("Minsk offset = %d, want three hours", offset)
	}

	cfg.Timezone = " Asia/Novosibirsk "
	if got := cfg.ResolvedTimezone(); got != "Asia/Novosibirsk" {
		t.Fatalf("ResolvedTimezone = %q, want the trimmed name", got)
	}

	cfg.Timezone = "Mars/Olympus"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "timezone must be an IANA zone name") {
		t.Fatalf("an unknown zone must be rejected, got %v", err)
	}

	cfg.Timezone = "UTC"
	cfg.HTTP.Port = 8080
	cfg.Telegram.Token = "token"
	cfg.DBPath = "archive.db"
	cfg.ChatDBPath = "chat.db"
	cfg.CacheDir = "cache"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("UTC must be accepted: %v", err)
	}
}
