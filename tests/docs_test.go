package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

var docs = []string{"README.md", "README.en.md"}

func readDoc(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func mentionsDoc(text, token string) bool {
	pattern := regexp.MustCompile(regexp.QuoteMeta(token) + `([^A-Za-z0-9_:]|$)`)
	return pattern.MatchString(text)
}

func TestDocsListEveryCommand(t *testing.T) {
	commands := telegram.SurfaceOfCommands()
	if len(commands) < 50 {
		t.Fatalf("expected the full command surface, got %d", len(commands))
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, command := range commands {
			if !mentionsDoc(content, command) {
				t.Errorf("%s does not document the %s command", doc, command)
			}
		}
	}
}

func TestDocsListEveryAPIEndpoint(t *testing.T) {
	raspCache, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}

	gin.SetMode(gin.TestMode)
	server := api.NewServer(raspCache, 0, health.NewDefaultTracker(), build.New("test", "", ""), nil, nil)
	server.SetMetricsConfig(api.MetricsConfig{})
	server.HandleGoogleOAuth("/google/oauth", nil)

	engine, ok := server.Handler().(*gin.Engine)
	if !ok {
		t.Fatal("expected a gin engine")
	}

	routes := engine.Routes()
	if len(routes) < 8 {
		t.Fatalf("expected the API surface plus the metrics route, got %d routes", len(routes))
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, route := range routes {
			if !strings.Contains(content, route.Path) {
				t.Errorf("%s does not document the %s endpoint", doc, route.Path)
			}
		}
	}
}

func TestDocsStateTheGoVersion(t *testing.T) {
	module, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	version := ""
	for _, line := range strings.Split(string(module), "\n") {
		if strings.HasPrefix(line, "go ") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "go "))
			break
		}
	}
	if version == "" {
		t.Fatal("go.mod has no go directive")
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		if !strings.Contains(content, version) {
			t.Errorf("%s does not state the minimum Go version %s", doc, version)
			continue
		}

		for _, mention := range goVersionMentions(content) {
			if mention != version {
				t.Errorf("%s mentions Go %s while go.mod pins %s", doc, mention, version)
			}
		}
	}
}

var (
	goVersionMentionRe = regexp.MustCompile(`(?i)\bgo[ :]?(\d+\.\d+(?:\.\d+)?)|(?i)\bgo1\.(\d+\.\d+(?:\.\d+)?)`)
	concreteImageTagRe = regexp.MustCompile(`ghcr\.io/[^\s:]+:[0-9]`)
)

func goVersionMentions(content string) []string {
	var mentions []string
	for _, match := range goVersionMentionRe.FindAllStringSubmatch(content, -1) {
		for _, group := range match[1:] {
			if group != "" {
				mentions = append(mentions, group)
			}
		}
	}
	return mentions
}

func TestDocsDoNotPinAReleaseVersion(t *testing.T) {
	for _, doc := range docs {
		if tag := concreteImageTagRe.FindString(readDoc(t, doc)); tag != "" {
			t.Errorf("%s pins the concrete image tag %q; link the releases page and the GHCR package instead", doc, tag)
		}
	}
}

func TestDocsCoverEveryConfigKey(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse config example: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("config example is empty")
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for key := range raw {
			if !strings.Contains(content, key) {
				t.Errorf("%s does not document the %s config key", doc, key)
			}
		}
	}
}

func TestDocsDocumentReminders(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	for _, token := range []string{"reminder_lead_minutes", "reminder_jitter_seconds"} {
		if !strings.Contains(string(data), token) {
			t.Errorf("config.example.yaml does not carry the %s key", token)
		}
	}
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"reminder_lead_minutes", "reminder_jitter_seconds"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the %s config key", doc, token)
			}
		}
		if !strings.Contains(content, "⏰ Напоминания") {
			t.Errorf("%s does not document the per-chat reminder menu", doc)
		}
	}
}

func TestDocsDocumentFuzzing(t *testing.T) {
	targets := []string{
		"FuzzGroupParserStaysStableAndClean",
		"FuzzWebhookHandlerKeepsStatusesSane",
		"FuzzDiffKeepsChangesWithinTheDays",
		"FuzzSearchStaysOrderedAndBounded",
		"FuzzCurrentAndUpcomingStayInsideTheDay",
		"FuzzKeyIsStableAndUniverseKeepsOneLabel",
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		if !strings.Contains(content, "go test -fuzz") {
			t.Errorf("%s does not document how to fuzz", doc)
		}
		for _, target := range targets {
			if !strings.Contains(content, target) {
				t.Errorf("%s does not document the %s fuzz target", doc, target)
			}
		}
	}
}

func TestDocsDocumentArchiveMaintenance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}

	keys := []string{"archive.backup_dir", "archive.backup_keep", "archive.retention_days", "archive.maintenance_schedule"}
	for _, key := range keys {
		field := strings.TrimPrefix(key, "archive.")
		if !strings.Contains(string(data), field+":") {
			t.Errorf("config.example.yaml does not carry the archive.%s key", field)
		}
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, key := range keys {
			if !strings.Contains(content, key) {
				t.Errorf("%s does not document the %s config key", doc, key)
			}
		}
	}
}

