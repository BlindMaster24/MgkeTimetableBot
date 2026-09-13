package racecheck

import (
	"fmt"
	"strings"
)

const (
	CommandPrefix = "go test -count=1 -p 1 -race"
	ProbePackage  = "./internal/build"
)

type Environment struct {
	GOOS       string
	CGOEnabled bool
	Compiler   string
	ProbeError string
	Packages   []string
}

type Plan struct {
	Runnable bool
	Command  string
	Reason   string
	Steps    []string
}

func DefaultPackages() []string {
	return []string{"./internal/...", "./tests/...", "./cmd/..."}
}

func Compilers(goos string) []string {
	switch goos {
	case "windows":
		return []string{"gcc", "x86_64-w64-mingw32-gcc"}
	case "darwin":
		return []string{"clang", "cc", "gcc"}
	}
	return []string{"gcc", "cc", "clang"}
}

func ProbeEnv(goos, compiler string) []string {
	if goos == "windows" && strings.Contains(compiler, " ") {
		return []string{"CGO_ENABLED=1"}
	}
	return []string{"CGO_ENABLED=1", "CC=" + compiler}
}

func Evaluate(env Environment) Plan {
	packages := env.Packages
	if len(packages) == 0 {
		packages = DefaultPackages()
	}

	plan := Plan{Command: fmt.Sprintf("%s %s", CommandPrefix, strings.Join(packages, " "))}

	switch {
	case !env.CGOEnabled && env.Compiler == "":
		plan.Reason = "the race detector needs cgo, but CGO_ENABLED=0 and no C compiler is on PATH"
		plan.Steps = append([]string{"set CGO_ENABLED=1 for the run"}, InstallSteps(env.GOOS)...)
	case !env.CGOEnabled:
		plan.Reason = fmt.Sprintf("the race detector needs cgo, but CGO_ENABLED=0 (found %s)", env.Compiler)
		plan.Steps = []string{"set CGO_ENABLED=1 for the run"}
	case env.Compiler == "":
		plan.Reason = "cgo is enabled, but no C compiler was found on PATH"
		plan.Steps = InstallSteps(env.GOOS)
	case env.ProbeError != "":
		plan.Reason = "the toolchain cannot build a race binary: " + env.ProbeError
		plan.Steps = append(ProbeSteps(env), InstallSteps(env.GOOS)...)
	default:
		plan.Runnable = true
		plan.Reason = fmt.Sprintf("cgo is enabled and %s builds a race binary", env.Compiler)
	}

	return plan
}

func ProbeSteps(env Environment) []string {
	if env.GOOS == "windows" && strings.Contains(env.Compiler, " ") {
		return []string{fmt.Sprintf("put the directory of %s on PATH instead of passing a path with spaces", env.Compiler)}
	}
	return []string{"check that the C toolchain ships its headers and can link, not just compile"}
}

func InstallSteps(goos string) []string {
	switch goos {
	case "windows":
		return []string{
			"install a C toolchain: winget install -e --id MSYS2.MSYS2, then pacman -S mingw-w64-x86_64-gcc",
			"or: choco install mingw -y",
			"make sure the toolchain bin directory is on PATH, then reopen the shell",
		}
	case "darwin":
		return []string{"install the command line tools: xcode-select --install"}
	}
	return []string{"install a C toolchain: sudo apt-get install -y build-essential"}
}
