package api

import (
	"context"
	"crypto/subtle"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/gin-gonic/gin"
)

const (
	DefaultMetricsPath = "/metrics"
	metricsContentType = "text/plain; version=0.0.4; charset=utf-8"
)

var metricFamilies = []string{
	"mgke_alerts_active",
	"mgke_api_endpoint_errors_total",
	"mgke_api_endpoint_latency_seconds_avg",
	"mgke_api_endpoint_requests_total",
	"mgke_api_errors_total",
	"mgke_api_requests_total",
	"mgke_api_slowest_seconds",
	"mgke_notifications_total",
	"mgke_archive_busy_failures_total",
	"mgke_archive_busy_retries_total",
	"mgke_archive_first_day_index",
	"mgke_archive_last_day_index",
	"mgke_archive_rows",
	"mgke_archive_rows_with_group",
	"mgke_archive_rows_with_teacher",
	"mgke_build_info",
	"mgke_cache_groups",
	"mgke_cache_groups_last_update_seconds",
	"mgke_cache_hits_total",
	"mgke_cache_last_success",
	"mgke_cache_misses_total",
	"mgke_cache_teachers",
	"mgke_cache_teachers_last_update_seconds",
	"mgke_calendar_consecutive_failures",
	"mgke_calendar_days_synced_total",
	"mgke_calendar_errors_total",
	"mgke_calendar_lag_seconds",
	"mgke_calendar_runs_total",
	"mgke_maintenance_failures",
	"mgke_maintenance_last_success_seconds",
	"mgke_parser_consecutive_failures",
	"mgke_parser_errors_total",
	"mgke_parser_guard_failures",
	"mgke_parser_lag_seconds",
	"mgke_parser_last_duration_seconds",
	"mgke_parser_layout_failures",
	"mgke_parser_runs_total",
	"mgke_reminders_tracked",
	"mgke_scheduler_entries",
	"mgke_telegram_callbacks_total",
	"mgke_telegram_commands_total",
	"mgke_telegram_send_errors_total",
	"mgke_uptime_seconds",
	"mgke_webhook_enabled",
	"mgke_webhook_mode",
	"mgke_webhook_pending_updates",
	"mgke_webhook_reachable",
	"mgke_webhook_registered",
	"mgke_webhook_secret_configured",
}

var metricLabelNames = []string{
	"callback",
	"command",
	"commit",
	"method",
	"mode",
	"notification",
	"path",
	"transport",
	"version",
}

func MetricFamilies() []string {
	return slices.Clone(metricFamilies)
}

func MetricLabelNames() []string {
	return slices.Clone(metricLabelNames)
}

var metricsMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

type MetricsConfig struct {
	Path  string
	Token string
}

type WebhookMetrics struct {
	Enabled    bool
	Mode       string
	Registered bool
	Secret     bool
	Reachable  bool
	Pending    int
}

type MetricsSources struct {
	Archive   func() (archive.Stats, error)
	Bounds    func() (archive.Bounds, error)
	Webhook   func(ctx context.Context) WebhookMetrics
	Scheduler func() int
	Reminders func() int
	Telegram  func() health.TelegramStats
}

func (s *Server) SetMetricsConfig(cfg MetricsConfig) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = DefaultMetricsPath
	}
	s.metricsPath = path
	s.metricsToken = strings.TrimSpace(cfg.Token)
	s.engine.GET(path, s.handleMetrics)
}

func (s *Server) SetMetricsSources(sources MetricsSources) {
	s.metricsSources = sources
}

func (s *Server) MetricsPath() string {
	return s.metricsPath
}

func (s *Server) handleMetrics(c *gin.Context) {
	if !s.metricsAuthorized(c) {
		c.Header("WWW-Authenticate", `Bearer realm="metrics"`)
		c.Data(http.StatusUnauthorized, "text/plain; charset=utf-8", []byte(s.message("api_unauthorized")+"\n"))
		return
	}

	c.Data(http.StatusOK, metricsContentType, []byte(s.renderMetrics(c.Request.Context())))
}

