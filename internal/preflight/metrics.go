package preflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

const (
	metricsCheckName      = "metrics"
	metricsChallenge      = `Bearer realm="metrics"`
	metricsProbeAddr      = "127.0.0.1:0"
	metricsProbeDirPrefix = "mgke-preflight-metrics-"
	metricsProbeTimeout   = 5 * time.Second
	metricsBodyLimit      = 1 << 20
	metricsDeployDir      = "deploy/monitoring"
)

var (
	monitoringMetricRe = regexp.MustCompile(`mgke_[a-z0-9_]+`)

	monitoringFileExts = map[string]bool{
		".yml":   true,
		".yaml":  true,
		".json":  true,
		".alloy": true,
	}

	archiveMetricFamilies = []string{
		"mgke_archive_first_day_index",
		"mgke_archive_last_day_index",
		"mgke_archive_rows",
		"mgke_archive_rows_with_group",
		"mgke_archive_rows_with_teacher",
	}

	noticerMetricFamilies = []string{
		"mgke_reminders_tracked",
		"mgke_scheduler_entries",
	}
)

type metricsProbeOptions struct {
	handler func(cacheDir, path, token string) (http.Handler, error)
	root    func() string
	timeout time.Duration
}

func checkMetrics(cfg *config.Config) Check {
	return runMetricsCheck(cfg, metricsProbeOptions{
		handler: func(cacheDir, path, token string) (http.Handler, error) {
			return metricsAPIHandler(cfg, cacheDir, path, token)
		},
		root:    findRepoRoot,
		timeout: metricsProbeTimeout,
	})
}

func metricsAPIHandler(cfg *config.Config, cacheDir, path, token string) (http.Handler, error) {
	raspCache, err := cache.New(cacheDir)
	if err != nil {
		return nil, err
	}

	counters := health.NewTelegramCounters()
	counters.CountCommand("/day")
	counters.CountCallback("day")
	counters.CountNotification("reminder")
	counters.CountSendError("send_message")

	sources := api.MetricsSources{
		Telegram: counters.TelegramStats,
		Webhook: func(context.Context) api.WebhookMetrics {
			return api.WebhookMetrics{
				Enabled: cfg != nil && cfg.Telegram.Webhook.Enabled,
				Mode:    webhookModeForConfig(cfg),
			}
		},
	}
	if archiveAvailable(cfg) {
		sources.Archive = func() (archive.Stats, error) { return archive.Stats{}, nil }
		sources.Bounds = func() (archive.Bounds, error) { return archive.Bounds{}, nil }
	}
	if cfg != nil && cfg.Telegram.Noticer {
		sources.Scheduler = func() int { return 0 }
		sources.Reminders = func() int { return 0 }
	}

	server := api.NewServer(raspCache, 0, health.NewDefaultTracker(), build.Info{}, nil, nil)
	server.SetMetricsConfig(api.MetricsConfig{Path: path, Token: token})
	server.SetMetricsSources(sources)
	return server.Handler(), nil
}

func webhookModeForConfig(cfg *config.Config) string {
	if cfg != nil && cfg.Telegram.Webhook.Enabled {
		return "webhook"
	}
	return "long_polling"
}

