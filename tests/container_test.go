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

func TestDockerfileShipsAFontForTheScheduleImages(t *testing.T) {
	dockerfile := readFile(t, "Dockerfile")

	if !strings.Contains(dockerfile, "font-dejavu") {
		t.Error("the image must install a font, otherwise the schedule PNG renderer has nothing to draw with")
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

func TestContainerSmokeTestRunsTheWebhookTransport(t *testing.T) {
	workflow := readFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	for _, token := range []string{
		"mgke-webhook",
		"MGKE_TELEGRAM_API_BASE_URL",
		"MGKE_TELEGRAM_WEBHOOK_ENABLED",
		"MGKE_TELEGRAM_WEBHOOK_SECRET_TOKEN",
		"X-Telegram-Bot-Api-Secret-Token",
		"scripts/tgstub",
		"setWebhook",
		"sendMessage",
		"answerCallbackQuery",
		"editMessageText",
		"testdata/webhook_callback_query.json",
	} {
		if !strings.Contains(workflow, token) {
			t.Errorf("the container job must also run the webhook transport, missing %q", token)
		}
	}

	if !strings.Contains(workflow, "(.body.message_id == 909090)") {
		t.Error("the container job must prove the inline button edited the message id carried by the update, not a stored one")
	}

	payload := readFile(t, filepath.Join("internal", "telegram", "testdata", "webhook_callback_query.json"))
	if !strings.Contains(payload, "\"message_id\": 909090") {
		t.Error("the callback payload the container job posts must carry the message id the job waits for")
	}

	if strings.Count(workflow, "docker stop --time") < 2 {
		t.Error("both container runs must stop the bot with a timeout so SIGTERM has a chance to arrive")
	}
	if strings.Count(workflow, "shutdown complete") < 2 {
		t.Error("both container runs must assert the graceful shutdown, the webhook listener included")
	}
}

func TestContainerSmokeTestPollsOnlyTheStubBotAPI(t *testing.T) {
	workflow := strings.ReplaceAll(readFile(t, filepath.Join(".github", "workflows", "ci.yml")), "\r\n", "\n")

	container := workflowJob(t, workflow, "container")

	if strings.Contains(container, "api.telegram.org") {
		t.Error("the container job must not depend on the real Bot API")
	}

	stub := strings.Index(container, "nohup /tmp/tgstub")
	poll := strings.Index(container, "docker run -d --name mgke-smoke")
	if stub < 0 || poll < 0 {
		t.Fatal("the container job must start the stub Bot API and the long polling container")
	}
	if stub > poll {
		t.Error("the stub Bot API must be ready before the long polling container starts")
	}

	block := container[poll:]
	if end := strings.Index(block, "\n\n      - name:"); end >= 0 {
		block = block[:end]
	}
	for _, want := range []string{"--network host", `MGKE_TELEGRAM_API_BASE_URL="http://127.0.0.1:18090"`} {
		if !strings.Contains(block, want) {
			t.Errorf("the long polling container must set %s so it never reaches the real Bot API", want)
		}
	}

	if strings.Count(container, `MGKE_TELEGRAM_API_BASE_URL="http://127.0.0.1:18090"`) < 2 {
		t.Error("both container runs must point at the stub Bot API")
	}
	if !strings.Contains(container, `select(.method == "getUpdates")`) {
		t.Error("the container job must prove the long polling loop polled the stub, not only that it logged a start")
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
