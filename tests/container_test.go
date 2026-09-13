package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readFile(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestDockerfileKeepsTheTimezone(t *testing.T) {
	dockerfile := readFile(t, "Dockerfile")

	if !strings.Contains(dockerfile, "tzdata") {
		t.Error("the image must install tzdata, otherwise TZ cannot be resolved")
	}
	if !strings.Contains(dockerfile, "ca-certificates") {
		t.Error("the image must install ca-certificates for HTTPS calls")
	}
	if !strings.Contains(dockerfile, "ENV CONFIG_PATH") {
		t.Error("the image must keep the ENV block that the runtime relies on")
	}

	timezone := regexp.MustCompile(`(?m)^\s*TZ=([A-Za-z_]+/[A-Za-z_]+)`).FindStringSubmatch(dockerfile)
	if timezone == nil {
		t.Fatal("the image must set TZ to an IANA zone, otherwise days and lesson times are computed in UTC")
	}
	if timezone[1] == "UTC" {
		t.Errorf("TZ = %s, want the college timezone", timezone[1])
	}
	if !strings.Contains(dockerfile, "date") || !strings.Contains(dockerfile, "BUILD_DATE") {
		t.Error("the image must inject the build metadata through ldflags")
	}
}

func TestComposePassesTheTimezone(t *testing.T) {
	compose := readFile(t, "docker-compose.yml")

	if !regexp.MustCompile(`TZ:\s*"?\$\{TZ:-[A-Za-z_]+/[A-Za-z_]+\}"?`).MatchString(compose) {
		t.Error("docker-compose must pass TZ into the container so the service runs in the college timezone")
	}

	var parsed struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(compose), &parsed); err != nil {
		t.Fatalf("parse docker-compose.yml: %v", err)
	}

	env := parsed.Services["bot"].Environment
	if env == nil {
		t.Fatal("expected a bot service with an environment block")
	}
	if env["TZ"] == "" {
		t.Error("the bot service must set TZ")
	}
	if env["CONFIG_PATH"] == "" || env["MGKE_CHAT_DB_PATH"] == "" {
		t.Errorf("the bot service lost its other environment: %+v", env)
	}
}

func TestReleaseWorkflowsInjectTheBuildMetadata(t *testing.T) {
	release := readFile(t, filepath.Join(".github", "workflows", "release.yml"))

	for _, token := range []string{"-X main.version=", "-X main.commit=", "-X main.date=", "BUILD_DATE="} {
		if !strings.Contains(release, token) {
			t.Errorf("the release workflow must inject %s", token)
		}
	}
}
