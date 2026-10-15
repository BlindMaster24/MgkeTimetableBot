package preflight

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
)

const monitoringFixtureRule = `groups:
  - name: fixture
    rules:
      - alert: FixtureAlert
        expr: mgke_uptime_seconds > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "fixture"
          description: "fixture"
          runbook_url: "https://example.test/runbook"
`

const monitoringFixturePrometheusRule = `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: fixture
spec:
  groups:
    - name: fixture
      rules:
        - alert: FixtureAlert
          expr: mgke_uptime_seconds > 0
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "fixture"
            description: "fixture"
            runbook_url: "https://example.test/runbook"
`

func writeMonitoringAsset(t *testing.T, root, name, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func monitoringRulesFixture(t *testing.T, expr string) string {
	t.Helper()

	root := t.TempDir()
	writeMonitoringAsset(t, root, monitoringRulesPath, strings.Replace(monitoringFixtureRule, "mgke_uptime_seconds > 0", expr, 1))
	writeMonitoringAsset(t, root, monitoringK8sRulesPath, strings.Replace(monitoringFixturePrometheusRule, "mgke_uptime_seconds > 0", expr, 1))
	return root
}

func monitoringWorkingTool(t *testing.T, calls *[]string) func(string, ...string) (string, error) {
	t.Helper()

	return func(binary string, args ...string) (string, error) {
		if calls != nil {
			*calls = append(*calls, binary+" "+strings.Join(args, " "))
		}
		return "SUCCESS: 1 rules found\n", nil
	}
}

func monitoringFoundTool(path string) func(string) (string, error) {
	return func(name string) (string, error) { return path, nil }
}

func requireMonitoringLevel(t *testing.T, check Check, level string) {
	t.Helper()

	if check.Level != level {
		t.Fatalf("%s level = %s, want %s (%s %v)", check.Name, check.Level, level, check.Detail, check.Hints)
	}
}

func requireMonitoringMentions(t *testing.T, check Check, text string) {
	t.Helper()

	for _, hint := range check.Hints {
		if strings.Contains(hint, text) {
			return
		}
	}
	if strings.Contains(check.Detail, text) {
		return
	}
	t.Errorf("%s does not mention %q: %s %v", check.Name, text, check.Detail, check.Hints)
}

func TestMonitoringCheckPassesOnTheCommittedAssets(t *testing.T) {
	root := findRepoRoot()
	if root == "" {
		t.Skip("the repository root is not reachable from the test working directory")
	}

	var calls []string
	report := RunMonitoring(MonitoringOptions{
		Root:     root,
		LookTool: monitoringFoundTool("/usr/local/bin/promtool"),
		RunTool:  monitoringWorkingTool(t, &calls),
	})

	if report.Failed() {
		t.Fatalf("the committed monitoring set must pass: %s", report.Text())
	}
	if report.Incomplete {
		t.Errorf("a run with promtool must not be incomplete: %s", report.Text())
	}
	if report.Warnings() != 0 || report.Count(LevelSkip) != 0 {
		t.Errorf("the committed set must be free of warnings and skips: %s", report.Text())
	}
	if report.Count(LevelOK) != 5 {
		t.Errorf("expected the rules, dashboard, alertmanager, surface and promtool checks to pass: %s", report.Text())
	}

	alert := report.Checks[2]
	if alert.Name != monitoringCheckAlert {
		t.Fatalf("expected the alertmanager check in third place, got %v", report.Checks)
	}
	if !strings.Contains(alert.Detail, "receivers") {
		t.Errorf("the alertmanager detail must count the receivers: %q", alert.Detail)
	}

	surface := report.Checks[3]
	if surface.Name != monitoringCheckSurface {
		t.Fatalf("expected the surface check in fourth place, got %v", report.Checks)
	}
	declared := fmt.Sprintf("%d families declared", len(api.MetricFamilies()))
	if !strings.Contains(surface.Detail, declared) || !strings.Contains(surface.Detail, "rendered over ") {
		t.Errorf("the surface detail must count the declared and rendered families: %q", surface.Detail)
	}

	if len(calls) != 2 {
		t.Fatalf("promtool must run twice, got %v", calls)
	}
	for _, want := range []string{"check rules " + filepath.Join(root, filepath.FromSlash(monitoringRulesPath)), "test rules " + filepath.Join(root, filepath.FromSlash(monitoringRuleTestsPath))} {
		if !strings.Contains(calls[0]+" "+calls[1], want) {
			t.Errorf("expected the call %q, got %v", want, calls)
		}
	}
}

func TestMonitoringRulesCheckCatchesBrokenRules(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"invalid yaml": {
			body: "groups: [\n",
			want: "is not valid YAML",
		},
		"no group": {
			body: "groups: []\n",
			want: "declares no rule group",
		},
		"duplicate alert": {
			body: strings.Replace(monitoringFixtureRule, "        expr:", "      - alert: FixtureAlert\n        expr:", 1),
			want: "is declared twice",
		},
		"missing severity": {
			body: strings.Replace(monitoringFixtureRule, "          severity: warning\n", "          severity: noisy\n", 1),
			want: `has the severity "noisy"`,
		},
		"missing annotation": {
			body: strings.Replace(monitoringFixtureRule, "          runbook_url: \"https://example.test/runbook\"\n", "", 1),
			want: "has no runbook_url annotation",
		},
		"unusable for": {
			body: strings.Replace(monitoringFixtureRule, "        for: 5m\n", "        for: soon\n", 1),
			want: "has an unusable for duration soon",
		},
		"missing expression": {
			body: strings.Replace(monitoringFixtureRule, "        expr: mgke_uptime_seconds > 0\n", "", 1),
			want: "has no expression",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
			writeMonitoringAsset(t, root, monitoringRulesPath, tc.body)

			check := checkMonitoringRules(root)
			requireMonitoringLevel(t, check, LevelFail)
			requireMonitoringMentions(t, check, tc.want)
		})
	}
}

