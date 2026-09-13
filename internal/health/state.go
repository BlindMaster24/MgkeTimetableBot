package health

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TrackerStateKey = "health.tracker"
	AlertsStateKey  = "health.alerts"
)

type StateStore interface {
	LoadState(key string) (string, bool, error)
	SaveState(key, value string) error
}

type State struct {
	ParserRuns           int64          `json:"parserRuns"`
	ParserErrors         int64          `json:"parserErrors"`
	ParserFailures       int            `json:"parserFailures"`
	ParserLastSuccessAt  string         `json:"parserLastSuccessAt,omitempty"`
	ParserLastErrorAt    string         `json:"parserLastErrorAt,omitempty"`
	ParserLastError      string         `json:"parserLastError,omitempty"`
	ParserLastDurationMS int64          `json:"parserLastDurationMs"`
	ParserLayoutFailures map[string]int `json:"parserLayoutFailures,omitempty"`
	ParserGuardFailures  map[string]int `json:"parserGuardFailures,omitempty"`

	CalendarRuns          int64  `json:"calendarRuns"`
	CalendarErrors        int64  `json:"calendarErrors"`
	CalendarFailures      int    `json:"calendarFailures"`
	CalendarDaysSynced    int64  `json:"calendarDaysSynced"`
	CalendarLastSuccessAt string `json:"calendarLastSuccessAt,omitempty"`
	CalendarLastErrorAt   string `json:"calendarLastErrorAt,omitempty"`
	CalendarLastError     string `json:"calendarLastError,omitempty"`
}

func LoadState(store StateStore, key string, value any) error {
	if store == nil {
		return nil
	}

	raw, ok, err := store.LoadState(key)
	if err != nil || !ok || raw == "" {
		return err
	}
	if err := json.Unmarshal([]byte(raw), value); err != nil {
		return fmt.Errorf("decode state %s: %w", key, err)
	}
	return nil
}

func SaveState(store StateStore, key string, value any) error {
	if store == nil {
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode state %s: %w", key, err)
	}
	return store.SaveState(key, string(raw))
}

func (t *Tracker) Restore(store StateStore) error {
	if store == nil {
		return nil
	}

	raw, ok, err := store.LoadState(TrackerStateKey)
	if err != nil || !ok || raw == "" {
		return err
	}

	var state State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return fmt.Errorf("decode state %s: %w", TrackerStateKey, err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.applyStateLocked(state)
	return nil
}

func (t *Tracker) Flush(store StateStore) error {
	t.mu.Lock()
	state := t.stateLocked()
	t.mu.Unlock()

	return SaveState(store, TrackerStateKey, state)
}

func (t *Tracker) stateLocked() State {
	state := State{
		ParserRuns:           t.parserRuns,
		ParserErrors:         t.parserErrors,
		ParserFailures:       t.parserFailures,
		ParserLastSuccessAt:  formatTime(t.parserLastSuccess),
		ParserLastErrorAt:    formatTime(t.parserLastError),
		ParserLastError:      t.parserLastErrorText,
		ParserLastDurationMS: t.parserLastDuration.Milliseconds(),

		CalendarRuns:          t.calendarRuns,
		CalendarErrors:        t.calendarErrors,
		CalendarFailures:      t.calendarFailures,
		CalendarDaysSynced:    t.calendarDaysSynced,
		CalendarLastSuccessAt: formatTime(t.calendarLastSuccess),
		CalendarLastErrorAt:   formatTime(t.calendarLastError),
		CalendarLastError:     t.calendarLastErrorText,
	}

	for source, layout := range t.parserLayout {
		if layout.failures == 0 {
			continue
		}
		if state.ParserLayoutFailures == nil {
			state.ParserLayoutFailures = make(map[string]int)
		}
		state.ParserLayoutFailures[source] = layout.failures
	}

	for source, guard := range t.parserGuard {
		if guard.failures == 0 {
			continue
		}
		if state.ParserGuardFailures == nil {
			state.ParserGuardFailures = make(map[string]int)
		}
		state.ParserGuardFailures[source] = guard.failures
	}

	return state
}

func (t *Tracker) applyStateLocked(state State) {
	t.parserRuns = state.ParserRuns
	t.parserErrors = state.ParserErrors
	t.parserFailures = state.ParserFailures
	t.parserLastSuccess = parseTime(state.ParserLastSuccessAt)
	t.parserLastError = parseTime(state.ParserLastErrorAt)
	t.parserLastErrorText = state.ParserLastError
	t.parserLastDuration = time.Duration(state.ParserLastDurationMS) * time.Millisecond

	t.calendarRuns = state.CalendarRuns
	t.calendarErrors = state.CalendarErrors
	t.calendarFailures = state.CalendarFailures
	t.calendarDaysSynced = state.CalendarDaysSynced
	t.calendarLastSuccess = parseTime(state.CalendarLastSuccessAt)
	t.calendarLastError = parseTime(state.CalendarLastErrorAt)
	t.calendarLastErrorText = state.CalendarLastError

	for source, failures := range state.ParserLayoutFailures {
		if t.parserLayout == nil {
			t.parserLayout = make(map[string]layoutState)
		}
		t.parserLayout[source] = layoutState{failures: failures}
	}

	for source, failures := range state.ParserGuardFailures {
		if t.parserGuard == nil {
			t.parserGuard = make(map[string]guardState)
		}
		t.parserGuard[source] = guardState{failures: failures}
	}
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