func (s *Server) metricsAuthorized(c *gin.Context) bool {
	token := strings.TrimSpace(s.metricsToken)
	if token == "" {
		return true
	}

	parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "bearer") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(parts[1])), []byte(token)) == 1
}

func (s *Server) renderMetrics(ctx context.Context) string {
	w := newPrometheusWriter()

	w.emit("mgke_build_info", "gauge", "Build information of the running bot.", "1",
		promLabel("version", s.build.Version), promLabel("commit", s.build.ShortCommit()))

	if s.cache != nil {
		stats := s.cache.Stats()
		w.emit("mgke_cache_groups", "gauge", "Groups held by the schedule cache.", strconv.Itoa(stats.GroupsCount))
		w.emit("mgke_cache_teachers", "gauge", "Teachers held by the schedule cache.", strconv.Itoa(stats.TeachersCount))
		w.emit("mgke_cache_hits_total", "counter", "Cache hits since the bot started.", strconv.FormatInt(stats.Hits, 10))
		w.emit("mgke_cache_misses_total", "counter", "Cache misses since the bot started.", strconv.FormatInt(stats.Misses, 10))
		w.emit("mgke_cache_last_success", "gauge", "Whether the last parse finished without an error.", boolValue(stats.SuccessUpdate))
		w.emit("mgke_cache_groups_last_update_seconds", "gauge", "Unix time of the last group update, 0 while the cache has never been filled.", unixSeconds(s.cache.GetGroupsUpdateTime()))
		w.emit("mgke_cache_teachers_last_update_seconds", "gauge", "Unix time of the last teacher update, 0 while the cache has never been filled.", unixSeconds(s.cache.GetTeachersUpdateTime()))
	}

	if s.health != nil {
		snapshot := s.health.Snapshot()
		parser := snapshot.Parser
		w.emit("mgke_uptime_seconds", "gauge", "Seconds the bot has been running.", strconv.FormatInt(snapshot.UptimeSeconds, 10))
		w.emit("mgke_parser_runs_total", "counter", "Timetable parse runs.", strconv.FormatInt(parser.Runs, 10))
		w.emit("mgke_parser_errors_total", "counter", "Timetable parse runs that failed.", strconv.FormatInt(parser.Errors, 10))
		w.emit("mgke_parser_consecutive_failures", "gauge", "Parse failures in a row.", strconv.Itoa(parser.ConsecutiveFailures))
		w.emit("mgke_parser_lag_seconds", "gauge", "Seconds since the last successful parse.", strconv.FormatInt(parser.LagSeconds, 10))
		w.emit("mgke_parser_last_duration_seconds", "gauge", "Duration of the last parse.", promSeconds(parser.LastDurationMS))
		w.emit("mgke_parser_layout_failures", "gauge", "Missing page selectors reported by the last parse.", strconv.Itoa(parser.LayoutFailures))
		w.emit("mgke_parser_guard_failures", "gauge", "Data-loss guards that fired on the last parse.", strconv.Itoa(parser.GuardFailures))

		calendar := snapshot.Calendar
		w.emit("mgke_calendar_runs_total", "counter", "Calendar sync runs.", strconv.FormatInt(calendar.Runs, 10))
		w.emit("mgke_calendar_errors_total", "counter", "Calendar sync runs that failed.", strconv.FormatInt(calendar.Errors, 10))
		w.emit("mgke_calendar_consecutive_failures", "gauge", "Calendar sync failures in a row.", strconv.Itoa(calendar.ConsecutiveFailures))
		w.emit("mgke_calendar_days_synced_total", "counter", "Calendar days written.", strconv.FormatInt(calendar.DaysSynced, 10))
		w.emit("mgke_calendar_lag_seconds", "gauge", "Seconds since the last successful calendar sync.", strconv.FormatInt(calendar.LagSeconds, 10))

		archiveStats := snapshot.Archive
		w.emit("mgke_archive_busy_retries_total", "counter", "Archive writes retried after a busy lock.", strconv.FormatInt(archiveStats.BusyRetries, 10))
		w.emit("mgke_archive_busy_failures_total", "counter", "Archive writes that stayed busy after the retries.", strconv.FormatInt(archiveStats.BusyFailures, 10))

		maintenance := snapshot.Maintenance
		w.emit("mgke_maintenance_failures", "gauge", "Backup and retention runs that failed in a row.", strconv.Itoa(maintenance.Failures))
		w.emit("mgke_maintenance_last_success_seconds", "gauge", "Unix time of the last successful backup and retention run, 0 while it never succeeded.", strconv.FormatInt(maintenance.LastSuccessUnix, 10))

		w.emit("mgke_alerts_active", "gauge", "Health alerts that are firing right now.", strconv.Itoa(len(snapshot.Alerts)))

		apiStats := snapshot.API
		w.emit("mgke_api_requests_total", "counter", "HTTP requests served by the API.", strconv.FormatInt(apiStats.Requests, 10))
		w.emit("mgke_api_errors_total", "counter", "HTTP requests that answered 5xx.", strconv.FormatInt(apiStats.Errors, 10))
		w.emit("mgke_api_slowest_seconds", "gauge", "Slowest API response since the bot started.", promSeconds(apiStats.SlowestMillis))
		s.renderEndpointMetrics(w, apiStats.Endpoints)
	}

	s.renderSourceMetrics(w, ctx)
	return w.String()
}

