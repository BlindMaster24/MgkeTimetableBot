package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
)

func TestDebugShowsTheBuildMetadata(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	b.SetBuildInfo(build.New("v1.2.3", "abcdef1234567890abcdef", "2026-09-13T08:00:00Z"))

	lines := strings.Join(b.debugLines(), "\n")
	for _, want := range []string{
		"-- Сборка --",
		"Версия: v1.2.3",
		"Коммит: abcdef1",
		"Собрано: 2026-09-13T08:00:00Z",
		"Toolchain: go",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the debug report omits %q:\n%s", want, lines)
		}
	}
}

func TestDebugMarksAnUnstampedBuild(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	b.SetBuildInfo(build.New("", "", ""))

	lines := strings.Join(b.debugLines(), "\n")
	if !strings.Contains(lines, "Версия: dev") || !strings.Contains(lines, "Коммит: unknown") {
		t.Errorf("a build without ldflags must be recognisable:\n%s", lines)
	}
}

func TestBuildInfoRoundTrip(t *testing.T) {
	b := setupTestBot(t)

	info := build.New("v2.0.0", "", "")
	b.SetBuildInfo(info)

	if got := b.BuildInfo(); got != info {
		t.Errorf("build info = %+v, want %+v", got, info)
	}
	if got := b.BuildInfo().Commit; got != "unknown" {
		t.Errorf("an empty commit must fall back to the placeholder, got %q", got)
	}
}
