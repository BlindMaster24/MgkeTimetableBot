package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsDefaultsStayOffAndKeepTheStandardPath(t *testing.T) {
	cfg := archiveBase()

	if cfg.Metrics.Enabled {
		t.Error("metrics must stay off until the operator turns them on")
	}
	if cfg.Metrics.Token != "" {
		t.Errorf("token = %q, want an open endpoint by default", cfg.Metrics.Token)
	}
	if got := cfg.ResolvedMetricsPath(); got != DefaultMetricsPath {
		t.Errorf("path = %q, want the default %q", got, DefaultMetricsPath)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("a disabled metrics section must validate: %v", err)
	}
}

func TestMetricsPathFallsBackToTheDefault(t *testing.T) {
	cfg := archiveBase()
	cfg.Metrics.Enabled = true
	cfg.Metrics.Path = "   "

	if got := cfg.ResolvedMetricsPath(); got != DefaultMetricsPath {
		t.Errorf("path = %q, want the default %q", got, DefaultMetricsPath)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("a blank path must fall back to the default: %v", err)
	}
}

func TestMetricsValidation(t *testing.T) {
	relative := archiveBase()
	relative.Metrics.Enabled = true
	relative.Metrics.Path = "metrics"
	if err := relative.Validate(); err == nil || !strings.Contains(err.Error(), "metrics.path") {
		t.Errorf("a relative path must be rejected, got %v", err)
	}

	bare := archiveBase()
	bare.Metrics.Enabled = true
	bare.Metrics.Path = "/"
	if err := bare.Validate(); err == nil || !strings.Contains(err.Error(), "metrics.path") {
		t.Errorf("the root path must be rejected, got %v", err)
	}

	insideAPI := archiveBase()
	insideAPI.Metrics.Enabled = true
	insideAPI.Metrics.Path = "/api/metrics"
	if err := insideAPI.Validate(); err == nil || !strings.Contains(err.Error(), "metrics.path") {
		t.Errorf("a path behind the API key gate must be rejected, got %v", err)
	}

	shortToken := archiveBase()
	shortToken.Metrics.Enabled = true
	shortToken.Metrics.Token = "short"
	if err := shortToken.Validate(); err == nil || !strings.Contains(err.Error(), "metrics.token") {
		t.Errorf("a short token must be rejected, got %v", err)
	}

	unused := archiveBase()
	unused.Metrics.Path = "metrics"
	unused.Metrics.Token = "short"
	if err := unused.Validate(); err != nil {
		t.Errorf("a disabled metrics section must not be validated: %v", err)
	}
}

func TestMetricsSectionComesFromTheEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  port: 8081\ntelegram:\n  token: \"token\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithEnv(path, envLookup(map[string]string{
		"MGKE_METRICS_ENABLED": "true",
		"MGKE_METRICS_PATH":    "/internal/metrics",
		"MGKE_METRICS_TOKEN":   "scrape-me-please",
	}))
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}

	if !cfg.Metrics.Enabled || cfg.ResolvedMetricsPath() != "/internal/metrics" {
		t.Errorf("metrics settings = %+v", cfg.Metrics)
	}
	if cfg.Metrics.Token != "scrape-me-please" {
		t.Errorf("token = %q", cfg.Metrics.Token)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("the environment values must validate: %v", err)
	}
}
