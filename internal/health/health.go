package health

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
)

const (
	LevelWarning  = "warning"
	LevelCritical = "critical"

	AlertParserStale      = "parser_stale"
	AlertParserFailures   = "parser_failures"
	AlertParserLayout     = "parser_layout"
	AlertParserGuard      = "parser_guard"
	AlertCalendarStale    = "calendar_stale"
	AlertCalendarFailures = "calendar_failures"
	AlertAPIErrors        = "api_errors"
)

type Thresholds struct {
	ParserStale      time.Duration
	ParserFailures   int
	ParserLayout     int
	ParserGuard      int
	CalendarStale    time.Duration
	CalendarFailures int
	APIErrors        int
	APIWindow        time.Duration
	APISlow          time.Duration
}

const (
	apiEndpointAlertLimit = 3
	apiSlowAlertLimit     = 3
	apiLastErrorLimit     = 5
	apiMessageLimit       = 160
	apiEndpointRetention  = time.Hour
	apiEndpointLimit      = 64
)

func DefaultThresholds() Thresholds {
	return Thresholds{
		ParserStale:      15 * time.Minute,
		ParserFailures:   3,
		ParserLayout:     2,
		ParserGuard:      1,
		CalendarStale:    6 * time.Hour,
		CalendarFailures: 3,
		APIErrors:        20,
		APIWindow:        5 * time.Minute,
		APISlow:          500 * time.Millisecond,
	}
}

func (t Thresholds) WithDefaults() Thresholds {
	fallback := DefaultThresholds()

	if t.ParserStale <= 0 {
		t.ParserStale = fallback.ParserStale
	}
	if t.ParserFailures <= 0 {
		t.ParserFailures = fallback.ParserFailures
	}
	if t.ParserLayout <= 0 {
		t.ParserLayout = fallback.ParserLayout
	}
	if t.ParserGuard <= 0 {
		t.ParserGuard = fallback.ParserGuard
	}
	if t.CalendarStale <= 0 {
		t.CalendarStale = fallback.CalendarStale
	}
	if t.CalendarFailures <= 0 {
		t.CalendarFailures = fallback.CalendarFailures
	}
	if t.APIErrors <= 0 {
		t.APIErrors = fallback.APIErrors
	}
	if t.APIWindow <= 0 {
		t.APIWindow = fallback.APIWindow
	}
	if t.APISlow <= 0 {
		t.APISlow = fallback.APISlow
	}
	return t
}

type Alert struct {
	Key    string `json:"key"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
}

type LayoutIssue struct {
	Source   string `json:"source"`
	Selector string `json:"selector"`
	Expected string `json:"expected,omitempty"`
	Found    int    `json:"found"`
}

type GuardIssue struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type layoutState struct {
	issues   []LayoutIssue
	failures int
}

type guardState struct {
	issues   []GuardIssue
	failures int
}

type ParserStats struct {
	Runs                int64         `json:"runs"`
	Errors              int64         `json:"errors"`
	ConsecutiveFailures int           `json:"consecutiveFailures"`
	LastSuccessAt       string        `json:"lastSuccessAt,omitempty"`
	LastErrorAt         string        `json:"lastErrorAt,omitempty"`
	LastError           string        `json:"lastError,omitempty"`
	LagSeconds          int64         `json:"lagSeconds"`
	LastDurationMS      int64         `json:"lastDurationMs"`
	Layout              []LayoutIssue `json:"layout,omitempty"`
	LayoutFailures      int           `json:"layoutFailures"`
	Guard               []GuardIssue  `json:"guard,omitempty"`
	GuardFailures       int           `json:"guardFailures"`
}

type CalendarStats struct {
	Runs                int64  `json:"runs"`
	Errors              int64  `json:"errors"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	DaysSynced          int64  `json:"daysSynced"`
	LastSuccessAt       string `json:"lastSuccessAt,omitempty"`
	LastErrorAt         string `json:"lastErrorAt,omitempty"`
	LastError           string `json:"lastError,omitempty"`
	LagSeconds          int64  `json:"lagSeconds"`
}