func TestDocsDocumentSQLiteTuningAndBackupAlerts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	for _, field := range []string{"busy_timeout_ms:", "busy_retries:", "backup_failures:"} {
		if !strings.Contains(string(data), field) {
			t.Errorf("config.example.yaml does not carry the %s key", field)
		}
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{
			"sqlite.busy_timeout_ms",
			"sqlite.busy_retries",
			"health.backup_failures",
			"backup_failed",
			"mgke_archive_busy_retries_total",
			"mgke_archive_busy_failures_total",
			"mgke_maintenance_failures",
			"mgke_maintenance_last_success_seconds",
		} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentPrometheusMetrics(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}

	keys := []string{"metrics.enabled", "metrics.path", "metrics.token"}
	for _, key := range keys {
		field := strings.TrimPrefix(key, "metrics.")
		if !strings.Contains(string(data), field+":") {
			t.Errorf("config.example.yaml does not carry the metrics.%s key", field)
		}
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, key := range keys {
			if !strings.Contains(content, key) {
				t.Errorf("%s does not document the %s config key", doc, key)
			}
		}
		for _, token := range []string{api.DefaultMetricsPath, "Prometheus", "MGKE_METRICS_TOKEN"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the metrics token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentTheMonitoringStack(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{
			"deploy/monitoring",
			"deploy/monitoring/README.md",
			"docker-compose.monitoring.yml",
			"--profile alerts",
			"deploy/monitoring/prometheus/rules/mgke.yml",
			"promtool check rules",
			"promtool test rules",
			"deploy/monitoring/grafana/dashboards/mgke-timetablebot.json",
			"deploy/monitoring/kubernetes/servicemonitor.yaml",
			"deploy/monitoring/kubernetes/prometheusrule.yaml",
			"deploy/monitoring/alloy/config.alloy",
			"MGKE_METRICS_TARGET",
			"scripts/monitoringcheck",
			"go run ./scripts/monitoringcheck",
			"-skip-promtool",
			"NOT VERIFIED",
			"| `monitoring` |",
		} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the monitoring token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentTheRaceDetector(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"scripts/racecheck", "CGO_ENABLED=1", "-race"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the race detector token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentTheIncidentHistory(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"/incidents", "health.incidents"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the incident history token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentTheAPIProbe(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"🔌 Проверить API", "api_slow_ms"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the API probe token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentThePreflightCheck(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"scripts/preflight", "-skip-site"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the preflight token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentTheMessageGoldens(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, token := range []string{"messages.golden", "go test ./internal/notification -update"} {
			if !strings.Contains(content, token) {
				t.Errorf("%s does not document the message golden token %q", doc, token)
			}
		}
	}
}

func TestDocsDocumentEnvironmentOverrides(t *testing.T) {
	for _, doc := range docs {
		content := readDoc(t, doc)
		if !strings.Contains(content, config.EnvPrefix+"_") {
			t.Errorf("%s does not document the %s_ environment prefix", doc, config.EnvPrefix)
		}
		if !strings.Contains(content, "CONFIG_PATH") {
			t.Errorf("%s does not document CONFIG_PATH", doc)
		}
	}
}

func TestDocsAreLinkedBothWays(t *testing.T) {
	russian := readDoc(t, "README.md")
	english := readDoc(t, "README.en.md")

	if !strings.Contains(russian, "README.en.md") {
		t.Error("README.md does not link to README.en.md")
	}
	if !strings.Contains(english, "README.md") {
		t.Error("README.en.md does not link back to README.md")
	}
}

var (
	readmePackageLine = regexp.MustCompile(`(?m)^  ([a-z0-9]+)/\s+—`)
	readmeCommandLine = regexp.MustCompile(`(?m)^cmd/([a-z0-9-]+)/`)
)

// TestReadmeStructureListsEveryPackage keeps the project layout honest: a new
// internal package or command must reach both READMEs, and a removed one must
// leave them.
func TestReadmeStructureListsEveryPackage(t *testing.T) {
	packages := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join("..", "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			packages[entry.Name()] = true
		}
	}

	commands := map[string]bool{}
	cmdEntries, err := os.ReadDir(filepath.Join("..", "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range cmdEntries {
		if entry.IsDir() {
			commands[entry.Name()] = true
		}
	}

	for _, doc := range []string{"README.md", "README.en.md"} {
		content := readRepoFile(t, doc)

		listed := map[string]bool{}
		for _, match := range readmePackageLine.FindAllStringSubmatch(content, -1) {
			listed[match[1]] = true
			if !packages[match[1]] {
				t.Errorf("%s lists internal/%s, which does not exist", doc, match[1])
			}
		}
		for pkg := range packages {
			if !listed[pkg] {
				t.Errorf("%s does not list internal/%s", doc, pkg)
			}
		}

		listedCommands := map[string]bool{}
		for _, match := range readmeCommandLine.FindAllStringSubmatch(content, -1) {
			listedCommands[match[1]] = true
			if !commands[match[1]] {
				t.Errorf("%s lists cmd/%s, which does not exist", doc, match[1])
			}
		}
		for command := range commands {
			if !listedCommands[command] {
				t.Errorf("%s does not list cmd/%s", doc, command)
			}
		}
	}
}

// TestEveryPackageHasADocComment keeps `go doc` useful: each package starts
// with the sentence that explains it.
func TestEveryPackageHasADocComment(t *testing.T) {
	for _, root := range []string{filepath.Join("..", "internal"), filepath.Join("..", "cmd")} {
		dirs, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			path := filepath.Join(root, dir.Name())
			pattern := filepath.Join(path, "*.go")
			files, err := filepath.Glob(pattern)
			if err != nil || len(files) == 0 {
				continue
			}
			if !hasPackageDoc(t, files) {
				t.Errorf("%s has no package doc comment", path)
			}
		}
	}
}

func hasPackageDoc(t *testing.T, files []string) bool {
	t.Helper()

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "package ") {
				return false
			}
			if strings.HasPrefix(line, "// Package ") || strings.HasPrefix(line, "// Command ") {
				return true
			}
		}
	}
	return false
}
