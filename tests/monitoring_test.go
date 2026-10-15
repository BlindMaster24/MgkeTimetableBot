package tests

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"gopkg.in/yaml.v3"
)

const (
	monitoringDir          = "deploy/monitoring"
	monitoringReadme       = "deploy/monitoring/README.md"
	monitoringRulesFile    = "deploy/monitoring/prometheus/rules/mgke.yml"
	monitoringRuleTests    = "deploy/monitoring/prometheus/rules/mgke.test.yml"
	monitoringPromFile     = "deploy/monitoring/prometheus/prometheus.yml"
	monitoringAlertmanager = "deploy/monitoring/alertmanager/alertmanager.yml"
	monitoringDashboard    = "deploy/monitoring/grafana/dashboards/mgke-timetablebot.json"
	monitoringDatasource   = "deploy/monitoring/grafana/provisioning/datasources/prometheus.yml"
	monitoringDashboardCfg = "deploy/monitoring/grafana/provisioning/dashboards/mgke.yml"
	monitoringServiceMon   = "deploy/monitoring/kubernetes/servicemonitor.yaml"
	monitoringK8sRules     = "deploy/monitoring/kubernetes/prometheusrule.yaml"
	monitoringK8sDashboard = "deploy/monitoring/kubernetes/grafana-dashboard-configmap.yaml"
	monitoringAlloy        = "deploy/monitoring/alloy/config.alloy"
	monitoringComposeFile  = "docker-compose.monitoring.yml"

	monitoringDashboardInput = "DS_PROMETHEUS"
	monitoringDatasourceUID  = "prometheus"
	monitoringConfigMapKey   = "mgke-timetablebot.json"
)

var (
	monitoringMetricName = regexp.MustCompile(`mgke_[a-z0-9_]+`)

	monitoringFiles = map[string]bool{
		".yml":   true,
		".yaml":  true,
		".json":  true,
		".alloy": true,
	}

	monitoringSeverities = map[string]bool{
		"critical": true,
		"warning":  true,
		"info":     true,
	}

	monitoringAnnotations = []string{"summary", "description", "runbook_url"}

	monitoringRunbookPrefix = "https://github.com/BlindMaster24/MgkeTimetableBot/blob/main/deploy/monitoring/README.md#"
)

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

type monitoringRuleFile struct {
	Groups []monitoringGroup `yaml:"groups"`
}

type monitoringPrometheusRule struct {
	Spec struct {
		Groups []monitoringGroup `yaml:"groups"`
	} `yaml:"spec"`
}

type monitoringExpectedAlert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}

type monitoringRuleTest struct {
	EvalTime  string                    `yaml:"eval_time"`
	AlertName string                    `yaml:"alertname"`
	ExpAlerts []monitoringExpectedAlert `yaml:"exp_alerts"`
}

type monitoringTestCase struct {
	Name        string `yaml:"name"`
	Interval    string `yaml:"interval"`
	InputSeries []struct {
		Series string `yaml:"series"`
		Values string `yaml:"values"`
	} `yaml:"input_series"`
	AlertRuleTests []monitoringRuleTest `yaml:"alert_rule_test"`
}

type monitoringRuleTestFile struct {
	RuleFiles          []string             `yaml:"rule_files"`
	EvaluationInterval string               `yaml:"evaluation_interval"`
	Tests              []monitoringTestCase `yaml:"tests"`
}

type monitoringTarget struct {
	Datasource dashboardDatasource `json:"datasource"`
	Expr       string              `json:"expr"`
	RefID      string              `json:"refId"`
}

type dashboardDatasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type monitoringPanel struct {
	ID         int                 `json:"id"`
	Type       string              `json:"type"`
	Title      string              `json:"title"`
	Datasource dashboardDatasource `json:"datasource"`
	Targets    []monitoringTarget  `json:"targets"`
	GridPos    struct {
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
			Name    string `json:"name"`
			Type    string `json:"type"`
			Query   string `json:"query"`
			Current struct {
				Text  string `json:"text"`
				Value string `json:"value"`
			} `json:"current"`
		} `json:"list"`
	} `json:"templating"`
}