type APIStats struct {
	Requests       int64             `json:"requests"`
	Errors         int64             `json:"errors"`
	RecentErrors   int               `json:"recentErrors"`
	LastStatus     int               `json:"lastStatus"`
	LastErrorAt    string            `json:"lastErrorAt,omitempty"`
	SlowestMillis  int64             `json:"slowestMillis"`
	LastDurationMS int64             `json:"lastDurationMs"`
	Endpoints      []APIEndpointStat `json:"endpoints,omitempty"`
	LastErrors     []APIErrorSample  `json:"lastErrors,omitempty"`
}

type APIRequest struct {
	Method    string
	Path      string
	Status    int
	Duration  time.Duration
	Message   string
	SelfProbe bool
}

type APIEndpointStat struct {
	Method     string `json:"method,omitempty"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	Requests   int    `json:"requests"`
	Errors     int    `json:"errors"`
	LastAt     string `json:"lastAt,omitempty"`
	Message    string `json:"message,omitempty"`
	LastMillis int64  `json:"lastMillis"`
	AvgMillis  int64  `json:"avgMillis"`
	MaxMillis  int64  `json:"maxMillis"`
	Slow       bool   `json:"slow,omitempty"`
}

type APIErrorSample struct {
	Method  string `json:"method,omitempty"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
	At      string `json:"at"`
	Message string `json:"message,omitempty"`
}

type apiEndpointState struct {
	method        string
	path          string
	status        int
	requests      int
	lastAt        time.Time
	message       string
	errorTimes    []time.Time
	durationTotal time.Duration
	durationCount int
	lastDuration  time.Duration
	maxDuration   time.Duration
}

func (s apiEndpointState) avgDuration() time.Duration {
	if s.durationCount == 0 {
		return 0
	}
	return s.durationTotal / time.Duration(s.durationCount)
}

type Snapshot struct {
	UptimeSeconds int64         `json:"uptimeSeconds"`
	Build         *build.Info   `json:"build,omitempty"`
	Parser        ParserStats   `json:"parser"`
	Calendar      CalendarStats `json:"calendar"`
	API           APIStats      `json:"api"`
	Alerts        []Alert       `json:"alerts"`
}

type Tracker struct {
	mu         sync.Mutex
	thresholds Thresholds
	startedAt  time.Time

	parserRuns          int64
	parserErrors        int64
	parserFailures      int
	parserLastSuccess   time.Time
	parserLastError     time.Time
	parserLastErrorText string
	parserLastDuration  time.Duration
	parserLayout        map[string]layoutState
	parserGuard         map[string]guardState

	calendarRuns          int64
	calendarErrors        int64
	calendarFailures      int
	calendarDaysSynced    int64
	calendarLastSuccess   time.Time
	calendarLastError     time.Time
	calendarLastErrorText string

	apiRequests     int64
	apiErrors       int64
	apiLastStatus   int
	apiLastError    time.Time
	apiSlowest      time.Duration
	apiLastDuration time.Duration
	apiRecentErrors []time.Time
	apiEndpoints    map[string]apiEndpointState
	apiLastErrors   []APIErrorSample
}

func NewTracker(thresholds Thresholds) *Tracker {
	return &Tracker{thresholds: thresholds, startedAt: time.Now()}
}

func NewDefaultTracker() *Tracker {
	return NewTracker(DefaultThresholds())
}

func (t *Tracker) ParserSuccess(duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.parserRuns++
	t.parserFailures = 0
	t.parserLastSuccess = time.Now()
	t.parserLastDuration = duration
}

func (t *Tracker) ParserFailure(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.parserRuns++
	t.parserErrors++
	t.parserFailures++
	t.parserLastError = time.Now()
	t.parserLastErrorText = errorText(err)
}

