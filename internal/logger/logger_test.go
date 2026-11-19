package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRotatedLogsAreCompressed(t *testing.T) {
	// A manual directory: lumberjack compresses a rotated file in the
	// background, and t.TempDir would fail its cleanup on the late write.
	dir, err := os.MkdirTemp("", "logger-rotation")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.RemoveAll(dir)
	})
	path := filepath.Join(dir, "bot.log")
	log := New("info", &FileConfig{Enabled: true, Path: path, MaxSizeMB: 1, MaxFiles: 2})
	defer log.Close()

	// One megabyte per file, so a few thousand lines cross the threshold.
	line := strings.Repeat("x", 1024)
	for i := 0; i < 3000; i++ {
		log.Info().Str("line", line).Msg("padding")
	}

	// lumberjack compresses a rotated file in the background, so poll briefly.
	deadline := time.Now().Add(10 * time.Second)
	for {
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.gz"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no compressed rotation found next to %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
