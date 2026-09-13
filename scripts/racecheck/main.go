package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/racecheck"
)

func main() {
	check := flag.Bool("check", false, "report whether the race detector can run here and exit")
	packages := flag.String("pkgs", "", "space separated package patterns (defaults to the full suite)")
	compiler := flag.String("cc", "", "path to the C compiler to use")
	flag.Parse()

	goos := goEnv("GOOS")
	resolved := *compiler
	if resolved == "" {
		resolved = findCompiler(goos)
	}

	env := racecheck.Environment{
		GOOS:       goos,
		CGOEnabled: goEnv("CGO_ENABLED") == "1",
		Compiler:   resolved,
		Packages:   strings.Fields(*packages),
	}
	if env.CGOEnabled && resolved != "" {
		env.ProbeError = probe(goos, resolved)
	}

	plan := racecheck.Evaluate(env)
	if plan.Runnable {
		fmt.Printf("race detector: ready — %s\n", plan.Reason)
	} else {
		fmt.Printf("race detector: unavailable — %s\n", plan.Reason)
	}
	if !plan.Runnable {
		for _, step := range plan.Steps {
			fmt.Printf("  %s\n", step)
		}
		fmt.Println("  the same suite runs under -race in CI: .github/workflows/ci.yml")
		os.Exit(2)
	}
	if *check {
		return
	}

	fmt.Printf("running %s\n", plan.Command)
	args := strings.Fields(strings.TrimPrefix(plan.Command, "go test "))
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), racecheck.ProbeEnv(goos, resolved)...)
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func probe(goos, compiler string) string {
	out := filepath.Join(os.TempDir(), "racecheck-probe")
	defer os.Remove(out)

	cmd := exec.Command("go", "build", "-race", "-o", out, racecheck.ProbePackage)
	cmd.Env = append(os.Environ(), racecheck.ProbeEnv(goos, compiler)...)
	combined, err := cmd.CombinedOutput()
	if err == nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(combined)), "\n")
	var pieces []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pieces = append(pieces, line)
		if len(pieces) == 2 {
			break
		}
	}
	if len(pieces) == 0 {
		return err.Error()
	}
	return strings.Join(pieces, "; ")
}

func goEnv(name string) string {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func findCompiler(goos string) string {
	for _, name := range racecheck.Compilers(goos) {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
