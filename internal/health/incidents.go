package health

import (
	"sync"
	"time"
)

const (
	IncidentsStateKey = "health.incidents"
	IncidentLimit     = 100

	ScopeParser   = "parser"
	ScopeCalendar = "calendar"
	ScopeAPI      = "api"

	ResolutionAuto   = "auto"
	ResolutionManual = "manual"
)

type StartupChange int

const (
	ChangeRestart StartupChange = iota
	ChangeConfig
	ChangeDeploy
)

type Incident struct {
	Key        string    `json:"key"`
	Scope      string    `json:"scope"`
	Detail     string    `json:"detail,omitempty"`
	Build      string    `json:"build,omitempty"`
	Config     string    `json:"config,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt,omitempty"`
	Resolution string    `json:"resolution,omitempty"`
	Note       string    `json:"note,omitempty"`
}

func (i Incident) Open() bool {
	return i.EndedAt.IsZero()
}

func (i Incident) Duration(at time.Time) time.Duration {
	if i.Open() {
		if at.Before(i.StartedAt) {
			return 0
		}
		return at.Sub(i.StartedAt)
	}
	return i.EndedAt.Sub(i.StartedAt)
}

type incidentState struct {
	Records []Incident `json:"records,omitempty"`
}

type StartupStamp struct {
	Build  string
	Config string
}

func (s StartupStamp) ChangeFrom(previous StartupStamp) StartupChange {
	if previous.Build != "" && previous.Build != s.Build {
		return ChangeDeploy
	}
	if previous.Config != "" && previous.Config != s.Config {
		return ChangeConfig
	}
	return ChangeRestart
}

type StartupNote func(change StartupChange, previous, current StartupStamp) string

type IncidentLog struct {
	mu       sync.Mutex
	store    StateStore
	limit    int
	stamp    StartupStamp
	records  []Incident
	restored bool
}

func NewIncidentLog(store StateStore, limit int) *IncidentLog {
	if limit <= 0 {
		limit = IncidentLimit
	}
	return &IncidentLog{store: store, limit: limit}
}

func AlertScope(key string) string {
	switch key {
	case AlertParserFailures, AlertParserStale, AlertParserLayout, AlertParserGuard:
		return ScopeParser
	case AlertCalendarFailures, AlertCalendarStale:
		return ScopeCalendar
	case AlertAPIErrors:
		return ScopeAPI
	}
	return ""
}

func (l *IncidentLog) SetStartupStamp(stamp StartupStamp) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.stamp = stamp
}

func (l *IncidentLog) NoteStartup(scope string, current StartupStamp, note StartupNote) []Incident {
	if l == nil || scope == "" || note == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	var annotated []Incident
	for index := range l.records {
		record := &l.records[index]
		if record.Scope != scope || !record.Open() {
			continue
		}
		previous := StartupStamp{Build: record.Build, Config: record.Config}
		record.Resolution = ResolutionManual
		record.Note = note(previous.ChangeFrom(current), previous, current)
		record.Build = current.Build
		record.Config = current.Config
		annotated = append(annotated, *record)
	}
	if len(annotated) == 0 {
		return nil
	}
	l.flushLocked()
	return annotated
}

func (l *IncidentLog) Record(key, detail string) {
	if l == nil || key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	for _, record := range l.records {
		if record.Key == key && record.Open() {
			return
		}
	}

	l.records = append(l.records, Incident{
		Key:       key,
		Scope:     AlertScope(key),
		Detail:    detail,
		Build:     l.stamp.Build,
		Config:    l.stamp.Config,
		StartedAt: time.Now(),
	})
	l.trimLocked()
	l.flushLocked()
}

func (l *IncidentLog) Resolve(key string) {
	if l == nil || key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	changed := false
	for index := range l.records {
		record := &l.records[index]
		if record.Key != key || !record.Open() {
			continue
		}
		record.EndedAt = time.Now()
		if record.Resolution == "" {
			record.Resolution = ResolutionAuto
		}
		changed = true
	}
	if !changed {
		return
	}
	l.flushLocked()
}

func (l *IncidentLog) MarkManual(scope, note string) bool {
	if l == nil || scope == "" {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	marked := false
	for index := range l.records {
		record := &l.records[index]
		if record.Scope != scope || !record.Open() {
			continue
		}
		record.Resolution = ResolutionManual
		record.Note = note
		marked = true
	}
	if marked {
		l.flushLocked()
	}
	return marked
}

func (l *IncidentLog) Recent(limit int) []Incident {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	if limit <= 0 || limit > len(l.records) {
		limit = len(l.records)
	}
	out := make([]Incident, 0, limit)
	for index := len(l.records) - 1; index >= 0 && len(out) < limit; index-- {
		out = append(out, l.records[index])
	}
	return out
}

func (l *IncidentLog) Open() []Incident {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.restoreLocked()

	var out []Incident
	for index := len(l.records) - 1; index >= 0; index-- {
		if l.records[index].Open() {
			out = append(out, l.records[index])
		}
	}
	return out
}

func (l *IncidentLog) restoreLocked() {
	if l.restored {
		return
	}
	l.restored = true

	var state incidentState
	if err := LoadState(l.store, IncidentsStateKey, &state); err != nil {
		return
	}
	l.records = state.Records
}

func (l *IncidentLog) trimLocked() {
	if len(l.records) <= l.limit {
		return
	}
	l.records = append([]Incident(nil), l.records[len(l.records)-l.limit:]...)
}

func (l *IncidentLog) flushLocked() {
	if l.store == nil {
		return
	}
	state := incidentState{Records: l.records}
	_ = SaveState(l.store, IncidentsStateKey, state)
}
