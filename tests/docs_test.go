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
	server := api.NewServer(raspCache, 0, health.NewDefaultTracker(), build.New("test", "", ""))
	server.HandleGoogleOAuth("/google/oauth", nil)

	engine, ok := server.Handler().(*gin.Engine)
	if !ok {
		t.Fatal("expected a gin engine")
	}

	routes := engine.Routes()
	if len(routes) < 7 {
		t.Fatalf("expected the API surface, got %d routes", len(routes))
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
		if !strings.Contains(readDoc(t, doc), version) {
			t.Errorf("%s does not state the minimum Go version %s", doc, version)
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