func TestMonitoringRulesCheckCatchesDriftedCopies(t *testing.T) {
	root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
	check := checkMonitoringRules(root)
	requireMonitoringLevel(t, check, LevelOK)

	writeMonitoringAsset(t, root, monitoringK8sRulesPath, strings.Replace(monitoringFixturePrometheusRule, "for: 5m", "for: 10m", 1))
	drifted := checkMonitoringRules(root)
	requireMonitoringLevel(t, drifted, LevelFail)
	requireMonitoringMentions(t, drifted, "drifted from")

	missing := t.TempDir()
	writeMonitoringAsset(t, missing, monitoringRulesPath, monitoringFixtureRule)
	absent := checkMonitoringRules(missing)
	requireMonitoringLevel(t, absent, LevelFail)
	requireMonitoringMentions(t, absent, monitoringK8sRulesPath)
}

func TestMonitoringDashboardCheckCatchesBrokenDashboards(t *testing.T) {
	valid := `{"schemaVersion":39,"__inputs":[{"name":"DS_PROMETHEUS","type":"datasource","pluginId":"prometheus"}],"templating":{"list":[{"name":"DS_PROMETHEUS","type":"datasource"}]},"panels":[{"id":1,"type":"timeseries","title":"Uptime","datasource":{"type":"prometheus","uid":"${DS_PROMETHEUS}"},"gridPos":{"h":8,"w":12,"x":0,"y":0},"targets":[{"datasource":{"type":"prometheus","uid":"${DS_PROMETHEUS}"},"expr":"mgke_uptime_seconds","refId":"A"}]}]}`
	configMap := func(body string) string {
		return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\ndata:\n  " + monitoringConfigMapKey + ": |\n" + indentForConfigMap(body)
	}

	cases := map[string]struct {
		body string
		copy string
		want string
	}{
		"invalid json":       {body: "{", copy: configMap("{}"), want: "is not valid JSON"},
		"wrong schema":       {body: strings.Replace(valid, `"schemaVersion":39`, `"schemaVersion":30`, 1), copy: configMap(strings.Replace(valid, `"schemaVersion":39`, `"schemaVersion":30`, 1)), want: "schemaVersion is 30"},
		"no input":           {body: strings.Replace(valid, `"__inputs":[{"name":"DS_PROMETHEUS","type":"datasource","pluginId":"prometheus"}],`, "", 1), copy: configMap(strings.Replace(valid, `"__inputs":[{"name":"DS_PROMETHEUS","type":"datasource","pluginId":"prometheus"}],`, "", 1)), want: "must declare exactly the DS_PROMETHEUS input"},
		"foreign datasource": {body: strings.Replace(valid, `"uid":"${DS_PROMETHEUS}"},"gridPos"`, `"uid":"other"},"gridPos"`, 1), copy: configMap(valid), want: "must use the DS_PROMETHEUS datasource"},
		"no target":          {body: strings.Replace(valid, `"targets":[{"datasource":{"type":"prometheus","uid":"${DS_PROMETHEUS}"},"expr":"mgke_uptime_seconds","refId":"A"}]`, `"targets":[]`, 1), copy: configMap(valid), want: "has no target"},
		"drifted copy":       {body: valid, copy: configMap(strings.Replace(valid, "Uptime", "Uptime 2", 1)), want: "drifted from"},
		"missing copy":       {body: valid, copy: "", want: monitoringK8sDashPath},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeMonitoringAsset(t, root, monitoringDashboardPath, tc.body)
			if tc.copy != "" {
				writeMonitoringAsset(t, root, monitoringK8sDashPath, tc.copy)
			}

			check := checkMonitoringDashboard(root)
			requireMonitoringLevel(t, check, LevelFail)
			requireMonitoringMentions(t, check, tc.want)
		})
	}
}

