package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveDefaultsKeepBackupsAndSkipPruning(t *testing.T) {
	cfg := &Config{}

	if cfg.BackupEnabled() || cfg.RetentionEnabled() || cfg.MaintenanceEnabled() {
		t.Error("an empty archive section must not run any maintenance")
	}
	if got := cfg.ResolvedBackupKeep(); got != DefaultBackupKeep {
		t.Errorf("keep = %d, want the default %d", got, DefaultBackupKeep)
	}
	if got := cfg.ResolvedMaintenanceSchedule(); got != DefaultMaintenanceSchedule {
		t.Errorf("schedule = %q, want the default %q", got, DefaultMaintenanceSchedule)
	}
}

func TestArchiveKeepWithoutALimitStaysUnlimited(t *testing.T) {
	cfg := &Config{}
	cfg.Archive.BackupKeep = -1

	if got := cfg.ResolvedBackupKeep(); got != 0 {
		t.Errorf("keep = %d, want 0 (keep every copy)", got)
	}
}

func archiveBase() *Config {
	cfg := &Config{}
	cfg.HTTP.Port = 8081
	cfg.Telegram.Token = "token"
	return cfg
}

func TestArchiveMaintenanceTurnsOnWithEitherHalf(t *testing.T) {
	backups := archiveBase()
	backups.Archive.BackupDir = " /data/backups "
	if !backups.BackupEnabled() || !backups.MaintenanceEnabled() || backups.RetentionEnabled() {
		t.Errorf("a backup directory must arm the maintenance: %+v", backups.Archive)
	}
	if err := backups.Validate(); err != nil {
		t.Errorf("a backup directory alone must validate: %v", err)
	}

	pruning := archiveBase()
	pruning.Archive.RetentionDays = 30
	if pruning.BackupEnabled() || !pruning.RetentionEnabled() || !pruning.MaintenanceEnabled() {
		t.Errorf("a retention window must arm the maintenance: %+v", pruning.Archive)
	}
	if err := pruning.Validate(); err != nil {
		t.Errorf("a retention window alone must validate: %v", err)
	}
}

func TestArchiveValidation(t *testing.T) {
	broken := archiveBase()
	broken.Archive.RetentionDays = 30
	broken.Archive.MaintenanceSchedule = "every day"
	broken.Archive.BackupKeep = -1
	err := broken.Validate()
	if err == nil {
		t.Fatal("expected the broken archive section to be rejected")
	}
	for _, want := range []string{"archive.backup_keep", "archive.maintenance_schedule"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}

	negative := archiveBase()
	negative.Archive.RetentionDays = -1
	if err := negative.Validate(); err == nil || !strings.Contains(err.Error(), "archive.retention_days") {
		t.Errorf("a negative retention must be rejected, got %v", err)
	}

	unused := archiveBase()
	unused.Archive.MaintenanceSchedule = "every day"
	if err := unused.Validate(); err != nil {
		t.Errorf("an unused schedule must not be validated: %v", err)
	}
}

func TestArchiveSectionComesFromTheEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  port: 8081\ntelegram:\n  token: \"token\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithEnv(path, envLookup(map[string]string{
		"MGKE_ARCHIVE_BACKUP_DIR":           "/data/backups",
		"MGKE_ARCHIVE_BACKUP_KEEP":          "3",
		"MGKE_ARCHIVE_RETENTION_DAYS":       "90",
		"MGKE_ARCHIVE_MAINTENANCE_SCHEDULE": "0 0 5 * * *",
	}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}

	if cfg.Archive.BackupDir != "/data/backups" || cfg.ResolvedBackupKeep() != 3 {
		t.Errorf("backup settings = %+v", cfg.Archive)
	}
	if cfg.Archive.RetentionDays != 90 {
		t.Errorf("retention = %d, want 90", cfg.Archive.RetentionDays)
	}
	if cfg.ResolvedMaintenanceSchedule() != "0 0 5 * * *" {
		t.Errorf("schedule = %q", cfg.ResolvedMaintenanceSchedule())
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("the environment values must validate: %v", err)
	}
}