type monitoringComposeService struct {
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command"`
	Profiles    []string          `yaml:"profiles"`
	Environment map[string]string `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	DependsOn   map[string]struct {
		Condition string `yaml:"condition"`
	} `yaml:"depends_on"`
}

type monitoringCompose struct {
	Services map[string]monitoringComposeService `yaml:"services"`
}

func parseMonitoring[T any](t *testing.T, name string) T {
	t.Helper()

	var parsed T
	if err := yaml.Unmarshal([]byte(readFile(t, name)), &parsed); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}

func declaredMetricFamilies() map[string]bool {
	declared := map[string]bool{}
	for _, name := range api.MetricFamilies() {
		declared[name] = true
	}
	return declared
}

func monitoringAssetFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	if err := filepath.WalkDir(filepath.Join("..", monitoringDir), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !monitoringFiles[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", monitoringDir, err)
	}

	compose, err := filepath.Abs(filepath.Join("..", monitoringComposeFile))
	if err != nil {
		t.Fatalf("resolve %s: %v", monitoringComposeFile, err)
	}
	return append(files, compose)
}

func TestMonitoringAssetsOnlyUseDeclaredMetrics(t *testing.T) {
	declared := declaredMetricFamilies()
	referenced := map[string]string{}

	for _, path := range monitoringAssetFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, name := range monitoringMetricName.FindAllString(string(data), -1) {
			referenced[name] = filepath.Base(path)
		}
	}

	if len(referenced) < 30 {
		t.Fatalf("expected the monitoring assets to reference the metrics surface, found %d names", len(referenced))
	}

	for name, file := range referenced {
		if !declared[name] {
			t.Errorf("%s references %s, which api.MetricFamilies() does not declare; rename the metric there first or fix the asset", file, name)
		}
	}
}

func TestMonitoringRulesAreWellFormed(t *testing.T) {
	file := parseMonitoring[monitoringRuleFile](t, monitoringRulesFile)
	readme := readFile(t, monitoringReadme)

	if len(file.Groups) < 4 {
		t.Fatalf("expected the rules in thematic groups, got %d", len(file.Groups))
	}

	seen := map[string]bool{}
	total := 0
	for _, group := range file.Groups {
		if strings.TrimSpace(group.Name) == "" {
			t.Error("every rule group needs a name")
		}
		for _, rule := range group.Rules {
			total++
			if rule.Alert == "" {
				t.Errorf("group %s has a rule without a name", group.Name)
				continue
			}
			if seen[rule.Alert] {
				t.Errorf("the alert %s is declared twice", rule.Alert)
			}
			seen[rule.Alert] = true

			if strings.TrimSpace(rule.Expr) == "" {
				t.Errorf("%s has no expression", rule.Alert)
			}
			if _, err := time.ParseDuration(rule.For); err != nil {
				t.Errorf("%s has an unusable for duration %q: %v", rule.Alert, rule.For, err)
			}
			if rule.KeepFiringFor != "" {
				if _, err := time.ParseDuration(rule.KeepFiringFor); err != nil {
					t.Errorf("%s has an unusable keep_firing_for %q: %v", rule.Alert, rule.KeepFiringFor, err)
				}
			}
			if !monitoringSeverities[rule.Labels["severity"]] {
				t.Errorf("%s has the severity %q, want one of critical, warning, info", rule.Alert, rule.Labels["severity"])
			}
			for _, annotation := range monitoringAnnotations {
				if strings.TrimSpace(rule.Annotations[annotation]) == "" {
					t.Errorf("%s has no %s annotation", rule.Alert, annotation)
				}
			}

			anchor := monitoringRunbookPrefix + strings.ToLower(rule.Alert)
			if rule.Annotations["runbook_url"] != anchor {
				t.Errorf("%s must point its runbook_url at %s, got %q", rule.Alert, anchor, rule.Annotations["runbook_url"])
			}
			if !strings.Contains(readme, "### "+rule.Alert) {
				t.Errorf("%s documents its runbook at %s, but deploy/monitoring/README.md has no \"### %s\" section", rule.Alert, anchor, rule.Alert)
			}
		}
	}

	if total < 12 {
		t.Errorf("expected at least twelve alerts, got %d", total)
	}
}

func TestMonitoringParserAlertsUseTheDurableCacheGauge(t *testing.T) {
	file := parseMonitoring[monitoringRuleFile](t, monitoringRulesFile)

	checked := 0
	for _, group := range file.Groups {
		for _, rule := range group.Rules {
			if !strings.Contains(rule.Expr, "mgke_cache_groups_last_update_seconds") {
				if strings.Contains(rule.Expr, "mgke_parser_lag_seconds") {
					t.Errorf("%s is built on the lag gauge alone; a parser that never ran reports a lag of zero, so every parser staleness alert must also read mgke_cache_groups_last_update_seconds", rule.Alert)
				}
				continue
			}
			checked++
		}
	}

	if checked < 3 {
		t.Fatalf("expected at least three alerts on the durable cache freshness gauge, got %d", checked)
	}

	for _, alert := range []string{"MgkeParserNeverFilled", "MgkeParserStale", "MgkeParserFrozen"} {
		if !strings.Contains(readFile(t, monitoringRulesFile), "alert: "+alert) {
			t.Errorf("the parser staleness family must keep the %s alert", alert)
		}
	}

	stale := ""
	for _, group := range file.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "MgkeParserStale" {
				stale = rule.Expr
			}
		}
	}
	for _, token := range []string{"mgke_cache_groups_last_update_seconds > 0", "mgke_parser_lag_seconds > 900"} {
		if !strings.Contains(stale, token) {
			t.Errorf("MgkeParserStale must read the durable freshness gauge and the lag gauge together, %q is missing from %q", token, stale)
		}
	}

	never := ""
	for _, group := range file.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "MgkeParserNeverFilled" {
				never = rule.Expr
			}
		}
	}
	if !strings.Contains(never, "mgke_uptime_seconds") || !strings.Contains(never, "mgke_cache_groups_last_update_seconds == 0") {
		t.Errorf("MgkeParserNeverFilled must combine the uptime with the freshness gauge, got %q", never)
	}
}

func TestMonitoringDashboardImportsWithoutEditing(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", monitoringDashboard))
	if err != nil {
		t.Fatalf("read %s: %v", monitoringDashboard, err)
	}

	var dashboard monitoringDashboardDoc
	if err := json.Unmarshal(data, &dashboard); err != nil {
		t.Fatalf("parse %s: %v", monitoringDashboard, err)
	}

	if dashboard.SchemaVersion != 39 {
		t.Errorf("the dashboard must export schemaVersion 39, got %d", dashboard.SchemaVersion)
	}
	if len(dashboard.Inputs) != 1 || dashboard.Inputs[0].Name != monitoringDashboardInput {
		t.Fatalf("expected exactly the %s input, got %+v", monitoringDashboardInput, dashboard.Inputs)
	}
	if dashboard.Inputs[0].Type != "datasource" || dashboard.Inputs[0].PluginID != "prometheus" {
		t.Errorf("the %s input must be a prometheus datasource, got %+v", monitoringDashboardInput, dashboard.Inputs[0])
	}

	bound := false
	for _, variable := range dashboard.Templating.List {
		if variable.Name != monitoringDashboardInput {
			continue
		}
		bound = true
		if variable.Type != "datasource" {
			t.Errorf("the %s variable must be a datasource variable, got %q", monitoringDashboardInput, variable.Type)
		}
		if variable.Current.Value != monitoringDatasourceUID {
			t.Errorf("the %s variable must default to the provisioned uid %q, got %q", monitoringDashboardInput, monitoringDatasourceUID, variable.Current.Value)
		}
	}
	if !bound {
		t.Errorf("the dashboard needs a %s datasource variable, otherwise the provisioned copy renders broken panels", monitoringDashboardInput)
	}

	provisioned := parseMonitoring[struct {
		Datasources []struct {
			UID string `yaml:"uid"`
		} `yaml:"datasources"`
	}](t, monitoringDatasource)
	if len(provisioned.Datasources) == 0 || provisioned.Datasources[0].UID != monitoringDatasourceUID {
		t.Errorf("the provisioned datasource must use the uid %q, got %+v", monitoringDatasourceUID, provisioned.Datasources)
	}

	ids := map[int]bool{}
	panels := 0
	exprs := strings.Builder{}
	for _, panel := range dashboard.Panels {
		if panel.Type == "row" {
			if panel.Title == "" {
				t.Errorf("row %d has no title", panel.ID)
			}
			continue
		}

		panels++
		if ids[panel.ID] {
			t.Errorf("the panel id %d is used twice", panel.ID)
		}
		ids[panel.ID] = true

		if panel.Title == "" {
			t.Errorf("panel %d has no title", panel.ID)
		}
		if panel.Datasource.UID != "${"+monitoringDashboardInput+"}" || panel.Datasource.Type != "prometheus" {
			t.Errorf("panel %d (%s) must use the %s datasource, got %+v", panel.ID, panel.Title, monitoringDashboardInput, panel.Datasource)
		}
		if panel.GridPos.W < 1 || panel.GridPos.W > 24 || panel.GridPos.X < 0 || panel.GridPos.X+panel.GridPos.W > 24 {
			t.Errorf("panel %d (%s) does not fit the 24 column grid: %+v", panel.ID, panel.Title, panel.GridPos)
		}
		if len(panel.Targets) == 0 {
			t.Errorf("panel %d (%s) has no target", panel.ID, panel.Title)
		}
		for _, target := range panel.Targets {
			if strings.TrimSpace(target.Expr) == "" {
				t.Errorf("panel %d (%s) has an empty expression", panel.ID, panel.Title)
			}
			if target.RefID == "" {
				t.Errorf("panel %d (%s) has a target without a refId", panel.ID, panel.Title)
			}
			if target.Datasource.UID != "${"+monitoringDashboardInput+"}" {
				t.Errorf("panel %d (%s) has a target outside the %s datasource", panel.ID, panel.Title, monitoringDashboardInput)
			}
			exprs.WriteString(target.Expr)
			exprs.WriteString("\n")
		}
	}

	if panels < 20 {
		t.Errorf("expected the dashboard to carry the whole surface, got %d panels", panels)
	}

	all := exprs.String()
	for _, required := range []string{
		"mgke_cache_groups_last_update_seconds",
		"mgke_parser_lag_seconds",
		"mgke_calendar_runs_total",
		"mgke_calendar_lag_seconds",
		"mgke_notifications_total",
		"mgke_telegram_commands_total",
		"mgke_telegram_send_errors_total",
		"mgke_api_endpoint_errors_total",
		"mgke_api_endpoint_latency_seconds_avg",
		"mgke_archive_rows",
		"mgke_reminders_tracked",
	} {
		if !strings.Contains(all, required) {
			t.Errorf("the dashboard must carry the %s panel the plan asks for", required)
		}
	}
}

func TestMonitoringKubernetesCopiesMatchThePlainFiles(t *testing.T) {
	rules := parseMonitoring[monitoringRuleFile](t, monitoringRulesFile)
	customResource := parseMonitoring[monitoringPrometheusRule](t, monitoringK8sRules)
	if !reflect.DeepEqual(rules.Groups, customResource.Spec.Groups) {
		t.Errorf("%s drifted from %s; regenerate it by indenting the groups block of the rules file under spec:", monitoringK8sRules, monitoringRulesFile)
	}

	configMap := parseMonitoring[struct {
		Data map[string]string `yaml:"data"`
	}](t, monitoringK8sDashboard)
	embedded, ok := configMap.Data[monitoringConfigMapKey]
	if !ok {
		t.Fatalf("%s must carry the %s key, got %v", monitoringK8sDashboard, monitoringConfigMapKey, configMap.Data)
	}

	var fromFile, fromConfigMap any
	fileData, err := os.ReadFile(filepath.Join("..", monitoringDashboard))
	if err != nil {
		t.Fatalf("read %s: %v", monitoringDashboard, err)
	}
	if err := json.Unmarshal(fileData, &fromFile); err != nil {
		t.Fatalf("parse %s: %v", monitoringDashboard, err)
	}
	if err := json.Unmarshal([]byte(embedded), &fromConfigMap); err != nil {
		t.Fatalf("parse the %s key: %v", monitoringConfigMapKey, err)
	}
	if !reflect.DeepEqual(fromFile, fromConfigMap) {
		t.Errorf("%s drifted from %s; regenerate it by indenting the dashboard under the %s key", monitoringK8sDashboard, monitoringDashboard, monitoringConfigMapKey)
	}

	serviceMonitor := readFile(t, monitoringServiceMon)
	for _, token := range []string{"monitoring.coreos.com/v1", "kind: ServiceMonitor", "kind: Service"} {
		if !strings.Contains(serviceMonitor, token) {
			t.Errorf("%s must carry %q", monitoringServiceMon, token)
		}
	}
	if !strings.Contains(readFile(t, monitoringK8sRules), "monitoring.coreos.com/v1") {
		t.Errorf("%s must use the Prometheus Operator custom resource", monitoringK8sRules)
	}
}

func TestMonitoringPromtoolTestsMatchTheRules(t *testing.T) {
	rules := parseMonitoring[monitoringRuleFile](t, monitoringRulesFile)
	byName := map[string]monitoringRule{}
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			byName[rule.Alert] = rule
		}
	}

	file := parseMonitoring[monitoringRuleTestFile](t, monitoringRuleTests)
	if len(file.RuleFiles) != 1 || file.RuleFiles[0] != filepath.Base(monitoringRulesFile) {
		t.Errorf("the unit tests must load %s, got %v", filepath.Base(monitoringRulesFile), file.RuleFiles)
	}
	if _, err := time.ParseDuration(file.EvaluationInterval); err != nil {
		t.Errorf("the evaluation interval %q is not a duration: %v", file.EvaluationInterval, err)
	}
	if len(file.Tests) < 8 {
		t.Fatalf("expected the unit tests to cover the alert families, got %d cases", len(file.Tests))
	}

	covered := map[string]bool{}
	for _, testCase := range file.Tests {
		if strings.TrimSpace(testCase.Name) == "" {
			t.Error("every unit test needs a name that says what it pins")
		}
		if _, err := time.ParseDuration(testCase.Interval); err != nil {
			t.Errorf("test %q has an unusable interval %q: %v", testCase.Name, testCase.Interval, err)
		}
		if len(testCase.InputSeries) == 0 {
			t.Errorf("test %q has no input series", testCase.Name)
		}
		if len(testCase.AlertRuleTests) == 0 {
			t.Errorf("test %q asserts nothing", testCase.Name)
		}

		for _, assertion := range testCase.AlertRuleTests {
			rule, ok := byName[assertion.AlertName]
			if !ok {
				t.Errorf("test %q asserts %s, which the rules file does not declare", testCase.Name, assertion.AlertName)
				continue
			}
			covered[assertion.AlertName] = true

			if _, err := time.ParseDuration(assertion.EvalTime); err != nil {
				t.Errorf("test %q evaluates %s at an unusable time %q: %v", testCase.Name, assertion.AlertName, assertion.EvalTime, err)
			}
			for _, expected := range assertion.ExpAlerts {
				if expected.ExpLabels["severity"] != rule.Labels["severity"] {
					t.Errorf("test %q expects %s with severity %q, the rule declares %q", testCase.Name, assertion.AlertName, expected.ExpLabels["severity"], rule.Labels["severity"])
				}
				if !reflect.DeepEqual(expected.ExpAnnotations, rule.Annotations) {
					t.Errorf("test %q pins different annotations than the rule %s: promtool compares them verbatim", testCase.Name, assertion.AlertName)
				}
			}
		}
	}

	if len(covered) != len(byName) {
		var missing []string
		for name := range byName {
			if !covered[name] {
				missing = append(missing, name)
			}
		}
		sort.Strings(missing)
		t.Errorf("every rule needs a unit test expectation, missing %s", strings.Join(missing, ", "))
	}

	raw := readFile(t, monitoringRuleTests)
	for _, alert := range []string{"MgkeParserNeverFilled", "MgkeParserStale", "MgkeCalendarStale", "MgkeAPIErrorBurst", "MgkeSendFailures"} {
		if !strings.Contains(raw, "alertname: "+alert) {
			t.Errorf("the unit tests must pin the %s alert", alert)
		}
	}
}

func TestMonitoringComposeWiresTheMetricsRoute(t *testing.T) {
	compose := parseMonitoring[monitoringCompose](t, monitoringComposeFile)

	required := []string{"scrape-config", "prometheus", "grafana", "alertmanager"}
	for _, name := range required {
		service, ok := compose.Services[name]
		if !ok {
			t.Fatalf("the stack needs a %s service, got %v", name, compose.Services)
		}
		if service.Image == "" || strings.HasSuffix(service.Image, ":latest") || !strings.Contains(service.Image, ":") {
			t.Errorf("the %s service must pin a concrete image tag, got %q", name, service.Image)
		}
		for _, published := range service.Ports {
			if !strings.HasPrefix(published, "127.0.0.1:") {
				t.Errorf("the %s service publishes %q on every interface, want loopback only", name, published)
			}
		}
	}

	alertmanager := compose.Services["alertmanager"]
	if len(alertmanager.Profiles) != 1 || alertmanager.Profiles[0] != "alerts" {
		t.Errorf("Alertmanager must sit behind the alerts profile, got %v", alertmanager.Profiles)
	}

	prometheus := compose.Services["prometheus"]
	for _, mount := range []string{
		"prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro",
		"prometheus/rules:/etc/prometheus/rules:ro",
		"/etc/prometheus/secrets",
		"/etc/prometheus/targets",
	} {
		if !strings.Contains(strings.Join(prometheus.Volumes, " "), mount) {
			t.Errorf("Prometheus must mount %s, got %v", mount, prometheus.Volumes)
		}
	}
	if prometheus.DependsOn["scrape-config"].Condition != "service_completed_successfully" {
		t.Errorf("Prometheus must wait for the scrape config, got %+v", prometheus.DependsOn)
	}
	if !strings.Contains(strings.Join(prometheus.Volumes, " "), "prometheus-secrets:/etc/prometheus/secrets:ro") {
		t.Error("the token must reach Prometheus through a volume, never through the compose file")
	}
	composeText := readFile(t, monitoringComposeFile)
	if !strings.Contains(composeText, "MGKE_METRICS_TOKEN: ${MGKE_METRICS_TOKEN:-}") {
		t.Error("the token must arrive from the environment with an empty default, so an open metrics route works out of the box")
	}

	grafana := compose.Services["grafana"]
	for _, mount := range []string{"grafana/provisioning:/etc/grafana/provisioning:ro", "grafana/dashboards:/var/lib/grafana/dashboards:ro"} {
		if !strings.Contains(strings.Join(grafana.Volumes, " "), mount) {
			t.Errorf("Grafana must mount %s, got %v", mount, grafana.Volumes)
		}
	}

	scrapeConfig := compose.Services["scrape-config"]
	script := strings.Join(scrapeConfig.Command, " ")
	for _, token := range []string{"metrics-token", "mgke.json", "MGKE_METRICS_TARGET"} {
		if !strings.Contains(script, token) {
			t.Errorf("the scrape-config service must write %q, got %q", token, script)
		}
	}
	if !strings.Contains(strings.Join(prometheus.Volumes, " "), "prometheus-targets:/etc/prometheus/targets:ro") {
		t.Error("the scrape target must reach Prometheus through a volume, otherwise the stack needs editing")
	}

	promConfig := readFile(t, monitoringPromFile)
	for _, token := range []string{"/etc/prometheus/targets/mgke.json", "credentials_file: /etc/prometheus/secrets/metrics-token", "job_name: mgke"} {
		if !strings.Contains(promConfig, token) {
			t.Errorf("%s must keep %q", monitoringPromFile, token)
		}
	}
	if !strings.Contains(promConfig, "/etc/prometheus/rules/mgke.yml") {
		t.Errorf("%s must load the rules file by name, the promtool test next to it is not a Prometheus config", monitoringPromFile)
	}
	if !strings.Contains(readFile(t, monitoringAlertmanager), "receivers:") {
		t.Errorf("%s must define receivers, empty ones on purpose", monitoringAlertmanager)
	}
}

func TestMonitoringAlloyKeepsCredentialsInTheEnvironment(t *testing.T) {
	alloy := readFile(t, monitoringAlloy)

	for _, token := range []string{"prometheus.scrape", "prometheus.remote_write", "forward_to", "bearer_token    = sys.env(", "basic_auth"} {
		if !strings.Contains(alloy, token) {
			t.Errorf("%s must carry %q", monitoringAlloy, token)
		}
	}
	for _, credential := range []string{"GRAFANA_CLOUD_API_KEY", "GRAFANA_CLOUD_PROMETHEUS_URL", "MGKE_METRICS_TOKEN"} {
		if !strings.Contains(alloy, "sys.env(\""+credential+"\")") {
			t.Errorf("%s must take %s from the environment", monitoringAlloy, credential)
		}
	}
	if regexp.MustCompile(`(password|bearer_token)\s*=\s*"`).MatchString(alloy) {
		t.Errorf("%s must not carry a credential literal", monitoringAlloy)
	}
}

func TestMonitoringSetIsComplete(t *testing.T) {
	for _, name := range []string{
		monitoringReadme,
		monitoringPromFile,
		monitoringRulesFile,
		monitoringRuleTests,
		monitoringAlertmanager,
		monitoringDashboard,
		monitoringDatasource,
		monitoringDashboardCfg,
		monitoringServiceMon,
		monitoringK8sRules,
		monitoringK8sDashboard,
		monitoringAlloy,
		monitoringComposeFile,
	} {
		if strings.TrimSpace(readFile(t, name)) == "" {
			t.Errorf("%s must not be empty", name)
		}
	}

	if !strings.Contains(readFile(t, monitoringDashboardCfg), "/var/lib/grafana/dashboards") {
		t.Errorf("%s must provision the committed dashboard directory", monitoringDashboardCfg)
	}
}