func indentForConfigMap(body string) string {
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func TestMonitoringSurfaceCheckCatchesUnknownAndUnrenderedFamilies(t *testing.T) {
	opts := MonitoringOptions{
		Handler:  fullyWiredMetricsHandler,
		LookTool: monitoringFoundTool("/usr/local/bin/promtool"),
		RunTool:  monitoringWorkingTool(t, nil),
	}

	known := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
	requireMonitoringLevel(t, checkMonitoringSurface(known, opts), LevelOK)

	unknown := monitoringRulesFixture(t, "mgke_ghost_family_total > 0")
	offender := checkMonitoringSurface(unknown, opts)
	requireMonitoringLevel(t, offender, LevelFail)
	requireMonitoringMentions(t, offender, "mgke_ghost_family_total is referenced by the monitoring assets but api.MetricFamilies() does not declare it")

	empty := t.TempDir()
	requireMonitoringLevel(t, checkMonitoringSurface(empty, opts), LevelFail)

	unrendered := checkMonitoringSurface(known, MonitoringOptions{
		Handler: func(cacheDir string) (http.Handler, error) {
			inner, err := fullyWiredMetricsHandler(cacheDir)
			if err != nil {
				return nil, err
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				recorder := httptest.NewRecorder()
				inner.ServeHTTP(recorder, r)
				w.WriteHeader(recorder.Code)
				io.WriteString(w, dropLines("mgke_calendar_lag_seconds")(recorder.Body.String()))
			}), nil
		},
	})
	requireMonitoringLevel(t, unrendered, LevelFail)
	requireMonitoringMentions(t, unrendered, "mgke_calendar_lag_seconds is declared but was never rendered")

	undeclared := checkMonitoringSurface(known, MonitoringOptions{
		Handler: func(cacheDir string) (http.Handler, error) {
			inner, err := fullyWiredMetricsHandler(cacheDir)
			if err != nil {
				return nil, err
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				recorder := httptest.NewRecorder()
				inner.ServeHTTP(recorder, r)
				w.WriteHeader(recorder.Code)
				body := recorder.Body.String()
				if strings.Contains(body, "# HELP mgke_uptime_seconds ") {
					body = strings.Replace(body, "# HELP mgke_uptime_seconds ", "# HELP mgke_ghost_total ghost\n# TYPE mgke_ghost_total gauge\nmgke_ghost_total 1\n# HELP mgke_uptime_seconds ", 1)
				}
				io.WriteString(w, body)
			}), nil
		},
	})
	requireMonitoringLevel(t, undeclared, LevelFail)
	requireMonitoringMentions(t, undeclared, "mgke_ghost_total is rendered but api.MetricFamilies() does not declare it")
}

func TestMonitoringSurfaceCheckFailsWhenTheRouteCannotBeRendered(t *testing.T) {
	root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
	check := checkMonitoringSurface(root, MonitoringOptions{
		Handler: func(string) (http.Handler, error) {
			return nil, fmt.Errorf("no cache here")
		},
	})

	requireMonitoringLevel(t, check, LevelFail)
	requireMonitoringMentions(t, check, "the metrics route could not be rendered")
	requireMonitoringMentions(t, check, "no cache here")
}

func TestMonitoringAlertmanagerCheckCatchesBrokenProfiles(t *testing.T) {
	valid := `global:
  resolve_timeout: 5m
route:
  receiver: default
  group_by:
    - alertname
  group_wait: 30s
  repeat_interval: 4h
  routes:
    - matchers:
        - severity = critical
      receiver: critical
receivers:
  - name: default
  - name: critical
`

	cases := map[string]struct {
		body string
		want string
	}{
		"invalid yaml":                  {body: "route: [\n", want: "is not valid YAML"},
		"no receiver":                   {body: strings.Replace(valid, "receivers:\n  - name: default\n  - name: critical\n", "receivers: []\n", 1), want: "declares no receiver"},
		"unknown route receiver":        {body: strings.Replace(valid, "  receiver: default\n", "  receiver: nobody\n", 1), want: "the route receiver nobody is not declared in receivers"},
		"duplicate receiver":            {body: strings.Replace(valid, "  - name: critical\n", "  - name: default\n", 1), want: "is declared twice"},
		"unusable interval":             {body: strings.Replace(valid, "  repeat_interval: 4h\n", "  repeat_interval: often\n", 1), want: "unusable repeat_interval often"},
		"unusable global":               {body: strings.Replace(valid, "  resolve_timeout: 5m\n", "  resolve_timeout: never\n", 1), want: "global.resolve_timeout is not a duration"},
		"empty matcher":                 {body: strings.Replace(valid, "        - severity = critical\n", "        - \"\"\n", 1), want: "empty matcher"},
		"nested route without receiver": {body: strings.Replace(valid, "      receiver: critical\n", "      receiver: \"\"\n", 1), want: "a route has no receiver"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeMonitoringAsset(t, root, monitoringAlertPath, tc.body)

			check := checkMonitoringAlertmanager(root)
			requireMonitoringLevel(t, check, LevelFail)
			requireMonitoringMentions(t, check, tc.want)
		})
	}

	root := t.TempDir()
	writeMonitoringAsset(t, root, monitoringAlertPath, valid)
	check := checkMonitoringAlertmanager(root)
	requireMonitoringLevel(t, check, LevelOK)
	requireMonitoringMentions(t, check, "amtool check-config")
}

