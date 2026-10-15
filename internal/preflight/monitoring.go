package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

const (
	MonitoringTitle = "monitoringcheck"

	monitoringCheckRules     = "rules"
	monitoringCheckDashboard = "dashboard"
	monitoringCheckSurface   = "surface"
	monitoringCheckAlert     = "alertmanager"
	monitoringCheckPromtool  = "promtool"

	monitoringRulesPath     = "deploy/monitoring/prometheus/rules/mgke.yml"
	monitoringRuleTestsPath = "deploy/monitoring/prometheus/rules/mgke.test.yml"
	monitoringDashboardPath = "deploy/monitoring/grafana/dashboards/mgke-timetablebot.json"
	monitoringAlertPath     = "deploy/monitoring/alertmanager/alertmanager.yml"
	monitoringK8sRulesPath  = "deploy/monitoring/kubernetes/prometheusrule.yaml"
	monitoringK8sDashPath   = "deploy/monitoring/kubernetes/grafana-dashboard-configmap.yaml"

	monitoringDashboardInput  = "DS_PROMETHEUS"
	monitoringConfigMapKey    = "mgke-timetablebot.json"
	monitoringSurfacePath     = api.DefaultMetricsPath
	monitoringProbeDirPrefix  = "mgke-monitoringcheck-"
	monitoringToolTimeout     = 60 * time.Second
	monitoringDashboardGrid   = 24
	monitoringDashboardSchema = 39
)

var (
	monitoringSeverities = map[string]bool{
		"critical": true,
		"warning":  true,
		"info":     true,
	}

	monitoringAnnotations = []string{"summary", "description", "runbook_url"}

	monitoringPromtoolSteps = map[string][]string{
		monitoringRulesPath:     {"check", "rules"},
		monitoringRuleTestsPath: {"test", "rules"},
	}

	monitoringPromtoolHints = []string{
		"promtool ships with Prometheus: https://github.com/prometheus/prometheus/releases",
		"run go run ./scripts/monitoringcheck -promtool /path/to/promtool, or -skip-promtool to accept an offline-only run",
	}
)

type MonitoringOptions struct {
	Root         string
	Promtool     string
	SkipPromtool bool
	Now          func() time.Time
	Handler      func(cacheDir string) (http.Handler, error)
	LookTool     func(name string) (string, error)
	RunTool      func(binary string, args ...string) (string, error)
}

type monitoringRule struct {
	Alert         string            `yaml:"alert"`
	Expr          string            `yaml:"expr"`
	For           string            `yaml:"for"`
	KeepFiringFor string            `yaml:"keep_firing_for"`
	Labels        map[string]string `yaml:"labels"`
	Annotations   map[string]string `yaml:"annotations"`
}

type monitoringGroup struct {
	Name  string           `yaml:"name"`
	Rules []monitoringRule `yaml:"rules"`
}

type monitoringRuleSet struct {
	Groups []monitoringGroup `yaml:"groups"`
}

type monitoringPrometheusRule struct {
	Spec struct {
		Groups []monitoringGroup `yaml:"groups"`
	} `yaml:"spec"`
}

type monitoringTarget struct {
	Datasource struct {
		Type string `json:"type"`
		UID  string `json:"uid"`
	} `json:"datasource"`
	Expr  string `json:"expr"`
	RefID string `json:"refId"`
}

type monitoringPanel struct {
	ID         int    `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Datasource struct {
		Type string `json:"type"`
		UID  string `json:"uid"`
	} `json:"datasource"`
	Targets []monitoringTarget `json:"targets"`
	GridPos struct {
		H int `json:"h"`
		W int `json:"w"`
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"gridPos"`
}

