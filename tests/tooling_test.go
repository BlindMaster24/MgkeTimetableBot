package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/racecheck"
)

func readRepoFile(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestCIRunsTheRaceCheckScript(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	if !strings.Contains(workflow, "go run ./scripts/racecheck") {
		t.Error("the race job must run the checked-in script so the local and CI commands cannot drift")
	}
	if !strings.Contains(workflow, "-race") {
		t.Error("CI must keep a job that runs the race detector")
	}
	if strings.Contains(workflow, "go test -count=1 -p 1 -race") {
		t.Error("the race command must live in scripts/racecheck, not inline in the workflow")
	}
}

func TestRaceCommandCoversTheWholeSuite(t *testing.T) {
	packages := racecheck.DefaultPackages()
	joined := strings.Join(packages, " ")

	for _, pkg := range []string{"./internal/...", "./tests/...", "./cmd/..."} {
		if !strings.Contains(joined, pkg) {
			t.Errorf("the race command must cover %s, got %q", pkg, joined)
		}
	}
	if !strings.HasPrefix(racecheck.CommandPrefix, "go test") {
		t.Errorf("command prefix = %q", racecheck.CommandPrefix)
	}
}