func TestMonitoringPromtoolCheckReportsAMissingTool(t *testing.T) {
	root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")

	missing := func(name string) (string, error) { return "", fmt.Errorf("%s not found", name) }
	check, incomplete := checkMonitoringPromtool(MonitoringOptions{Root: root, LookTool: missing, RunTool: monitoringWorkingTool(t, nil)})

	requireMonitoringLevel(t, check, LevelSkip)
	if !incomplete {
		t.Error("a missing promtool must mark the run incomplete")
	}
	for _, want := range []string{"promtool is not installed", "were NOT checked"} {
		requireMonitoringMentions(t, check, want)
	}
	requireMonitoringMentions(t, check, "-skip-promtool")

	skipped, incomplete := checkMonitoringPromtool(MonitoringOptions{Root: root, SkipPromtool: true, RunTool: monitoringWorkingTool(t, nil)})
	requireMonitoringLevel(t, skipped, LevelSkip)
	if incomplete {
		t.Error("-skip-promtool is an explicit operator decision, not an incomplete run")
	}
	requireMonitoringMentions(t, skipped, "skipped by flag")

	named, _ := checkMonitoringPromtool(MonitoringOptions{Root: root, Promtool: "promtool", LookTool: missing, RunTool: monitoringWorkingTool(t, nil)})
	requireMonitoringLevel(t, named, LevelFail)
	requireMonitoringMentions(t, named, `the promtool binary "promtool" was not found`)
}

func TestMonitoringPromtoolCheckReportsRejectedRules(t *testing.T) {
	root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
	tool := func(binary string, args ...string) (string, error) {
		return "  FAILED: parse error at line 12\n", fmt.Errorf("exit status 1")
	}

	check, incomplete := checkMonitoringPromtool(MonitoringOptions{Root: root, Promtool: filepath.Join(root, "promtool"), RunTool: tool})
	requireMonitoringLevel(t, check, LevelFail)
	if incomplete {
		t.Error("a failing promtool run is a failure, not an incomplete run")
	}
	for _, want := range []string{"promtool check rules", "failed", "FAILED: parse error at line 12"} {
		requireMonitoringMentions(t, check, want)
	}
}

func TestMonitoringReportMarksAnIncompleteRun(t *testing.T) {
	root := monitoringRulesFixture(t, "mgke_uptime_seconds > 0")
	report := RunMonitoring(MonitoringOptions{
		Root:     root,
		LookTool: func(name string) (string, error) { return "", fmt.Errorf("%s not found", name) },
		RunTool:  monitoringWorkingTool(t, nil),
	})

	if !report.Incomplete {
		t.Fatalf("a run without promtool must be incomplete: %s", report.Text())
	}
	text := report.Text()
	for _, want := range []string{"monitoringcheck ", "assets=deploy/monitoring", "[skip] promtool", "result: NOT VERIFIED"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report text is missing %q:\n%s", want, text)
		}
	}

	if _, err := report.JSON(); err != nil {
		t.Errorf("the report must serialize as JSON: %v", err)
	}
}

func TestMonitoringReportKeepsThePreflightShape(t *testing.T) {
	report := Report{ConfigPath: "configs/config.yaml", Started: time.Now()}
	report.add(Check{Name: "config", Level: LevelOK, Detail: "fine"})

	text := report.Text()
	if !strings.Contains(text, "preflight ") || !strings.Contains(text, "config=configs/config.yaml") {
		t.Errorf("a report without a title must keep the preflight header: %q", text)
	}
	if strings.Contains(text, "NOT VERIFIED") {
		t.Errorf("a complete report must not claim to be unverified: %q", text)
	}
}