func (t *Tracker) ParserReport(source string, issues []LayoutIssue, guard []GuardIssue) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserLayout == nil {
		t.parserLayout = make(map[string]layoutState)
	}
	if t.parserGuard == nil {
		t.parserGuard = make(map[string]guardState)
	}

	state := t.parserLayout[source]
	if len(issues) > 0 {
		state.failures++
	} else {
		state.failures = 0
	}
	state.issues = issues
	t.parserLayout[source] = state

	guardState := t.parserGuard[source]
	if len(guard) > 0 {
		guardState.failures++
		guardState.issues = guard
	} else {
		guardState.failures = 0
		guardState.issues = nil
	}
	t.parserGuard[source] = guardState
}

func (t *Tracker) CalendarSuccess(days int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.calendarRuns++
	t.calendarFailures = 0
	t.calendarLastSuccess = time.Now()
	t.calendarDaysSynced += int64(days)
}

func (t *Tracker) CalendarFailure(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.calendarRuns++
	t.calendarErrors++
	t.calendarFailures++
	t.calendarLastError = time.Now()
	t.calendarLastErrorText = errorText(err)
}

func (t *Tracker) RecordAPI(req APIRequest) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.apiRequests++
	t.apiLastStatus = req.Status
	t.apiLastDuration = req.Duration
	if req.Duration > t.apiSlowest {
		t.apiSlowest = req.Duration
	}
	failed := req.Status >= 500 && !req.SelfProbe
	if failed {
		t.apiErrors++
		t.apiLastError = now
		t.apiRecentErrors = append(t.apiRecentErrors, now)
	}
	t.recordAPIEndpointLocked(req, now, failed)
	t.pruneAPIErrors(now)
}

func (t *Tracker) recordAPIEndpointLocked(req APIRequest, now time.Time, failed bool) {
	if req.Path == "" {
		return
	}
	if t.apiEndpoints == nil {
		t.apiEndpoints = make(map[string]apiEndpointState)
	}

	key := apiEndpointKey(req.Method, req.Path)
	state := t.apiEndpoints[key]
	state.method = req.Method
	state.path = req.Path
	state.status = req.Status
	state.requests++
	state.lastAt = now
	state.lastDuration = req.Duration
	state.durationTotal += req.Duration
	state.durationCount++
	if req.Duration > state.maxDuration {
		state.maxDuration = req.Duration
	}
	if failed {
		state.errorTimes = append(state.errorTimes, now)
		if message := apiMessage(req.Message); message != "" {
			state.message = message
		}
	}
	t.apiEndpoints[key] = state

	if !failed {
		return
	}

	sample := APIErrorSample{
		Method:  req.Method,
		Path:    req.Path,
		Status:  req.Status,
		At:      formatTime(now),
		Message: apiMessage(req.Message),
	}
	t.apiLastErrors = append([]APIErrorSample{sample}, t.apiLastErrors...)
	if len(t.apiLastErrors) > apiLastErrorLimit {
		t.apiLastErrors = t.apiLastErrors[:apiLastErrorLimit]
	}
}

func (t *Tracker) Snapshot() Snapshot {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneAPIErrors(now)

	return Snapshot{
		UptimeSeconds: int64(now.Sub(t.startedAt).Seconds()),
		Parser: ParserStats{
			Runs:                t.parserRuns,
			Errors:              t.parserErrors,
			ConsecutiveFailures: t.parserFailures,
			LastSuccessAt:       formatTime(t.parserLastSuccess),
			LastErrorAt:         formatTime(t.parserLastError),
			LastError:           t.parserLastErrorText,
			LagSeconds:          lagSeconds(now, t.parserLastSuccess),
			LastDurationMS:      t.parserLastDuration.Milliseconds(),
			Layout:              t.layoutIssuesLocked(),
			LayoutFailures:      t.layoutFailuresLocked(),
			Guard:               t.guardIssuesLocked(),
			GuardFailures:       t.guardFailuresLocked(),
		},
		Calendar: CalendarStats{
			Runs:                t.calendarRuns,
			Errors:              t.calendarErrors,
			ConsecutiveFailures: t.calendarFailures,
			DaysSynced:          t.calendarDaysSynced,
			LastSuccessAt:       formatTime(t.calendarLastSuccess),
			LastErrorAt:         formatTime(t.calendarLastError),
			LastError:           t.calendarLastErrorText,
			LagSeconds:          lagSeconds(now, t.calendarLastSuccess),
		},
		API: APIStats{
			Requests:       t.apiRequests,
			Errors:         t.apiErrors,
			RecentErrors:   len(t.apiRecentErrors),
			LastStatus:     t.apiLastStatus,
			LastErrorAt:    formatTime(t.apiLastError),
			SlowestMillis:  t.apiSlowest.Milliseconds(),
			LastDurationMS: t.apiLastDuration.Milliseconds(),
			Endpoints:      t.apiEndpointStatsLocked(),
			LastErrors:     append([]APIErrorSample(nil), t.apiLastErrors...),
		},
		Alerts: t.alertsLocked(now),
	}
}