func (s *Server) renderEndpointMetrics(w *prometheusWriter, endpoints []health.APIEndpointStat) {
	routes := s.registeredRoutes()

	stats := make([]health.APIEndpointStat, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if !routes[endpoint.Path] {
			continue
		}
		stats = append(stats, endpoint)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Path != stats[j].Path {
			return stats[i].Path < stats[j].Path
		}
		return stats[i].Method < stats[j].Method
	})

	for _, endpoint := range stats {
		labels := []string{promLabel("method", metricsMethod(endpoint.Method)), promLabel("path", endpoint.Path)}
		w.emit("mgke_api_endpoint_requests_total", "counter", "Requests per API route.", strconv.Itoa(endpoint.Requests), labels...)
		w.emit("mgke_api_endpoint_errors_total", "counter", "5xx responses per API route.", strconv.Itoa(endpoint.Errors), labels...)
		w.emit("mgke_api_endpoint_latency_seconds_avg", "gauge", "Average response time per API route.", promSeconds(endpoint.AvgMillis), labels...)
	}
}

func (s *Server) renderSourceMetrics(w *prometheusWriter, ctx context.Context) {
	if s.metricsSources.Archive != nil {
		if stats, err := s.metricsSources.Archive(); err == nil {
			w.emit("mgke_archive_rows", "gauge", "Rows stored in the timetable archive.", strconv.Itoa(stats.Total))
			w.emit("mgke_archive_rows_with_group", "gauge", "Archive rows that belong to a group.", strconv.Itoa(stats.WithGroup))
			w.emit("mgke_archive_rows_with_teacher", "gauge", "Archive rows that belong to a teacher.", strconv.Itoa(stats.WithTeacher))
		}
	}

	if s.metricsSources.Bounds != nil {
		if bounds, err := s.metricsSources.Bounds(); err == nil {
			w.emit("mgke_archive_first_day_index", "gauge", "Oldest day kept in the archive.", strconv.FormatInt(bounds.Min, 10))
			w.emit("mgke_archive_last_day_index", "gauge", "Newest day kept in the archive.", strconv.FormatInt(bounds.Max, 10))
		}
	}

	if s.metricsSources.Scheduler != nil {
		w.emit("mgke_scheduler_entries", "gauge", "Cron entries the notification scheduler has armed.", strconv.Itoa(s.metricsSources.Scheduler()))
	}

	if s.metricsSources.Reminders != nil {
		w.emit("mgke_reminders_tracked", "gauge", "Lesson reminders currently remembered per chat.", strconv.Itoa(s.metricsSources.Reminders()))
	}

	if s.metricsSources.Telegram != nil {
		stats := s.metricsSources.Telegram()
		w.emitCounters("mgke_telegram_commands_total", "Telegram commands handled per command name.", "command", stats.Commands)
		w.emitCounters("mgke_telegram_callbacks_total", "Telegram callback buttons handled per prefix.", "callback", stats.Callbacks)
		w.emitCounters("mgke_notifications_total", "Notifications sent per notification kind.", "notification", stats.Notifications)
		w.emitCounters("mgke_telegram_send_errors_total", "Failed Telegram calls per transport.", "transport", stats.Transports)
	}

	if s.metricsSources.Webhook != nil {
		webhook := s.metricsSources.Webhook(ctx)
		w.emit("mgke_webhook_enabled", "gauge", "Whether the bot serves updates over a webhook.", boolValue(webhook.Enabled))
		w.emit("mgke_webhook_mode", "gauge", "Update transport the bot is configured for.", "1", promLabel("mode", webhookMode(webhook.Mode)))
		w.emit("mgke_webhook_registered", "gauge", "Whether Telegram knows the public webhook address.", boolValue(webhook.Registered))
		w.emit("mgke_webhook_secret_configured", "gauge", "Whether a webhook secret token is configured.", boolValue(webhook.Secret))
		w.emit("mgke_webhook_reachable", "gauge", "Whether the last getWebhookInfo call reached Telegram.", boolValue(webhook.Reachable))
		w.emit("mgke_webhook_pending_updates", "gauge", "Updates Telegram is holding for the webhook.", strconv.Itoa(webhook.Pending))
	}
}

