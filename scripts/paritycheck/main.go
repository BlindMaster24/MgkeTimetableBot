package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parity"
	"github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

const (
	defaultTSRef      = "go"
	goPackageDir      = "internal/telegram"
	localePath        = "internal/i18n/locales/ru.json"
	tsBotsRoot        = "src/services/bots"
	fixturePath       = "internal/telegram/testdata/parity/ts_surface.json"
	allowlistPath     = "internal/telegram/testdata/parity/known_differences.json"
	tsCommandsRoot    = tsBotsRoot + "/commands"
	tsCallbacksRoot   = tsBotsRoot + "/callbacks"
	tsKeyboardRoot    = tsBotsRoot + "/keyboard"
	literalLookBehind = 240
)

var (
	tgCommandExpr  = regexp.MustCompile(`command:\s*'([^']+)'`)
	payloadActExpr = regexp.MustCompile(`payloadAction:\s*string\s*=\s*'([^']+)'`)
	noYesSmileExpr = regexp.MustCompile(`noYesSmile\([^()]*,\s*$`)
)

func main() {
	chdirRepoRoot()

	tsRef := flag.String("ts-ref", defaultTSRef, "git ref holding the old TypeScript bot")
	update := flag.Bool("update", false, "rewrite the TypeScript surface fixture")
	allowlistFile := flag.String("allowlist", allowlistPath, "path to the known-differences allowlist")
	dumpGo := flag.Bool("dump-go", false, "print the live Go surface and exit")
	flag.Parse()

	if *dumpGo {
		printSurface(telegram.SurfaceOf())
		return
	}

	tsSurface, err := extractTypeScriptSurface(*tsRef)
	if err != nil {
		fail(err)
	}

	if *update {
		if err := parity.SaveSurface(fixturePath, tsSurface); err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s (%d commands, %d callbacks, %d buttons)\n",
			fixturePath, len(tsSurface.Commands), len(tsSurface.Callbacks), len(tsSurface.Buttons))
		return
	}

	recorded, err := parity.LoadSurface(fixturePath)
	if err != nil {
		fail(err)
	}
	if drift := parity.Compare(recorded, tsSurface, parity.Allowlist{}); len(drift) > 0 {
		fmt.Printf("the checked-in fixture is stale (%d differences vs %s); run with -update\n", len(drift), *tsRef)
		printDiffs(drift)
		os.Exit(1)
	}

	allow, err := parity.LoadAllowlist(*allowlistFile)
	if err != nil {
		fail(err)
	}

	goSurface, err := extractGoSurface()
	if err != nil {
		fail(err)
	}

	diffs := parity.Compare(tsSurface, goSurface, allow)
	if len(diffs) == 0 {
		fmt.Printf("parity ok: %d commands, %d callbacks, %d buttons match %s\n",
			len(tsSurface.Commands), len(tsSurface.Callbacks), len(tsSurface.Buttons), *tsRef)
		return
	}
	printDiffs(diffs)
	os.Exit(1)
}

func extractGoSurface() (parity.Surface, error) {
	locale, err := parity.LoadLocale(localePath)
	if err != nil {
		return parity.Surface{}, err
	}
	buttons, err := parity.GoButtonLiterals(goPackageDir, locale)
	if err != nil {
		return parity.Surface{}, err
	}

	live := telegram.SurfaceOf()
	return parity.Surface{
		Commands:  live.Commands,
		Callbacks: live.Callbacks,
		Buttons:   buttons,
	}, nil
}

func extractTypeScriptSurface(ref string) (parity.Surface, error) {
	files, err := gitList(ref, tsBotsRoot)
	if err != nil {
		return parity.Surface{}, err
	}

	var commands, callbacks, buttons []string
	var commandMatches, callbackMatches [][]string
	for _, file := range files {
		if !strings.HasSuffix(file, ".ts") {
			continue
		}
		src, err := gitShow(ref, file)
		if err != nil {
			return parity.Surface{}, err
		}
		src = parity.StripComments(src)
		switch {
		case strings.HasPrefix(file, tsCommandsRoot+"/"):
			commandMatches = append(commandMatches, tgCommandExpr.FindAllStringSubmatch(src, -1)...)
		case strings.HasPrefix(file, tsCallbacksRoot+"/"):
			callbackMatches = append(callbackMatches, payloadActExpr.FindAllStringSubmatch(src, -1)...)
		}
		if isKeyboardSource(file) {
			buttons = append(buttons, buttonLiteralsFrom(src)...)
		}
	}

	commands = append(commands, flatten(commandMatches)...)
	callbacks = append(callbacks, flatten(callbackMatches)...)
	return parity.Surface{
		Commands:  commands,
		Callbacks: callbacks,
		Buttons:   buttons,
	}, nil
}

func isKeyboardSource(file string) bool {
	return strings.HasPrefix(file, tsKeyboardRoot+"/") ||
		strings.HasPrefix(file, tsCommandsRoot+"/") ||
		strings.HasPrefix(file, tsCallbacksRoot+"/")
}

func buttonLiteralsFrom(src string) []string {
	var out []string
	for _, literal := range parity.StringLiteralsAt(src) {
		if isButtonLiteral(src, literal.Start) {
			out = append(out, literal.Value)
		}
	}
	return out
}

func isButtonLiteral(src string, start int) bool {
	from := start - literalLookBehind
	if from < 0 {
		from = 0
	}
	before := strings.TrimRight(src[from:start], " \t\r\n")
	if strings.HasSuffix(before, "text:") {
		return true
	}
	return noYesSmileExpr.MatchString(before)
}

func gitList(ref, dir string) ([]string, error) {
	out, err := runGit("ls-tree", "-r", "--name-only", ref, dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func gitShow(ref, file string) (string, error) {
	return runGit("show", ref+":"+file)
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func flatten(matches [][]string) []string {
	var out []string
	for _, match := range matches {
		if len(match) > 1 {
			out = append(out, match[1])
		}
	}
	return out
}

func printSurface(surface parity.Surface) {
	sorted := surface
	sort.Strings(sorted.Commands)
	sort.Strings(sorted.Callbacks)
	sort.Strings(sorted.Buttons)
	data, _ := json.MarshalIndent(sorted, "", "  ")
	fmt.Println(string(data))
}

func printDiffs(diffs []parity.Diff) {
	for _, diff := range diffs {
		fmt.Printf("  %s/%s: %q\n", diff.Section, diff.Kind, diff.Value)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "paritycheck:", err)
	fmt.Fprintln(os.Stderr, "run from the repository root; the go branch must be fetched (git fetch origin go)")
	os.Exit(1)
}

func chdirRepoRoot() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	if root := findRepoRoot(cwd); root != "" {
		_ = os.Chdir(root)
	}
}

func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
