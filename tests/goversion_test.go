package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var pinnedGoVersionFiles = []string{
	"go.mod",
	"README.md",
	"README.en.md",
	"AGENTS.md",
	"docs/google-calendar.md",
}

var skippedGoVersionDirs = map[string]bool{
	"old":          true,
	"node_modules": true,
	"cache":        true,
	"data":         true,
}

var (
	goVersionPinRe       = regexp.MustCompile(`(?i)\bgo(?:lang)?(?:\s+version)?[^0-9A-Za-z]{0,12}(\d+\.\d+(?:\.\d+)?)`)
	hardcodedToolchainRe = regexp.MustCompile(`golang:[0-9]`)
	pinnedBuildArgRe     = regexp.MustCompile(`GO_VERSION=\$\{\{ steps\.([A-Za-z0-9_-]+)\.outputs\.go-version \}\}`)
)

type goVersionMention struct {
	line    int
	version string
}

type toolchainStep struct {
	ID   string `yaml:"id"`
	Uses string `yaml:"uses"`
	With struct {
		GoVersionFile string `yaml:"go-version-file"`
		BuildArgs     string `yaml:"build-args"`
	} `yaml:"with"`
}

type toolchainWorkflow struct {
	Jobs map[string]struct {
		Steps []toolchainStep `yaml:"steps"`
	} `yaml:"jobs"`
}

type toolchainCompose struct {
	Services map[string]struct {
		Build *struct {
			Args map[string]string `yaml:"args"`
		} `yaml:"build"`
	} `yaml:"services"`
}

func TestEveryGoVersionMentionMatchesTheGoModPin(t *testing.T) {
	pinned := pinnedGoVersion(t)

	for _, name := range pinnedGoVersionFiles {
		mentions := goVersionMentionsIn(t, name)
		if len(mentions) == 0 {
			t.Errorf("%s no longer repeats the pinned Go version %s; this guard goes blind when a file drops its mention", name, pinned)
			continue
		}
		reportMismatches(t, name, mentions, pinned)
	}

	for _, path := range goVersionSurfaceFiles(t) {
		name := repoRelativePath(path)
		if knownGoVersionFile(name) {
			continue
		}
		reportMismatches(t, name, goVersionMentionsIn(t, name), pinned)
	}
}

func TestNoFileHardcodesTheToolchainImage(t *testing.T) {
	for _, path := range goVersionSurfaceFiles(t) {
		name := repoRelativePath(path)
		if hardcodedToolchainRe.MatchString(readRepoFile(t, name)) {
			t.Errorf("%s hardcodes the builder image tag; take the toolchain from go.mod through the GO_VERSION build argument so a version bump cannot leave it behind", name)
		}
	}
}

func TestImageBuildsTakeTheToolchainFromGoMod(t *testing.T) {
	dockerfile := readRepoFile(t, "Dockerfile")
	if !hasLine(dockerfile, "ARG GO_VERSION") {
		t.Error("the Dockerfile must declare the GO_VERSION build argument the builder tag is built from")
	}
	if !strings.Contains(dockerfile, "golang:${GO_VERSION}-alpine") {
		t.Error("the Dockerfile builder image must be tagged with golang:${GO_VERSION}-alpine")
	}

	builds := 0

	workflows, err := filepath.Glob(filepath.Join("..", ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatalf("glob the workflows: %v", err)
	}
	for _, path := range workflows {
		name := repoRelativePath(path)

		var parsed toolchainWorkflow
		if err := yaml.Unmarshal([]byte(readRepoFile(t, name)), &parsed); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		for jobName, job := range parsed.Jobs {
			for _, step := range job.Steps {
				if step.With.BuildArgs == "" {
					continue
				}
				builds++

				match := pinnedBuildArgRe.FindStringSubmatch(step.With.BuildArgs)
				if match == nil {
					t.Errorf("%s job %s builds an image without passing the toolchain version it read from go.mod", name, jobName)
					continue
				}
				if !jobPinsTheToolchainFromGoMod(job.Steps, match[1]) {
					t.Errorf("%s job %s takes GO_VERSION from step %q, which is not a setup-go step reading go.mod", name, jobName, match[1])
				}
			}
		}
	}

	composeFiles, err := filepath.Glob(filepath.Join("..", "docker-compose*.yml"))
	if err != nil {
		t.Fatalf("glob the compose files: %v", err)
	}
	for _, path := range composeFiles {
		name := repoRelativePath(path)

		var parsed toolchainCompose
		if err := yaml.Unmarshal([]byte(readRepoFile(t, name)), &parsed); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		for serviceName, service := range parsed.Services {
			if service.Build == nil {
				continue
			}
			builds++

			if !strings.Contains(service.Build.Args["GO_VERSION"], "${GO_VERSION") {
				t.Errorf("%s service %s builds the image without taking GO_VERSION from the environment, so the toolchain cannot come from go.mod", name, serviceName)
			}
		}
	}

	if builds == 0 {
		t.Error("no workflow or compose file builds the image any more; this guard would pass without checking anything")
	}
}

func hasLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func jobPinsTheToolchainFromGoMod(steps []toolchainStep, id string) bool {
	for _, step := range steps {
		if step.ID == id && strings.HasPrefix(step.Uses, "actions/setup-go") && step.With.GoVersionFile == "go.mod" {
			return true
		}
	}
	return false
}

func reportMismatches(t *testing.T, name string, mentions []goVersionMention, pinned string) {
	t.Helper()

	for _, mention := range mentions {
		if mention.version != pinned {
			t.Errorf("%s:%d names Go %s while go.mod pins %s; a bump must touch every mention and the docs together", name, mention.line, mention.version, pinned)
		}
	}
}

func goVersionMentionsIn(t *testing.T, name string) []goVersionMention {
	t.Helper()

	var mentions []goVersionMention
	for number, line := range strings.Split(readRepoFile(t, name), "\n") {
		for _, match := range goVersionPinRe.FindAllStringSubmatch(line, -1) {
			mentions = append(mentions, goVersionMention{line: number + 1, version: match[1]})
		}
	}
	return mentions
}

func pinnedGoVersion(t *testing.T) string {
	t.Helper()

	for _, line := range strings.Split(readRepoFile(t, "go.mod"), "\n") {
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	t.Fatal("go.mod has no go directive to pin the toolchain")
	return ""
}

func goVersionSurfaceFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir("..", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path == ".." {
				return nil
			}
			if name == ".github" {
				return nil
			}
			if strings.HasPrefix(name, ".") || skippedGoVersionDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if carriesGoVersion(name) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the tree: %v", err)
	}
	return files
}

func carriesGoVersion(name string) bool {
	if name == "go.mod" || strings.HasPrefix(name, "Dockerfile") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".yml", ".yaml":
		return true
	}
	return false
}

func knownGoVersionFile(name string) bool {
	for _, known := range pinnedGoVersionFiles {
		if filepath.ToSlash(known) == name {
			return true
		}
	}
	return false
}

func repoRelativePath(path string) string {
	return filepath.ToSlash(strings.TrimPrefix(path, ".."+string(os.PathSeparator)))
}
