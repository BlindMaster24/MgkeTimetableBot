package health

import (
	"fmt"
	"sync"
	"time"
)

const (
	LevelWarning  = "warning"
	LevelCritical = "critical"

	AlertParserStale      = "parser_stale"
	AlertParserFailures   = "parser_failures"
	AlertCalendarStale    = "calendar_stale"
	AlertCalendarFailures = "calendar_failures"
	AlertAPIErrors        = "api_errors"
)

type Thresholds struct {
	ParserStale      time.Duration
	ParserFailures   int
	CalendarStale    time.Duration
	CalendarFailures int
	APIErrors        int
	APIWindow        time.Duration
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		ParserStale:      15 * time.Minute,
		ParserFailures:   3,
		CalendarStale:    6 * time.Hour,
		CalendarFailures: 3,
		APIErrors:        20,
		APIWindow:        5 * time.Minute,
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
	return t
}

type Alert struct {
	Key    string `json:"key"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
}

type ParserStats struct {
	Runs                int64  `json:"runs"`
	Errors              int64  `json:"errors"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LastSuccessAt       string `json:"lastSuccessAt,omitempty"`
	LastErrorAt         string `json:"lastErrorAt,omitempty"`
	LastError           string `json:"lastError,omitempty"`
	LagSeconds          int64  `json:"lagSeconds"`
	LastDurationMS      int64  `json:"lastDurationMs"`
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
	Requests       int64  `json:"requests"`
	Errors         int64  `json:"errors"`
	RecentErrors   int    `json:"recentErrors"`
	LastStatus     int    `json:"lastStatus"`
	LastErrorAt    string `json:"lastErrorAt,omitempty"`
	SlowestMillis  int64  `json:"slowestMillis"`
	LastDurationMS int64  `json:"lastDurationMs"`
}

type Snapshot struct {
	UptimeSeconds int64         `json:"uptimeSeconds"`
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

func (t *Tracker) APIRequest(status int, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.apiRequests++
	t.apiLastStatus = status
	t.apiLastDuration = duration
	if duration > t.apiSlowest {
		t.apiSlowest = duration
	}
	if status >= 500 {
		t.apiErrors++
		t.apiLastError = time.Now()
		t.apiRecentErrors = append(t.apiRecentErrors, t.apiLastError)
	}
	t.pruneAPIErrors(time.Now())
}

func (t *Tracker) Snapshot() Snapshot {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

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
		},
		Alerts: t.alertsLocked(now),
	}
}

func (t *Tracker) Alerts() []Alert {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.alertsLocked(time.Now())
}

func (t *Tracker) alertsLocked(now time.Time) []Alert {
	t.pruneAPIErrors(now)

	var alerts []Alert

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
			Detail: fmt.Sprintf("errors=%d window=%s", len(t.apiRecentErrors), t.thresholds.APIWindow),
		})
	}

	return alerts
}

func (t *Tracker) pruneAPIErrors(now time.Time) {
	if t.thresholds.APIWindow <= 0 || len(t.apiRecentErrors) == 0 {
		t.apiRecentErrors = nil
		return
	}

	cutoff := now.Add(-t.thresholds.APIWindow)
	kept := t.apiRecentErrors[:0]
	for _, at := range t.apiRecentErrors {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		t.apiRecentErrors = nil
		return
	}
	t.apiRecentErrors = kept
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
