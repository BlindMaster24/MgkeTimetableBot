package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	log := New("info", nil)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerSub(t *testing.T) {
	log := New("debug", nil)
	sub := log.Sub("parser")
	if sub == nil {
		t.Fatal("expected non-nil sub logger")
	}

	e := sub.Info()
	if e == nil {
		t.Error("expected non-nil event")
	}
}

func TestLoggerLevels(t *testing.T) {
	log := New("trace", nil)

	if log.Debug() == nil {
		t.Error("expected debug event")
	}
	if log.Info() == nil {
		t.Error("expected info event")
	}
	if log.Warn() == nil {
		t.Error("expected warn event")
	}
	if log.Error() == nil {
		t.Error("expected error event")
	}
}

func TestLoggerWithFile(t *testing.T) {
	cfg := &FileConfig{
		Enabled:   false,
		Path:      t.TempDir() + "/test.log",
		MaxSizeMB: 1,
		MaxFiles:  1,
	}
	log := New("info", cfg)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerCloseReleasesTheLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	log := New("info", &FileConfig{Enabled: true, Path: path, MaxSizeMB: 1, MaxFiles: 1})

	log.Info().Msg("hello")
	if err := log.Close(); err != nil {
		t.Fatalf("close the file logger: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the log file: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Error("the file logger did not write the entry")
	}
	if err := os.Remove(path); err != nil {
		t.Errorf("the closed logger still holds the file: %v", err)
	}
	if err := New("info", nil).Close(); err != nil {
		t.Errorf("closing a console logger returned %v", err)
	}
}

func TestLoggerInvalidLevel(t *testing.T) {
	log := New("invalid_level", nil)
	if log == nil {
		t.Fatal("expected non-nil logger (should default to info)")
	}
}
