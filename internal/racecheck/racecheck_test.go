package racecheck

import (
	"strings"
	"testing"
)

func TestEvaluateIsRunnableWithCgoAndACompiler(t *testing.T) {
	plan := Evaluate(Environment{GOOS: "linux", CGOEnabled: true, Compiler: "/usr/bin/gcc"})

	if !plan.Runnable {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Command != "go test -count=1 -p 1 -race ./internal/... ./tests/... ./cmd/..." {
		t.Errorf("command = %q", plan.Command)
	}
	if !strings.Contains(plan.Reason, "/usr/bin/gcc") {
		t.Errorf("reason must name the compiler: %q", plan.Reason)
	}
}

func TestEvaluateReportsMissingCgo(t *testing.T) {
	plan := Evaluate(Environment{GOOS: "windows", CGOEnabled: false, Compiler: "C:/mingw/bin/gcc.exe"})

	if plan.Runnable {
		t.Fatalf("plan must not be runnable: %+v", plan)
	}
	if !strings.Contains(plan.Reason, "CGO_ENABLED=0") {
		t.Errorf("reason = %q", plan.Reason)
	}
	if len(plan.Steps) == 0 || !strings.Contains(plan.Steps[0], "CGO_ENABLED=1") {
		t.Errorf("the first step must enable cgo: %+v", plan.Steps)
	}
}

func TestEvaluateReportsAMissingCompiler(t *testing.T) {
	plan := Evaluate(Environment{GOOS: "linux", CGOEnabled: true})

	if plan.Runnable {
		t.Fatalf("plan must not be runnable: %+v", plan)
	}
	if !strings.Contains(plan.Reason, "no C compiler") {
		t.Errorf("reason = %q", plan.Reason)
	}
	if len(plan.Steps) == 0 || !strings.Contains(plan.Steps[0], "build-essential") {
		t.Errorf("linux must hint at build-essential: %+v", plan.Steps)
	}
}

func TestEvaluateRejectsAToolchainThatCannotBuildRaces(t *testing.T) {
	plan := Evaluate(Environment{
		GOOS:       "windows",
		CGOEnabled: true,
		Compiler:   "C:/Program Files/LLVM/bin/clang.exe",
		ProbeError: "stdlib.h file not found",
	})

	if plan.Runnable {
		t.Fatalf("a compiler without headers must not count as runnable: %+v", plan)
	}
	if !strings.Contains(plan.Reason, "stdlib.h file not found") {
		t.Errorf("reason = %q", plan.Reason)
	}
	joined := strings.Join(plan.Steps, "\n")
	if !strings.Contains(joined, "PATH") {
		t.Errorf("a spaced compiler path needs the PATH hint: %+v", plan.Steps)
	}
}

func TestProbeEnvSkipsACompilerPathWithSpaces(t *testing.T) {
	env := ProbeEnv("windows", "C:/Program Files/LLVM/bin/clang.exe")
	if len(env) != 1 || env[0] != "CGO_ENABLED=1" {
		t.Errorf("windows env = %v", env)
	}

	env = ProbeEnv("linux", "/usr/bin/gcc")
	if len(env) != 2 || env[1] != "CC=/usr/bin/gcc" {
		t.Errorf("linux env = %v", env)
	}
}

func TestEvaluateCollectsBothProblems(t *testing.T) {
	plan := Evaluate(Environment{GOOS: "windows"})

	if plan.Runnable {
		t.Fatalf("plan must not be runnable: %+v", plan)
	}
	if !strings.Contains(plan.Reason, "no C compiler") {
		t.Errorf("reason = %q", plan.Reason)
	}
	joined := strings.Join(plan.Steps, "\n")
	if !strings.Contains(joined, "CGO_ENABLED=1") || !strings.Contains(joined, "mingw") {
		t.Errorf("windows steps must enable cgo and name mingw: %+v", plan.Steps)
	}
}

func TestInstallStepsPerPlatform(t *testing.T) {
	if steps := InstallSteps("darwin"); len(steps) != 1 || !strings.Contains(steps[0], "xcode-select") {
		t.Errorf("darwin steps = %+v", steps)
	}
	if steps := InstallSteps("windows"); !strings.Contains(strings.Join(steps, " "), "MSYS2") {
		t.Errorf("windows steps = %+v", steps)
	}
	if steps := InstallSteps("linux"); !strings.Contains(strings.Join(steps, " "), "apt-get") {
		t.Errorf("linux steps = %+v", steps)
	}
}

func TestDefaultPackagesMatchCI(t *testing.T) {
	want := []string{"./internal/...", "./tests/...", "./cmd/..."}
	got := DefaultPackages()
	if len(got) != len(want) {
		t.Fatalf("packages = %v", got)
	}
	for index, pkg := range want {
		if got[index] != pkg {
			t.Errorf("packages[%d] = %q, want %q", index, got[index], pkg)
		}
	}
}

func TestCompilersPerPlatform(t *testing.T) {
	if names := Compilers("windows"); names[0] != "gcc" || !strings.Contains(strings.Join(names, " "), "mingw") {
		t.Errorf("windows compilers = %v", names)
	}
	if names := Compilers("darwin"); names[0] != "clang" {
		t.Errorf("darwin compilers = %v", names)
	}
	if names := Compilers("linux"); names[0] != "gcc" {
		t.Errorf("linux compilers = %v", names)
	}
}
