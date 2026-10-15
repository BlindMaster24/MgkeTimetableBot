package tests

import (
	"os"
	"path/filepath"
	"regexp"
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

func TestCITestsEveryPlatform(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	for _, runner := range []string{"ubuntu-latest", "windows-latest", "macos-latest"} {
		if !strings.Contains(workflow, runner) {
			t.Errorf("ci.yml must run on %s as well", runner)
		}
	}
	if !strings.Contains(workflow, "matrix:") {
		t.Error("the platform jobs must use a matrix, not a single runner")
	}
	if !strings.Contains(workflow, "fail-fast: false") {
		t.Error("one failing platform must not cancel the others")
	}
	if !strings.Contains(workflow, "shell: bash") {
		t.Error("the workflow must pin bash so one command string works on every platform")
	}
}

func TestCIRunsTheMonitoringCheck(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	if !strings.Contains(workflow, "monitoring:") {
		t.Error("ci.yml must keep a job that checks the monitoring assets")
	}
	if !strings.Contains(workflow, "go run ./scripts/monitoringcheck") {
		t.Error("the monitoring job must run the checked-in script so the local and CI commands cannot drift")
	}
	if !strings.Contains(workflow, "test rules deploy/monitoring/prometheus/rules/mgke.test.yml") {
		t.Error("the monitoring job must run the promtool unit tests, not only the offline checker")
	}

	version := regexp.MustCompile(`PROMTOOL_VERSION:\s*([0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(workflow)
	if version == nil {
		t.Fatal("the monitoring job must pin the promtool version explicitly")
	}
	if regexp.MustCompile(`PROMTOOL_SHA256:\s*[0-9a-f]{64}`).FindString(workflow) == "" {
		t.Fatal("the pinned promtool download must be verified against a sha256 checksum")
	}
	for _, want := range []string{"sha256sum --check", "v${PROMTOOL_VERSION}"} {
		if !strings.Contains(workflow, want) {
			t.Errorf("the monitoring job must download and verify the pinned archive through %q", want)
		}
	}

	for _, doc := range append(append([]string{}, docs...), filepath.Join("deploy", "monitoring", "README.md")) {
		if !strings.Contains(readRepoFile(t, doc), version[1]) {
			t.Errorf("%s does not document the promtool version %s the CI job installs", doc, version[1])
		}
	}
}

func TestCIRunsTheNewestToolchain(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	if !strings.Contains(workflow, "go-version: stable") {
		t.Error("CI must run the suite on the newest stable Go, not only on the pinned minimum")
	}
	if !strings.Contains(workflow, "GOTOOLCHAIN: local") {
		t.Error("the pinned toolchain must stay local so Go never substitutes one silently")
	}
}

func TestReleaseVerifiesEveryPlatform(t *testing.T) {
	release := readRepoFile(t, filepath.Join(".github", "workflows", "release.yml"))

	for _, runner := range []string{"ubuntu-latest", "windows-latest", "macos-latest"} {
		if !strings.Contains(release, runner) {
			t.Errorf("release.yml must verify the revision on %s too", runner)
		}
	}
}

func TestContainerSmokeTestChecksTheGracefulShutdown(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	if !strings.Contains(workflow, "docker stop --time") {
		t.Error("the container smoke test must stop the bot with a timeout so SIGTERM has a chance to arrive")
	}
	if !strings.Contains(workflow, "shutdown complete") {
		t.Error("the container smoke test must assert the bot finished its graceful shutdown")
	}
}

func TestContainerSmokeTestChecksTheImageFont(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	if !strings.Contains(workflow, "DejaVuSans.ttf") {
		t.Error("the container smoke test must prove the image can render the schedule PNGs")
	}
}

func TestSecurityWorkflowScansTheDependencies(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "security.yml"))

	if !strings.Contains(workflow, "govulncheck") {
		t.Error("the security workflow must run govulncheck")
	}
	if !strings.Contains(workflow, "schedule:") {
		t.Error("the vulnerability scan must run on a schedule, not only on push")
	}
	if !strings.Contains(workflow, "workflow_dispatch:") {
		t.Error("the vulnerability scan must be runnable by hand")
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
