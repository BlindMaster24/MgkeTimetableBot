package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const envTestYAML = `
db_path: "./from-file.db"
chat_db_path: "./from-file-chats.db"
logging:
  level: "info"
http:
  port: 8081
telegram:
  token: "file-token"
  admin_ids: [1, 2]
  noticer: false
google:
  oauth:
    client_id: "file-client"
parser:
  enabled: false
  calls:
    enabled: true
    prefer_site: true
health:
  check_minutes: 1
timetable:
  weekdays:
    - [["08:00", "08:45"], ["08:55", "09:40"]]
`

func writeEnvConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(envTestYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func envLookup(values map[string]string) Lookup {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestEnvOverridesFileValues(t *testing.T) {
	cfg, err := LoadWithEnv(writeEnvConfig(t), envLookup(map[string]string{"MGKE_DB_PATH": "./from-env.db",
		"MGKE_CHAT_DB_PATH": "./from-env-chats.db",

		"MGKE_HTTP_PORT": "9090", "MGKE_LOGGING_LEVEL": "debug", "MGKE_TELEGRAM_TOKEN": "env-token",
		"TG_TOKEN": "legacy-token",

		"MGKE_TELEGRAM_ADMIN_IDS":     "7,8,9",
		"MGKE_TELEGRAM_NOTICER":       "yes",
		"MGKE_GOOGLE_OAUTH_CLIENT_ID": "env-client", "MGKE_PARSER_ENABLED": "on",
		"MGKE_PARSER_ACTIVITY":          "8,20",
		"MGKE_PARSER_PROXY":             "http://127.0.0.1:8080",
		"MGKE_PARSER_CALLS_PREFER_SITE": "no",
	}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}

	if cfg.DBPath != "./from-env.db" || cfg.ChatDBPath != "./from-env-chats.db" {
		t.Errorf("storage paths = %q %q", cfg.DBPath, cfg.ChatDBPath)
	}
	if cfg.HTTP.Port != 9090 {
		t.Errorf("http = %+v", cfg.HTTP)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("log level = %q", cfg.Logging.Level)
	}
	if cfg.Telegram.Token != "env-token" {
		t.Errorf("the canonical MGKE name should win over the legacy alias, got %q", cfg.Telegram.Token)
	}
	if len(cfg.Telegram.AdminIDs) != 3 || cfg.Telegram.AdminIDs[0] != 7 || cfg.Telegram.AdminIDs[2] != 9 {
		t.Errorf("admin ids = %v", cfg.Telegram.AdminIDs)
	}
	if !cfg.Telegram.Noticer {
		t.Error("MGKE_TELEGRAM_NOTICER should turn the noticer on")
	}
	if cfg.Google.OAuth.ClientID != "env-client" {
		t.Errorf("google client id = %q", cfg.Google.OAuth.ClientID)
	}
	if !cfg.Parser.Enabled || cfg.Parser.Activity != [2]int{8, 20} {
		t.Errorf("parser settings = %+v", cfg.Parser)
	}
	if cfg.Parser.Proxy == nil || *cfg.Parser.Proxy != "http://127.0.0.1:8080" {
		t.Errorf("parser proxy = %v", cfg.Parser.Proxy)
	}
	if cfg.Parser.Calls == nil || cfg.Parser.Calls.PreferSite {
		t.Errorf("calls = %+v", cfg.Parser.Calls)
	}
}

func TestEnvLeavesMissingValuesAlone(t *testing.T) {
	cfg, err := LoadWithEnv(writeEnvConfig(t), envLookup(map[string]string{}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}

	if cfg.DBPath != "./from-file.db" || cfg.HTTP.Port != 8081 {
		t.Errorf("file values were changed: %+v", cfg)
	}
	if cfg.Telegram.Token != "file-token" {
		t.Errorf("token = %q", cfg.Telegram.Token)
	}
	if cfg.Parser.Enabled || cfg.Parser.Proxy != nil {
		t.Errorf("parser settings should stay as in the file: %+v", cfg.Parser)
	}
}

func TestEnvAllocatesOptionalSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("db_path: ./x.db\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithEnv(path, envLookup(map[string]string{
		"MGKE_HEALTH_CHECK_MINUTES": "5",
		"MGKE_PARSER_CALLS_ENABLED": "true",
		"MGKE_PARSER_CALLS_NOTIFY":  "false",
	}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}

	if cfg.Health == nil || cfg.Health.CheckMinutes != 5 {
		t.Fatalf("health section = %+v", cfg.Health)
	}
	if cfg.Parser.Calls == nil || !cfg.Parser.Calls.Enabled {
		t.Fatalf("calls section = %+v", cfg.Parser.Calls)
	}
}

func TestEnvKeepsAbsentOptionalSectionsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("db_path: ./x.db\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithEnv(path, envLookup(map[string]string{"MGKE_DB_PATH": "./x.db"}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}
	if cfg.Health != nil {
		t.Errorf("health section should stay nil, got %+v", cfg.Health)
	}
	if cfg.Parser.Calls != nil {
		t.Errorf("calls section should stay nil, got %+v", cfg.Parser.Calls)
	}
}

func TestEnvReportsInvalidValues(t *testing.T) {
	cfg := &Config{}
	err := ApplyEnv(cfg, envLookup(map[string]string{
		"MGKE_HTTP_PORT":          "not-a-number",
		"MGKE_TELEGRAM_ADMIN_IDS": "1,two",
		"MGKE_PARSER_ACTIVITY":    "nope",
	}))
	if err == nil {
		t.Fatal("expected an error for invalid values")
	}

	message := err.Error()
	for _, name := range []string{"MGKE_HTTP_PORT", "MGKE_TELEGRAM_ADMIN_IDS", "MGKE_PARSER_ACTIVITY"} {
		if !strings.Contains(message, name) {
			t.Errorf("error should mention %s: %v", name, message)
		}
	}
}

func TestEnvRejectsUnsupportedStructures(t *testing.T) {
	cfg := &Config{}
	err := ApplyEnv(cfg, envLookup(map[string]string{
		"MGKE_TIMETABLE_WEEKDAYS": "09:00-09:45",
	}))
	if err == nil {
		t.Fatal("expected an error for a complex field")
	}
	if !strings.Contains(err.Error(), "MGKE_TIMETABLE_WEEKDAYS") {
		t.Errorf("error = %v", err)
	}
}

func TestEnvEmptyStringOverrides(t *testing.T) {
	cfg := &Config{DBPath: "./from-file.db"}
	if err := ApplyEnv(cfg, envLookup(map[string]string{"MGKE_DB_PATH": ""})); err != nil {
		t.Fatalf("apply env: %v", err)
	}
	if cfg.DBPath != "" {
		t.Errorf("db path = %q, want the empty override", cfg.DBPath)
	}
}

func TestEnvAcceptsLegacyTagAlias(t *testing.T) {
	cfg := &Config{}
	if err := ApplyEnv(cfg, envLookup(map[string]string{
		"TG_TOKEN":  "legacy-token",
		"DB_PATH":   "./legacy.db",
		"LOG_LEVEL": "warn",
	})); err != nil {
		t.Fatalf("apply env: %v", err)
	}

	if cfg.Telegram.Token != "legacy-token" {
		t.Errorf("token = %q", cfg.Telegram.Token)
	}
	if cfg.DBPath != "./legacy.db" {
		t.Errorf("db path = %q", cfg.DBPath)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("log level = %q", cfg.Logging.Level)
	}
}

func TestEnvNamesCoverEveryLeaf(t *testing.T) {
	legacy := map[string]bool{
		"DB_PATH":     true,
		"LOG_LEVEL":   true,
		"HTTP_PORT":   true,
		"TG_TOKEN":    true,
		"ENCRYPT_KEY": true,
	}

	seen := make(map[string]bool)
	prefixed := 0
	for _, name := range EnvNames() {
		if seen[name] {
			t.Errorf("duplicate env name %s", name)
		}
		seen[name] = true

		if strings.HasPrefix(name, EnvPrefix+"_") {
			prefixed++
			continue
		}
		if !legacy[name] {
			t.Errorf("env name %s is neither prefixed nor a known alias", name)
		}
	}

	if prefixed < 40 {
		t.Fatalf("expected the whole config surface, got %d prefixed names", prefixed)
	}

	for _, expected := range []string{
		"MGKE_HTTP_PORT",
		"MGKE_TELEGRAM_TOKEN",
		"MGKE_PARSER_ENABLED",
		"MGKE_PARSER_ENDPOINTS_BELL_SCHEDULE",
		"MGKE_HEALTH_PARSER_STALE_MINUTES",
		"MGKE_GOOGLE_SERVICE_ACCOUNT_PRIVATE_KEY",
	} {
		if !seen[expected] {
			t.Errorf("env name %s is not exposed", expected)
		}
	}
}