func (t *Tracker) SlowThreshold() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.thresholds.APISlow
}

func (t *Tracker) Alerts() []Alert {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.alertsLocked(time.Now())
}

func (t *Tracker) layoutIssuesLocked() []LayoutIssue {
	sources := make([]string, 0, len(t.parserLayout))
	for source := range t.parserLayout {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var issues []LayoutIssue
	for _, source := range sources {
		issues = append(issues, t.parserLayout[source].issues...)
	}
	return issues
}

func (t *Tracker) layoutFailuresLocked() int {
	worst := 0
	for _, state := range t.parserLayout {
		if state.failures > worst {
			worst = state.failures
		}
	}
	return worst
}

func (t *Tracker) guardIssuesLocked() []GuardIssue {
	sources := make([]string, 0, len(t.parserGuard))
	for source := range t.parserGuard {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var issues []GuardIssue
	for _, source := range sources {
		issues = append(issues, t.parserGuard[source].issues...)
	}
	return issues
}

func (t *Tracker) guardFailuresLocked() int {
	worst := 0
	for _, state := range t.parserGuard {
		if state.failures > worst {
			worst = state.failures
		}
	}
	return worst
}

func (t *Tracker) alertsLocked(now time.Time) []Alert {
	t.pruneAPIErrors(now)

	var alerts []Alert

	if failures := t.layoutFailuresLocked(); failures >= t.thresholds.ParserLayout && t.thresholds.ParserLayout > 0 {
		alerts = append(alerts, Alert{
			Key:    AlertParserLayout,
			Level:  LevelWarning,
			Detail: fmt.Sprintf("runs=%d threshold=%d %s", failures, t.thresholds.ParserLayout, layoutDetail(t.layoutIssuesLocked())),
		})
	}

	if failures := t.guardFailuresLocked(); failures >= t.thresholds.ParserGuard && t.thresholds.ParserGuard > 0 {
		alerts = append(alerts, Alert{
			Key:    AlertParserGuard,
			Level:  LevelCritical,
			Detail: fmt.Sprintf("runs=%d threshold=%d %s", failures, t.thresholds.ParserGuard, guardDetail(t.guardIssuesLocked())),
		})
	}

	if t.parserFailures > 0 && t.parserFailures >= t.thresholds.ParserFailures {
		alerts = append(alerts, Alert{
			Key:    AlertParserFailures,
			Level:  LevelCritical,
			Detail: fmt.Sprintf("consecutiveFailures=%d threshold=%d", t.parserFailures, t.thresholds.ParserFailures),
		})
	}

	if !t.parserLastSuccess.IsZero() && t.thresholds.ParserStale > 0 {
		lag := now.Sub(t.parserLastSuccess)
		if lag >= t.thresholds.ParserStale {
			alerts = append(alerts, Alert{
				Key:    AlertParserStale,
				Level:  LevelWarning,
				Detail: fmt.Sprintf("lag=%s threshold=%s", lag.Truncate(time.Second), t.thresholds.ParserStale),
			})
		}
	}

	if t.calendarFailures > 0 && t.calendarFailures >= t.thresholds.CalendarFailures {
		alerts = append(alerts, Alert{
			Key:    AlertCalendarFailures,
			Level:  LevelCritical,
			Detail: fmt.Sprintf("consecutiveFailures=%d threshold=%d", t.calendarFailures, t.thresholds.CalendarFailures),
		})
	}

	if t.calendarRuns > 0 && !t.calendarLastSuccess.IsZero() && t.thresholds.CalendarStale > 0 {
		lag := now.Sub(t.calendarLastSuccess)
		if lag >= t.thresholds.CalendarStale {
			alerts = append(alerts, Alert{
				Key:    AlertCalendarStale,
				Level:  LevelWarning,
				Detail: fmt.Sprintf("lag=%s threshold=%s", lag.Truncate(time.Second), t.thresholds.CalendarStale),
			})
		}
	}

	if len(t.apiRecentErrors) >= t.thresholds.APIErrors && t.thresholds.APIErrors > 0 {
		alerts = append(alerts, Alert{
			Key:    AlertAPIErrors,
			Level:  LevelCritical,
			Detail: apiAlertDetail(len(t.apiRecentErrors), t.thresholds.APIWindow, t.apiEndpointStatsLocked(), t.apiLastErrors),
		})
	}

	return alerts
}

func guardDetail(issues []GuardIssue) string {
	if len(issues) == 0 {
		return ""
	}

	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s: %s: %s", issue.Source, issue.Reason, issue.Detail))
	}
	return strings.Join(parts, ", ")
}

func layoutDetail(issues []LayoutIssue) string {
	if len(issues) == 0 {
		return ""
	}

	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Source, issue.Selector))
	}
	return "missing " + strings.Join(parts, ", ")
}

