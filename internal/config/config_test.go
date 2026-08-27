package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
dev: true
db_path: "./test.db"
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
  v2:
    enabled: true
    fallback_to_v1: true
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

	if !cfg.Dev {
		t.Error("expected dev=true")
	}
	if cfg.DBPath != "./test.db" {
		t.Errorf("expected db_path ./test.db, got %s", cfg.DBPath)
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
	if cfg.Parser.V2 == nil {
		t.Fatal("expected v2 config")
	}
	if !cfg.Parser.V2.Enabled {
		t.Error("expected v2.enabled=true")
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
