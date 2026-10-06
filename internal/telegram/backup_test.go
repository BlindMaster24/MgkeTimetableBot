package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const backupAdminID = int64(4242)

func backupTestBot(t *testing.T, fn func(ctx context.Context) (BackupReport, error)) (*Bot, *recordingCaller) {
	t.Helper()

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, backupAdminID)
	if fn != nil {
		b.SetBackupFunc(fn)
	}
	return b, caller
}

func sendBackup(t *testing.T, b *Bot, caller *recordingCaller, userID int64) string {
	t.Helper()

	caller.reset()
	cmd := &backupCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/backup"}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	text, _ := lastDeliveredMessage(t, caller)
	return text
}

func TestBackupMatchText(t *testing.T) {
	b, _ := backupTestBot(t, nil)
	cmd := &backupCmd{bot: b}

	for _, text := range []string{"/backup", "/BACKUP", " /backup "} {
		if !cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = false, want true", text)
		}
	}
	for _, text := range []string{"/backups", "backup", "/backup now", "/day"} {
		if cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = true, want false", text)
		}
	}
}

func TestBackupCommandIsAdminOnly(t *testing.T) {
	cmd := &backupCmd{bot: mustBot(t)}
	if !cmd.AdminOnly() {
		t.Error("/backup must be an admin command")
	}
}

func TestBackupReportsTheSnapshotAndThePrune(t *testing.T) {
	b, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		return BackupReport{
			Backup:        true,
			Retention:     true,
			Path:          "/data/backups/sqlite3-20261006-043015.db",
			Removed:       2,
			Kept:          7,
			PrunedRows:    41,
			RetentionDays: 30,
		}, nil
	})

	text := sendBackup(t, b, caller, backupAdminID)
	for _, want := range []string{"/data/backups/sqlite3-20261006-043015.db", "7", "2", "41", "30"} {
		if !strings.Contains(text, want) {
			t.Errorf("answer must contain %q, got %q", want, text)
		}
	}
}

func TestBackupSkipsThePartsThatAreSwitchedOff(t *testing.T) {
	const path = "/data/backups/sqlite3-20261006-043015.db"

	withoutRetention, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		return BackupReport{Backup: true, Path: path, Kept: 1}, nil
	})
	want := withoutRetention.locData("backup_done", map[string]any{"Path": path, "Removed": 0, "Kept": 1})
	if text := sendBackup(t, withoutRetention, caller, backupAdminID); text != want {
		t.Errorf("answer = %q, want only the backup line %q", text, want)
	}

	withoutBackup, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		return BackupReport{Retention: true, PrunedRows: 41, RetentionDays: 30}, nil
	})
	want = withoutBackup.locData("backup_pruned", map[string]any{"Rows": 41, "Days": 30})
	if text := sendBackup(t, withoutBackup, caller, backupAdminID); text != want {
		t.Errorf("answer = %q, want only the prune line %q", text, want)
	}
}

func TestBackupTellsWhenNothingIsConfigured(t *testing.T) {
	disabled := mustBot(t).locData("backup_disabled", nil)

	withoutFunc, caller := backupTestBot(t, nil)
	if text := sendBackup(t, withoutFunc, caller, backupAdminID); !strings.Contains(text, disabled) {
		t.Errorf("answer = %q, want the disabled notice", text)
	}

	off, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		return BackupReport{}, nil
	})
	if text := sendBackup(t, off, caller, backupAdminID); !strings.Contains(text, disabled) {
		t.Errorf("answer = %q, want the disabled notice", text)
	}
}

func TestBackupSurfacesAFailure(t *testing.T) {
	b, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		return BackupReport{Backup: true, Path: "/data/backups/sqlite3-20261006-043015.db", Kept: 1}, errors.New("disk is full")
	})

	text := sendBackup(t, b, caller, backupAdminID)
	if !strings.Contains(text, "disk is full") {
		t.Errorf("answer must name the failure, got %q", text)
	}
	if !strings.Contains(text, "/data/backups/sqlite3-20261006-043015.db") {
		t.Errorf("answer must still report what was written, got %q", text)
	}
}

func TestBackupRefusesANonAdmin(t *testing.T) {
	b, caller := backupTestBot(t, func(context.Context) (BackupReport, error) {
		t.Error("a stranger must not reach the backup task")
		return BackupReport{}, nil
	})

	const strangerID = int64(5150)
	if text := sendBackup(t, b, caller, strangerID); !strings.Contains(text, "Доступ запрещён") {
		t.Errorf("answer = %q, want the access refusal", text)
	}
}

func mustBot(t *testing.T) *Bot {
	t.Helper()

	b, _ := backupTestBot(t, nil)
	return b
}