func (t *Tracker) apiEndpointStatsLocked() []APIEndpointStat {
	if len(t.apiEndpoints) == 0 {
		return nil
	}

	stats := make([]APIEndpointStat, 0, len(t.apiEndpoints))
	for _, state := range t.apiEndpoints {
		stats = append(stats, t.apiEndpointStatLocked(state))
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Errors != stats[j].Errors {
			return stats[i].Errors > stats[j].Errors
		}
		if stats[i].Requests != stats[j].Requests {
			return stats[i].Requests > stats[j].Requests
		}
		return apiEndpointKey(stats[i].Method, stats[i].Path) < apiEndpointKey(stats[j].Method, stats[j].Path)
	})
	return stats
}

func (t *Tracker) apiEndpointStatLocked(state apiEndpointState) APIEndpointStat {
	stat := APIEndpointStat{
		Method:     state.method,
		Path:       state.path,
		Status:     state.status,
		Requests:   state.requests,
		Errors:     len(state.errorTimes),
		LastAt:     formatTime(state.lastAt),
		Message:    state.message,
		LastMillis: state.lastDuration.Milliseconds(),
		AvgMillis:  state.avgDuration().Milliseconds(),
		MaxMillis:  state.maxDuration.Milliseconds(),
	}
	if t.thresholds.APISlow > 0 {
		stat.Slow = state.lastDuration >= t.thresholds.APISlow || state.avgDuration() >= t.thresholds.APISlow
	}
	return stat
}

func apiAlertDetail(errors int, window time.Duration, endpoints []APIEndpointStat, last []APIErrorSample) string {
	lines := []string{fmt.Sprintf("errors=%d window=%s", errors, window)}

	if summary := apiEndpointSummary(endpoints); summary != "" {
		lines = append(lines, "paths: "+summary)
	}
	if summary := apiSlowSummary(endpoints); summary != "" {
		lines = append(lines, "slow: "+summary)
	}
	for _, sample := range apiLastErrorSummary(last) {
		lines = append(lines, "last: "+sample)
	}
	return strings.Join(lines, "\n")
}

func apiEndpointSummary(endpoints []APIEndpointStat) string {
	parts := make([]string, 0, apiEndpointAlertLimit)
	for _, endpoint := range endpoints {
		if endpoint.Errors <= 0 {
			continue
		}
		if len(parts) == apiEndpointAlertLimit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s x%d (%d)", apiEndpointKey(endpoint.Method, endpoint.Path), endpoint.Errors, endpoint.Status))
	}
	return strings.Join(parts, ", ")
}