func (s *Server) registeredRoutes() map[string]bool {
	routes := make(map[string]bool)
	for _, route := range s.engine.Routes() {
		routes[route.Path] = true
	}
	return routes
}

func metricsMethod(method string) string {
	upper := strings.ToUpper(strings.TrimSpace(method))
	if metricsMethods[upper] {
		return upper
	}
	return "OTHER"
}

func webhookMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "webhook") {
		return "webhook"
	}
	return "long_polling"
}

func boolValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func unixSeconds(value time.Time) string {
	if value.IsZero() {
		return "0"
	}
	return strconv.FormatInt(value.Unix(), 10)
}

func promSeconds(millis int64) string {
	return strconv.FormatFloat(float64(millis)/1000, 'f', -1, 64)
}

type prometheusWriter struct {
	builder strings.Builder
	emitted map[string]bool
}

func newPrometheusWriter() *prometheusWriter {
	return &prometheusWriter{emitted: make(map[string]bool)}
}

func (w *prometheusWriter) emit(name, kind, help, value string, labels ...string) {
	if !w.emitted[name] {
		w.emitted[name] = true
		w.builder.WriteString("# HELP " + name + " " + help + "\n")
		w.builder.WriteString("# TYPE " + name + " " + kind + "\n")
	}
	w.builder.WriteString(name)
	if len(labels) > 0 {
		w.builder.WriteString("{" + strings.Join(labels, ",") + "}")
	}
	w.builder.WriteString(" " + value + "\n")
}

func (w *prometheusWriter) emitCounters(name, help, label string, values map[string]int64) {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		w.emit(name, "counter", help, strconv.FormatInt(values[key], 10), promLabel(label, key))
	}
}

func (w *prometheusWriter) String() string {
	return w.builder.String()
}

var promLabelReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func promLabel(name, value string) string {
	return name + `="` + promLabelReplacer.Replace(value) + `"`
}
