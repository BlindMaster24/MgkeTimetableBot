package logger

import (
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

func TestLoggerInvalidLevel(t *testing.T) {
	log := New("invalid_level", nil)
	if log == nil {
		t.Fatal("expected non-nil logger (should default to info)")
	}
}