func archiveAvailable(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	path := cfg.ResolvedDBPath()
	if adopted, ok := config.AdoptLegacyDBPath(path); ok {
		path = adopted
	}
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func excusedMetricFamilies(cfg *config.Config, archiveReady bool) map[string]bool {
	excused := make(map[string]bool)
	if !archiveReady {
		for _, name := range archiveMetricFamilies {
			excused[name] = true
		}
	}
	if cfg == nil || !cfg.Telegram.Noticer {
		for _, name := range noticerMetricFamilies {
			excused[name] = true
		}
	}
	return excused
}

func runMetricsCheck(cfg *config.Config, opts metricsProbeOptions) Check {
	if cfg == nil || !cfg.Metrics.Enabled {
		return Check{Name: metricsCheckName, Level: LevelSkip, Detail: "metrics are disabled"}
	}

	opts = opts.withDefaults()

	probe, err := startMetricsProbe(cfg, opts)
	if err != nil {
		return metricsFailed(cfg, "the metrics route could not be served", err)
	}
	defer probe.close()

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	var failures []string
	var warnings []string

	status, header, _, err := probe.scrape(ctx, "")
	if err != nil {
		return metricsFailed(cfg, "the scrape without a token failed", err)
	}

	switch {
	case probe.token != "" && status != http.StatusUnauthorized:
		failures = append(failures, fmt.Sprintf("a scrape without the token answered %d, want 401", status))
	case probe.token != "" && header.Get("WWW-Authenticate") != metricsChallenge:
		warnings = append(warnings, fmt.Sprintf("a rejected scrape should challenge with %q, got %q", metricsChallenge, header.Get("WWW-Authenticate")))
	case probe.token == "" && status != http.StatusOK:
		failures = append(failures, fmt.Sprintf("a scrape answered %d while metrics.token is empty, want 200", status))
	case probe.token == "":
		warnings = append(warnings, "metrics.token is empty, anyone who reaches the API port can read the aggregates")
	}

	authorization := ""
	if probe.token != "" {
		authorization = "Bearer " + probe.token
	}

	status, _, body, err := probe.scrape(ctx, authorization)
	if err != nil {
		return metricsFailed(cfg, "the scrape with the token failed", err)
	}
	if status != http.StatusOK {
		return metricsFailed(cfg, "the scrape with the token was refused", fmt.Errorf("a scrape with the token answered %d, want 200", status))
	}

	kinds, err := metricFamilyKinds(body)
	if err != nil {
		return metricsFailed(cfg, "the scrape is not a valid prometheus exposition", err)
	}
	helps, duplicates := scanMetricLines(body)
	referenced := referencedMetricFamilies(opts.root())

	excused := excusedMetricFamilies(cfg, archiveAvailable(cfg))

	for _, name := range requiredMetricFamilies(excused) {
		kind, ok := kinds[name]
		if !ok {
			failures = append(failures, name+" is missing from the scrape")
			continue
		}
		if !helps[name] {
			failures = append(failures, name+" is served without a HELP line")
		}
		if kind != "COUNTER" && kind != "GAUGE" {
			failures = append(failures, name+" is served as "+kind)
		}
	}

	for _, name := range referenced {
		if _, ok := kinds[name]; ok {
			continue
		}
		if excused[name] {
			continue
		}
		failures = append(failures, name+" is used by the monitoring deploy but missing from the scrape")
	}

	if len(duplicates) > 0 {
		failures = append(failures, "the scrape repeats the series "+strings.Join(duplicates, ", "))
	}
	if err := probe.serveFailure(); err != nil {
		failures = append(failures, "the probe listener failed: "+err.Error())
	}

	detail := fmt.Sprintf("%d families over %s, %d metric names referenced by the monitoring deploy", len(kinds), probe.path, len(referenced))
	return metricsVerdict(cfg, detail, failures, warnings)
}

func metricsVerdict(cfg *config.Config, detail string, failures, warnings []string) Check {
	hints := make([]string, 0, len(failures)+len(warnings)+len(metricsHints(cfg)))
	for _, failure := range failures {
		hints = append(hints, "error: "+failure)
	}
	for _, warning := range warnings {
		hints = append(hints, "warning: "+warning)
	}

	switch {
	case len(failures) > 0:
		return Check{Name: metricsCheckName, Level: LevelFail, Detail: detail, Hints: append(hints, metricsHints(cfg)...)}
	case len(warnings) > 0:
		return Check{Name: metricsCheckName, Level: LevelWarn, Detail: detail, Hints: hints}
	}
	return Check{Name: metricsCheckName, Level: LevelOK, Detail: detail}
}

func metricsFailed(cfg *config.Config, detail string, err error) Check {
	var hints []string
	if err != nil {
		hints = append(hints, "error: "+err.Error())
	}
	return Check{Name: metricsCheckName, Level: LevelFail, Detail: detail, Hints: append(hints, metricsHints(cfg)...)}
}

func metricsHints(cfg *config.Config) []string {
	token := "empty"
	if strings.TrimSpace(cfg.Metrics.Token) != "" {
		token = "set"
	}

	return []string{
		fmt.Sprintf("config: metrics.enabled=%t, metrics.path=%q, metrics.token=%s", cfg.Metrics.Enabled, cfg.ResolvedMetricsPath(), token),
		"curl -s -H 'Authorization: Bearer <token>' http://127.0.0.1:<api port>" + cfg.ResolvedMetricsPath(),
		"the bot logs prometheus metrics enabled at startup and answers GET /api/health on the same port",
	}
}

func (o metricsProbeOptions) withDefaults() metricsProbeOptions {
	if o.handler == nil {
		o.handler = func(cacheDir, path, token string) (http.Handler, error) {
			return metricsAPIHandler(nil, cacheDir, path, token)
		}
	}
	if o.root == nil {
		o.root = findRepoRoot
	}
	if o.timeout <= 0 {
		o.timeout = metricsProbeTimeout
	}
	return o
}

type metricsProbe struct {
	server   *http.Server
	listener net.Listener
	dir      string
	baseURL  string
	path     string
	token    string
	client   *http.Client
	serveErr chan error
}

func startMetricsProbe(cfg *config.Config, opts metricsProbeOptions) (*metricsProbe, error) {
	dir, err := os.MkdirTemp("", metricsProbeDirPrefix)
	if err != nil {
		return nil, err
	}

	path := cfg.ResolvedMetricsPath()
	handler, err := routableMetricsHandler(opts.handler, filepath.Join(dir, "rasp"), path, cfg.Metrics.Token)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	listener, err := net.Listen("tcp", metricsProbeAddr)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	probe := &metricsProbe{
		server:   &http.Server{Handler: handler, ReadHeaderTimeout: opts.timeout},
		listener: listener,
		dir:      dir,
		baseURL:  "http://" + listener.Addr().String(),
		path:     path,
		token:    cfg.Metrics.Token,
		client:   &http.Client{Timeout: opts.timeout},
		serveErr: make(chan error, 1),
	}

	go func() {
		if err := probe.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			probe.serveErr <- err
		}
	}()

	return probe, nil
}