type monitoringDashboardDoc struct {
	SchemaVersion int `json:"schemaVersion"`
	Inputs        []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		PluginID string `json:"pluginId"`
	} `json:"__inputs"`
	Panels     []monitoringPanel `json:"panels"`
	Templating struct {
		List []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"list"`
	} `json:"templating"`
}

type monitoringAlertRoute struct {
	Receiver       string                 `yaml:"receiver"`
	GroupBy        []string               `yaml:"group_by"`
	GroupWait      string                 `yaml:"group_wait"`
	GroupInterval  string                 `yaml:"group_interval"`
	RepeatInterval string                 `yaml:"repeat_interval"`
	Matchers       []string               `yaml:"matchers"`
	Routes         []monitoringAlertRoute `yaml:"routes"`
}

type monitoringAlertConfig struct {
	Global struct {
		ResolveTimeout string `yaml:"resolve_timeout"`
	} `yaml:"global"`
	Route     monitoringAlertRoute `yaml:"route"`
	Receivers []struct {
		Name string `yaml:"name"`
	} `yaml:"receivers"`
}

func RunMonitoring(opts MonitoringOptions) Report {
	opts = opts.withDefaults()

	started := time.Now()
	if opts.Now != nil {
		started = opts.Now()
	}

	report := &Report{
		Title:        MonitoringTitle,
		SubjectLabel: "assets",
		Subject:      metricsDeployDir,
		Started:      started,
	}

	report.add(checkMonitoringRules(opts.Root))
	report.add(checkMonitoringDashboard(opts.Root))
	report.add(checkMonitoringAlertmanager(opts.Root))
	report.add(checkMonitoringSurface(opts.Root, opts))

	promtool, incomplete := checkMonitoringPromtool(opts)
	report.add(promtool)
	report.Incomplete = incomplete

	report.finish(started)
	return *report
}

func (o MonitoringOptions) withDefaults() MonitoringOptions {
	if strings.TrimSpace(o.Root) == "" {
		o.Root = findRepoRoot()
	}
	if o.Handler == nil {
		o.Handler = fullyWiredMetricsHandler
	}
	if o.LookTool == nil {
		o.LookTool = exec.LookPath
	}
	if o.RunTool == nil {
		o.RunTool = runMonitoringTool
	}
	return o
}

func checkMonitoringRules(root string) Check {
	data, err := readMonitoringFile(root, monitoringRulesPath)
	if err != nil {
		return Check{Name: monitoringCheckRules, Level: LevelFail, Detail: err.Error(), Hints: []string{"deploy/monitoring is the monitored contract of the metrics route and is committed with the bot"}}
	}

	var rules monitoringRuleSet
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return Check{Name: monitoringCheckRules, Level: LevelFail, Detail: monitoringRulesPath + " is not valid YAML", Hints: []string{"error: " + err.Error()}}
	}
	if len(rules.Groups) == 0 {
		return Check{Name: monitoringCheckRules, Level: LevelFail, Detail: monitoringRulesPath + " declares no rule group"}
	}

	var failures []string
	seen := map[string]bool{}
	alerts := 0
	for _, group := range rules.Groups {
		if strings.TrimSpace(group.Name) == "" {
			failures = append(failures, "a rule group has no name")
		}
		for _, rule := range group.Rules {
			alerts++
			if strings.TrimSpace(rule.Alert) == "" {
				failures = append(failures, "group "+group.Name+" has a rule without an alert name")
				continue
			}
			if seen[rule.Alert] {
				failures = append(failures, "the alert "+rule.Alert+" is declared twice")
			}
			seen[rule.Alert] = true

			if strings.TrimSpace(rule.Expr) == "" {
				failures = append(failures, rule.Alert+" has no expression")
			}
			if rule.For != "" {
				if _, err := time.ParseDuration(rule.For); err != nil {
					failures = append(failures, rule.Alert+" has an unusable for duration "+rule.For)
				}
			}
			if rule.KeepFiringFor != "" {
				if _, err := time.ParseDuration(rule.KeepFiringFor); err != nil {
					failures = append(failures, rule.Alert+" has an unusable keep_firing_for "+rule.KeepFiringFor)
				}
			}
			if !monitoringSeverities[rule.Labels["severity"]] {
				failures = append(failures, fmt.Sprintf("%s has the severity %q, want critical, warning or info", rule.Alert, rule.Labels["severity"]))
			}
			for _, annotation := range monitoringAnnotations {
				if strings.TrimSpace(rule.Annotations[annotation]) == "" {
					failures = append(failures, rule.Alert+" has no "+annotation+" annotation")
				}
			}
		}
	}

	failures = append(failures, monitoringRuleCopyFailures(root, rules)...)

	detail := fmt.Sprintf("%d alerts in %d groups", alerts, len(rules.Groups))
	if len(failures) > 0 {
		return Check{Name: monitoringCheckRules, Level: LevelFail, Detail: detail, Hints: monitoringErrors(failures)}
	}
	return Check{Name: monitoringCheckRules, Level: LevelOK, Detail: detail + ", " + monitoringK8sRulesPath + " matches"}
}

func monitoringRuleCopyFailures(root string, rules monitoringRuleSet) []string {
	data, err := readMonitoringFile(root, monitoringK8sRulesPath)
	if err != nil {
		return []string{err.Error()}
	}

	var custom monitoringPrometheusRule
	if err := yaml.Unmarshal(data, &custom); err != nil {
		return []string{monitoringK8sRulesPath + " is not valid YAML: " + err.Error()}
	}
	if !reflect.DeepEqual(rules.Groups, custom.Spec.Groups) {
		return []string{monitoringK8sRulesPath + " drifted from " + monitoringRulesPath + ": indent the groups block of the rules file under spec.groups again"}
	}
	return nil
}

func checkMonitoringDashboard(root string) Check {
	data, err := readMonitoringFile(root, monitoringDashboardPath)
	if err != nil {
		return Check{Name: monitoringCheckDashboard, Level: LevelFail, Detail: err.Error()}
	}

	var dashboard monitoringDashboardDoc
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return Check{Name: monitoringCheckDashboard, Level: LevelFail, Detail: monitoringDashboardPath + " is not valid JSON", Hints: []string{"error: " + err.Error()}}
	}

	var failures []string
	if dashboard.SchemaVersion != monitoringDashboardSchema {
		failures = append(failures, fmt.Sprintf("schemaVersion is %d, want %d", dashboard.SchemaVersion, monitoringDashboardSchema))
	}
	if len(dashboard.Inputs) != 1 || dashboard.Inputs[0].Name != monitoringDashboardInput {
		failures = append(failures, "the dashboard must declare exactly the "+monitoringDashboardInput+" input")
	} else if dashboard.Inputs[0].Type != "datasource" || dashboard.Inputs[0].PluginID != "prometheus" {
		failures = append(failures, "the "+monitoringDashboardInput+" input must be a prometheus datasource")
	}

	bound := false
	for _, variable := range dashboard.Templating.List {
		if variable.Name != monitoringDashboardInput {
			continue
		}
		bound = variable.Type == "datasource"
		if !bound {
			failures = append(failures, "the "+monitoringDashboardInput+" variable must be a datasource variable")
		}
	}
	if !bound {
		failures = append(failures, "the dashboard needs a "+monitoringDashboardInput+" datasource variable so a provisioned copy renders")
	}

	ids := map[int]bool{}
	panels := 0
	for _, panel := range dashboard.Panels {
		if panel.Type == "row" {
			if strings.TrimSpace(panel.Title) == "" {
				failures = append(failures, fmt.Sprintf("row %d has no title", panel.ID))
			}
			continue
		}

		panels++
		if ids[panel.ID] {
			failures = append(failures, fmt.Sprintf("the panel id %d is used twice", panel.ID))
		}
		ids[panel.ID] = true

		if strings.TrimSpace(panel.Title) == "" {
			failures = append(failures, fmt.Sprintf("panel %d has no title", panel.ID))
		}
		if panel.Datasource.UID != "${"+monitoringDashboardInput+"}" || panel.Datasource.Type != "prometheus" {
			failures = append(failures, fmt.Sprintf("panel %d (%s) must use the %s datasource", panel.ID, panel.Title, monitoringDashboardInput))
		}
		if panel.GridPos.W < 1 || panel.GridPos.W > monitoringDashboardGrid || panel.GridPos.X < 0 || panel.GridPos.X+panel.GridPos.W > monitoringDashboardGrid {
			failures = append(failures, fmt.Sprintf("panel %d (%s) does not fit the %d column grid", panel.ID, panel.Title, monitoringDashboardGrid))
		}
		if len(panel.Targets) == 0 {
			failures = append(failures, fmt.Sprintf("panel %d (%s) has no target", panel.ID, panel.Title))
		}
		for _, target := range panel.Targets {
			if strings.TrimSpace(target.Expr) == "" {
				failures = append(failures, fmt.Sprintf("panel %d (%s) has an empty expression", panel.ID, panel.Title))
			}
			if target.RefID == "" {
				failures = append(failures, fmt.Sprintf("panel %d (%s) has a target without a refId", panel.ID, panel.Title))
			}
		}
	}
	if panels == 0 {
		failures = append(failures, "the dashboard carries no panel")
	}

	failures = append(failures, monitoringDashboardCopyFailures(data, root)...)

	detail := fmt.Sprintf("%d panels, schemaVersion %d", panels, dashboard.SchemaVersion)
	if len(failures) > 0 {
		return Check{Name: monitoringCheckDashboard, Level: LevelFail, Detail: detail, Hints: monitoringErrors(failures)}
	}
	return Check{Name: monitoringCheckDashboard, Level: LevelOK, Detail: detail + ", " + monitoringK8sDashPath + " matches"}
}

func monitoringDashboardCopyFailures(data []byte, root string) []string {
	embedded, err := readMonitoringFile(root, monitoringK8sDashPath)
	if err != nil {
		return []string{err.Error()}
	}

	var configMap struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(embedded, &configMap); err != nil {
		return []string{monitoringK8sDashPath + " is not valid YAML: " + err.Error()}
	}

	copied, ok := configMap.Data[monitoringConfigMapKey]
	if !ok {
		return []string{monitoringK8sDashPath + " carries no " + monitoringConfigMapKey + " key"}
	}

	var fromFile, fromConfigMap any
	if err := json.Unmarshal(data, &fromFile); err != nil {
		return []string{monitoringDashboardPath + " is not valid JSON"}
	}
	if err := json.Unmarshal([]byte(copied), &fromConfigMap); err != nil {
		return []string{monitoringConfigMapKey + " in " + monitoringK8sDashPath + " is not valid JSON"}
	}
	if !reflect.DeepEqual(fromFile, fromConfigMap) {
		return []string{monitoringK8sDashPath + " drifted from " + monitoringDashboardPath + ": copy the dashboard under the " + monitoringConfigMapKey + " key again"}
	}
	return nil
}

func checkMonitoringSurface(root string, opts MonitoringOptions) Check {
	declared := api.MetricFamilies()
	referenced := referencedMetricFamilies(root)
	if len(referenced) == 0 {
		return Check{Name: monitoringCheckSurface, Level: LevelFail, Detail: "no metric name is referenced under " + metricsDeployDir, Hints: []string{"error: the rules, the dashboard or the scrape config went missing"}}
	}

	declaredSet := map[string]bool{}
	for _, name := range declared {
		declaredSet[name] = true
	}

	var failures []string
	for _, name := range referenced {
		if !declaredSet[name] {
			failures = append(failures, name+" is referenced by the monitoring assets but api.MetricFamilies() does not declare it")
		}
	}

	rendered, err := renderedMetricFamilies(opts)
	if err != nil {
		return Check{Name: monitoringCheckSurface, Level: LevelFail, Detail: "the metrics route could not be rendered", Hints: []string{"error: " + err.Error()}}
	}

	for _, name := range declared {
		if !rendered[name] {
			failures = append(failures, name+" is declared but was never rendered: fix the collector in internal/api/metrics.go or drop it from MetricFamilies()")
		}
	}
	for _, name := range sortedKeys(rendered) {
		if !declaredSet[name] {
			failures = append(failures, name+" is rendered but api.MetricFamilies() does not declare it")
		}
	}

	detail := fmt.Sprintf("%d families declared, %d rendered over %s, %d referenced by %s", len(declared), len(rendered), monitoringSurfacePath, len(referenced), metricsDeployDir)
	if len(failures) > 0 {
		return Check{Name: monitoringCheckSurface, Level: LevelFail, Detail: detail, Hints: monitoringErrors(failures)}
	}
	return Check{Name: monitoringCheckSurface, Level: LevelOK, Detail: detail}
}

func renderedMetricFamilies(opts MonitoringOptions) (map[string]bool, error) {
	dir, err := os.MkdirTemp("", monitoringProbeDirPrefix)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	handler, err := opts.Handler(filepath.Join(dir, "rasp"))
	if err != nil {
		return nil, err
	}

	probe := httptest.NewRecorder()
	handler.ServeHTTP(probe, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, monitoringSurfacePath, nil))
	if recorder.Code != http.StatusOK {
		return nil, fmt.Errorf("scraping %s answered %d, want 200", monitoringSurfacePath, recorder.Code)
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(recorder.Body.String()))
	if err != nil {
		return nil, fmt.Errorf("scraping %s: %w", monitoringSurfacePath, err)
	}

	rendered := make(map[string]bool, len(families))
	for name := range families {
		rendered[name] = true
	}
	return rendered, nil
}

func fullyWiredMetricsHandler(cacheDir string) (http.Handler, error) {
	raspCache, err := cache.New(cacheDir)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(cacheDir)
	repo, err := archive.New(filepath.Join(dir, "archive.db"))
	if err != nil {
		return nil, err
	}
	stats, statsErr := repo.Stats()
	bounds, boundsErr := repo.DayIndexBounds()
	if err := repo.Close(); err != nil {
		return nil, err
	}

	counters := health.NewTelegramCounters()
	counters.CountCommand("/day")
	counters.CountCallback("day")
	counters.CountNotification("reminder")
	counters.CountSendError("send_message")

	server := api.NewServer(raspCache, 0, health.NewDefaultTracker(), build.New(MonitoringTitle, "", ""), nil, nil)
	server.SetMetricsConfig(api.MetricsConfig{Path: monitoringSurfacePath})
	server.SetMetricsSources(api.MetricsSources{
		Archive: func() (archive.Stats, error) { return stats, statsErr },
		Bounds:  func() (archive.Bounds, error) { return bounds, boundsErr },
		Webhook: func(context.Context) api.WebhookMetrics {
			return api.WebhookMetrics{
				Enabled:    true,
				Mode:       "webhook",
				Registered: true,
				Secret:     true,
				Reachable:  true,
				Pending:    2,
			}
		},
		Scheduler: func() int { return 2 },
		Reminders: func() int { return 1 },
		Telegram:  counters.TelegramStats,
	})
	return server.Handler(), nil
}

func checkMonitoringAlertmanager(root string) Check {
	data, err := readMonitoringFile(root, monitoringAlertPath)
	if err != nil {
		return Check{Name: monitoringCheckAlert, Level: LevelFail, Detail: err.Error()}
	}

	var config monitoringAlertConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Check{Name: monitoringCheckAlert, Level: LevelFail, Detail: monitoringAlertPath + " is not valid YAML", Hints: []string{"error: " + err.Error()}}
	}

	var failures []string
	receivers := map[string]bool{}
	for _, receiver := range config.Receivers {
		name := strings.TrimSpace(receiver.Name)
		if name == "" {
			failures = append(failures, "a receiver has no name")
			continue
		}
		if receivers[name] {
			failures = append(failures, "the receiver "+name+" is declared twice")
		}
		receivers[name] = true
	}
	if len(config.Receivers) == 0 {
		failures = append(failures, "the profile declares no receiver")
	}

	failures = append(failures, monitoringRouteFailures(config.Route, receivers)...)

	if config.Global.ResolveTimeout != "" {
		if _, err := time.ParseDuration(config.Global.ResolveTimeout); err != nil {
			failures = append(failures, "global.resolve_timeout is not a duration: "+config.Global.ResolveTimeout)
		}
	}

	detail := fmt.Sprintf("%d receivers, %d groups", len(config.Receivers), len(config.Route.GroupBy))
	if len(failures) > 0 {
		return Check{Name: monitoringCheckAlert, Level: LevelFail, Detail: detail, Hints: monitoringErrors(failures)}
	}

	hints := []string{"amtool check-config " + monitoringAlertPath + " validates the same file with the Alertmanager itself"}
	return Check{Name: monitoringCheckAlert, Level: LevelOK, Detail: detail, Hints: hints}
}

func monitoringRouteFailures(route monitoringAlertRoute, receivers map[string]bool) []string {
	var failures []string

	if name := strings.TrimSpace(route.Receiver); name == "" {
		failures = append(failures, "a route has no receiver")
	} else if !receivers[name] {
		failures = append(failures, "the route receiver "+name+" is not declared in receivers")
	}

	for field, value := range map[string]string{
		"group_wait":      route.GroupWait,
		"group_interval":  route.GroupInterval,
		"repeat_interval": route.RepeatInterval,
	} {
		if value == "" {
			continue
		}
		if _, err := time.ParseDuration(value); err != nil {
			failures = append(failures, "a route has an unusable "+field+" "+value)
		}
	}

	for _, matcher := range route.Matchers {
		if strings.TrimSpace(matcher) == "" {
			failures = append(failures, "a route carries an empty matcher")
		}
	}

	for _, child := range route.Routes {
		failures = append(failures, monitoringRouteFailures(child, receivers)...)
	}
	return failures
}

func checkMonitoringPromtool(opts MonitoringOptions) (Check, bool) {
	rules, tests := monitoringCounts(opts.Root)

	if opts.SkipPromtool {
		return Check{
			Name:   monitoringCheckPromtool,
			Level:  LevelSkip,
			Detail: fmt.Sprintf("skipped by flag: %d rules and %d unit-test cases were NOT checked", rules, tests),
			Hints:  []string{"drop -skip-promtool to run promtool check rules and promtool test rules"},
		}, false
	}

	binary := strings.TrimSpace(opts.Promtool)
	switch {
	case binary == "":
		found, err := opts.LookTool("promtool")
		if err != nil {
			return Check{
				Name:   monitoringCheckPromtool,
				Level:  LevelSkip,
				Detail: fmt.Sprintf("promtool is not installed: %d rules and %d unit-test cases were NOT checked", rules, tests),
				Hints:  monitoringPromtoolHints,
			}, true
		}
		binary = found
	case !strings.ContainsAny(binary, `/\`):
		found, err := opts.LookTool(binary)
		if err != nil {
			return Check{
				Name:   monitoringCheckPromtool,
				Level:  LevelFail,
				Detail: fmt.Sprintf("the promtool binary %q was not found: %d rules and %d unit-test cases were NOT checked", binary, rules, tests),
				Hints:  monitoringErrors([]string{"pass -promtool /path/to/promtool or drop the flag to look it up on PATH"}),
			}, false
		}
		binary = found
	}

	var failures []string
	for _, path := range []string{monitoringRulesPath, monitoringRuleTestsPath} {
		steps := monitoringPromtoolSteps[path]
		output, err := opts.RunTool(binary, append(append([]string{}, steps...), filepath.Join(opts.Root, filepath.FromSlash(path)))...)
		if err != nil {
			failures = append(failures, fmt.Sprintf("promtool %s %s failed: %v", strings.Join(steps, " "), path, err))
			failures = append(failures, monitoringToolTail(output)...)
		}
	}

	detail := fmt.Sprintf("%d rules and %d unit-test cases checked with %s", rules, tests, binary)
	if len(failures) > 0 {
		return Check{Name: monitoringCheckPromtool, Level: LevelFail, Detail: "promtool reported a problem with " + monitoringRulesPath, Hints: monitoringErrors(failures)}, false
	}
	return Check{Name: monitoringCheckPromtool, Level: LevelOK, Detail: detail}, false
}

func monitoringCounts(root string) (int, int) {
	rules, cases := 0, 0

	if data, err := readMonitoringFile(root, monitoringRulesPath); err == nil {
		var parsed monitoringRuleSet
		if yaml.Unmarshal(data, &parsed) == nil {
			for _, group := range parsed.Groups {
				rules += len(group.Rules)
			}
		}
	}

	if data, err := readMonitoringFile(root, monitoringRuleTestsPath); err == nil {
		var parsed struct {
			Tests []struct {
				Name string `yaml:"name"`
			} `yaml:"tests"`
		}
		if yaml.Unmarshal(data, &parsed) == nil {
			cases = len(parsed.Tests)
		}
	}
	return rules, cases
}

func runMonitoringTool(binary string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), monitoringToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func monitoringToolTail(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}

	var hints []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			hints = append(hints, trimmed)
		}
	}
	return hints
}

func readMonitoringFile(root, name string) ([]byte, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%s: the repository root was not found, run the check from the repository", name)
	}

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return data, nil
}

func monitoringErrors(failures []string) []string {
	hints := make([]string, 0, len(failures))
	for _, failure := range failures {
		hints = append(hints, "error: "+failure)
	}
	return hints
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