func apiSlowSummary(endpoints []APIEndpointStat) string {
	slow := make([]APIEndpointStat, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Slow {
			slow = append(slow, endpoint)
		}
	}
	sort.Slice(slow, func(i, j int) bool {
		if slow[i].AvgMillis != slow[j].AvgMillis {
			return slow[i].AvgMillis > slow[j].AvgMillis
		}
		return apiEndpointKey(slow[i].Method, slow[i].Path) < apiEndpointKey(slow[j].Method, slow[j].Path)
	})
	if len(slow) > apiSlowAlertLimit {
		slow = slow[:apiSlowAlertLimit]
	}

	parts := make([]string, 0, len(slow))
	for _, endpoint := range slow {
		parts = append(parts, fmt.Sprintf("%s avg=%dms max=%dms n=%d (last %dms)",
			apiEndpointKey(endpoint.Method, endpoint.Path), endpoint.AvgMillis, endpoint.MaxMillis, endpoint.Requests, endpoint.LastMillis))
	}
	return strings.Join(parts, ", ")
}

func apiLastErrorSummary(samples []APIErrorSample) []string {
	parts := make([]string, 0, apiEndpointAlertLimit)
	for _, sample := range samples {
		if len(parts) == apiEndpointAlertLimit {
			break
		}
		line := fmt.Sprintf("%d %s", sample.Status, apiEndpointKey(sample.Method, sample.Path))
		if sample.Message != "" {
			line += ": " + sample.Message
		}
		if sample.At != "" {
			if at, err := time.Parse(time.RFC3339, sample.At); err == nil {
				line += " (" + at.Local().Format("15:04:05") + ")"
			}
		}
		parts = append(parts, line)
	}
	return parts
}

func apiEndpointKey(method, path string) string {
	if method == "" {
		return path
	}
	return method + " " + path
}

func apiMessage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && payload.Error != "" {
		trimmed = payload.Error
	}

	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) > apiMessageLimit {
		trimmed = strings.TrimSpace(trimmed[:apiMessageLimit]) + "..."
	}
	return trimmed
}

func (t *Tracker) pruneAPIErrors(now time.Time) {
	if t.thresholds.APIWindow <= 0 {
		t.apiRecentErrors = nil
		t.apiEndpoints = nil
		return
	}

	cutoff := now.Add(-t.thresholds.APIWindow)

	if len(t.apiRecentErrors) > 0 {
		kept := t.apiRecentErrors[:0]
		for _, at := range t.apiRecentErrors {
			if at.After(cutoff) {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			t.apiRecentErrors = nil
		} else {
			t.apiRecentErrors = kept
		}
	}

	retention := now.Add(-apiEndpointRetention)
	for key, state := range t.apiEndpoints {
		state.errorTimes = trimTimes(state.errorTimes, cutoff)
		if !state.lastAt.After(retention) {
			delete(t.apiEndpoints, key)
			continue
		}
		t.apiEndpoints[key] = state
	}
	t.trimAPIEndpointsLocked()
}

func trimTimes(times []time.Time, cutoff time.Time) []time.Time {
	kept := times[:0]
	for _, at := range times {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func (t *Tracker) trimAPIEndpointsLocked() {
	if len(t.apiEndpoints) <= apiEndpointLimit {
		return
	}

	keys := make([]string, 0, len(t.apiEndpoints))
	for key := range t.apiEndpoints {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return t.apiEndpoints[keys[i]].lastAt.After(t.apiEndpoints[keys[j]].lastAt)
	})
	for _, key := range keys[apiEndpointLimit:] {
		delete(t.apiEndpoints, key)
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func lagSeconds(now, then time.Time) int64 {
	if then.IsZero() {
		return 0
	}
	lag := now.Sub(then)
	if lag < 0 {
		return 0
	}
	return int64(lag.Seconds())
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