func routableMetricsHandler(factory func(cacheDir, path, token string) (http.Handler, error), cacheDir, path, token string) (handler http.Handler, err error) {
	defer func() {
		if value := recover(); value != nil {
			handler = nil
			err = fmt.Errorf("metrics.path %q is not routable: %v", path, value)
		}
	}()
	return factory(cacheDir, path, token)
}

func (p *metricsProbe) serveFailure() error {
	select {
	case err := <-p.serveErr:
		return err
	default:
		return nil
	}
}

func (p *metricsProbe) scrape(ctx context.Context, authorization string) (int, http.Header, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+p.path, nil)
	if err != nil {
		return 0, nil, "", err
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	response, err := p.client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, metricsBodyLimit))
	if err != nil {
		return response.StatusCode, response.Header, "", err
	}
	return response.StatusCode, response.Header, string(body), nil
}

func (p *metricsProbe) close() {
	ctx, cancel := context.WithTimeout(context.Background(), metricsProbeTimeout)
	defer cancel()

	_ = p.server.Shutdown(ctx)
	_ = p.listener.Close()
	if p.dir != "" {
		os.RemoveAll(p.dir)
	}
}

func metricFamilyKinds(body string) (map[string]string, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	kinds := make(map[string]string, len(families))
	for name, family := range families {
		kinds[name] = family.GetType().String()
	}
	return kinds, nil
}

func scanMetricLines(body string) (map[string]bool, []string) {
	helps := map[string]bool{}
	series := map[string]bool{}
	var duplicates []string

	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "# HELP "); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				helps[fields[0]] = true
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		key := seriesKey(line)
		if series[key] {
			duplicates = append(duplicates, key)
			continue
		}
		series[key] = true
	}

	sort.Strings(duplicates)
	return helps, duplicates
}

func seriesKey(line string) string {
	name, rest, hasLabels := strings.Cut(line, "{")
	if !hasLabels {
		name, _, _ = strings.Cut(line, " ")
		return strings.TrimSpace(name)
	}

	labels, _, found := strings.Cut(rest, "}")
	if !found {
		return line
	}
	parts := strings.Split(labels, ",")
	sort.Strings(parts)
	return name + "{" + strings.Join(parts, ",") + "}"
}

func requiredMetricFamilies(excused map[string]bool) []string {
	var required []string
	for _, name := range api.MetricFamilies() {
		if !excused[name] {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return required
}

func referencedMetricFamilies(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}

	found := map[string]bool{}
	_ = filepath.WalkDir(filepath.Join(root, metricsDeployDir), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !monitoringFileExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, name := range monitoringMetricRe.FindAllString(string(data), -1) {
			found[name] = true
		}
		return nil
	})

	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
