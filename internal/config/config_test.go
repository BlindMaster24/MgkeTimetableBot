package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
db_path: "./test.db"
chat_db_path: "./test-chats.db"
cache_dir: "./test-cache"
logging:
  level: "debug"
  file:
    enabled: false
http:
  port: 8080
telegram:
  token: "test-token"
  admin_ids: [123, 456]
parser:
  enabled: true
  activity: [9, 17]
timetable:
  weekdays:
    - [["08:00", "08:45"], ["08:55", "09:40"]]
    - [["09:50", "10:35"], ["10:45", "11:30"]]
  saturday:
    - [["08:00", "08:45"], ["08:55", "09:40"]]
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DBPath != "./test.db" {
		t.Errorf("expected db_path ./test.db, got %s", cfg.DBPath)
	}
	if cfg.ChatDBPath != "./test-chats.db" || cfg.CacheDir != "./test-cache" {
		t.Errorf("expected storage paths, got %q %q", cfg.ChatDBPath, cfg.CacheDir)
	}
	if cfg.ResolvedChatDBPath() != "./test-chats.db" || cfg.ResolvedCacheDir() != "./test-cache" {
		t.Errorf("resolved paths = %q %q", cfg.ResolvedChatDBPath(), cfg.ResolvedCacheDir())
	}
	if !cfg.Parser.Enabled || cfg.Parser.Activity[0] != 9 || cfg.Parser.Activity[1] != 17 {
		t.Errorf("parser config = %+v", cfg.Parser)
	}
	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.Telegram.Token != "test-token" {
		t.Errorf("expected token test-token, got %s", cfg.Telegram.Token)
	}
	if len(cfg.Telegram.AdminIDs) != 2 {
		t.Errorf("expected 2 admin IDs, got %d", len(cfg.Telegram.AdminIDs))
	}
	if len(cfg.Timetable.Weekdays) != 2 {
		t.Errorf("expected 2 weekday slots, got %d", len(cfg.Timetable.Weekdays))
	}
	if len(cfg.Timetable.Saturday) != 1 {
		t.Errorf("expected 1 saturday slot, got %d", len(cfg.Timetable.Saturday))
	}
	if cfg.Timetable.Weekdays[0][0][0] != "08:00" {
		t.Errorf("expected first weekday start 08:00, got %s", cfg.Timetable.Weekdays[0][0][0])
	}
}

func TestLoadHealthConfig(t *testing.T) {
	yaml := `
health:
  check_minutes: 5
  cooldown_minutes: 45
  parser_stale_minutes: 20
  parser_failures: 4
  parser_layout_failures: 3
  parser_guard_failures: 1
  calendar_stale_minutes: 120
  calendar_failures: 2
  api_errors: 50
  api_window_minutes: 10
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "health.yaml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Health == nil {
		t.Fatal("expected the health section to be parsed")
	}
	if cfg.Health.CheckMinutes != 5 || cfg.Health.CooldownMinutes != 45 {
		t.Errorf("health schedule = %+v", cfg.Health)
	}
	if cfg.Health.ParserStaleMinutes != 20 || cfg.Health.ParserFailures != 4 {
		t.Errorf("parser thresholds = %+v", cfg.Health)
	}
	if cfg.Health.ParserLayoutFailures != 3 {
		t.Errorf("parser layout threshold = %d", cfg.Health.ParserLayoutFailures)
	}
	if cfg.Health.ParserGuardFailures != 1 {
		t.Errorf("parser guard threshold = %d", cfg.Health.ParserGuardFailures)
	}
	if cfg.Health.CalendarStaleMinutes != 120 || cfg.Health.CalendarFailures != 2 {
		t.Errorf("calendar thresholds = %+v", cfg.Health)
	}
	if cfg.Health.APIErrors != 50 || cfg.Health.APIWindowMinutes != 10 {
		t.Errorf("api thresholds = %+v", cfg.Health)
	}
	if cfg.Health.Disabled {
		t.Error("health checks should be enabled by default")
	}
}

func TestLoadParserGuardConfig(t *testing.T) {
	yaml := `
parser:
  guard:
    disabled: true
    min_items: 30
    max_drop_percent: 60
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "guard.yaml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Parser.Guard.Disabled || cfg.Parser.Guard.MinItems != 30 || cfg.Parser.Guard.MaxDropPercent != 60 {
		t.Errorf("guard config = %+v", cfg.Parser.Guard)
	}
}

func TestLoadParserGuardDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "no-guard.yaml")
	if err := os.WriteFile(cfgPath, []byte("parser:\n  enabled: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Parser.Guard.Disabled {
		t.Error("the guard is on unless it is disabled explicitly")
	}
	if cfg.Parser.Guard.MinItems != 0 || cfg.Parser.Guard.MaxDropPercent != 0 {
		t.Errorf("unset thresholds stay zero and are filled by the parser defaults: %+v", cfg.Parser.Guard)
	}
}

func TestLoadConfigWithoutHealthSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "no-health.yaml")
	if err := os.WriteFile(cfgPath, []byte("dev: false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Health != nil {
		t.Errorf("expected no health section, got %+v", cfg.Health)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	os.WriteFile(cfgPath, []byte(":\n  :\n    invalid:"), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
