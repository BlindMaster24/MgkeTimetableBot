package parser

import (
	"fmt"
	"strings"
	"time"
)

const (
	SourceGroups   = "groups"
	SourceTeachers = "teachers"
	SourceCalls    = "calls"
	SourceTeam     = "team"
)

type Probe struct {
	Source   string `json:"source"`
	Selector string `json:"selector"`
	Expected string `json:"expected,omitempty"`
	Found    int    `json:"found"`
	Required bool   `json:"required"`
}

func (p Probe) OK() bool {
	return !p.Required || p.Found > 0
}

type Report struct {
	Source    string    `json:"source"`
	URL       string    `json:"url,omitempty"`
	At        time.Time `json:"at"`
	Items     int       `json:"items"`
	Probes    []Probe   `json:"probes,omitempty"`
	Warnings  []string  `json:"warnings,omitempty"`
	Fallbacks []string  `json:"fallbacks,omitempty"`
	KeptOld   bool      `json:"keptOld,omitempty"`
}

func (r Report) Failing() []Probe {
	var failing []Probe
	for _, probe := range r.Probes {
		if !probe.OK() {
			failing = append(failing, probe)
		}
	}
	return failing
}

func (r Report) OK() bool {
	return len(r.Failing()) == 0 && len(r.Warnings) == 0
}

func (r Report) Summary() string {
	parts := []string{fmt.Sprintf("%s: items=%d", r.Source, r.Items)}

	if failing := r.Failing(); len(failing) > 0 {
		names := make([]string, 0, len(failing))
		for _, probe := range failing {
			names = append(names, probe.Selector)
		}
		parts = append(parts, "no data for: "+strings.Join(names, ", "))
	}
	if len(r.Fallbacks) > 0 {
		parts = append(parts, "fallbacks: "+strings.Join(r.Fallbacks, ", "))
	}
	if len(r.Warnings) > 0 {
		parts = append(parts, "warnings: "+strings.Join(r.Warnings, "; "))
	}

	return strings.Join(parts, " | ")
}

func (r *Report) Warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

func (r *Report) KeepOld(reason string) {
	r.KeptOld = true
	r.Warn("previous data kept: %s", reason)
}

type reportBuilder struct {
	report Report
}

func newReport(source, url string) *reportBuilder {
	return &reportBuilder{report: Report{Source: source, URL: url}}
}

func (b *reportBuilder) probe(selector, expected string, found int, required bool) {
	b.report.Probes = append(b.report.Probes, Probe{
		Source:   b.report.Source,
		Selector: selector,
		Expected: expected,
		Found:    found,
		Required: required,
	})
}

func (b *reportBuilder) fallback(name string) {
	for _, existing := range b.report.Fallbacks {
		if existing == name {
			return
		}
	}
	b.report.Fallbacks = append(b.report.Fallbacks, name)
}

func (b *reportBuilder) warn(format string, args ...any) {
	b.report.Warn(format, args...)
}

func (b *reportBuilder) done(items int) Report {
	report := b.report
	report.Items = items
	report.At = time.Now()
	return report
}

func mergeReports(reports ...Report) Report {
	if len(reports) == 0 {
		return Report{At: time.Now()}
	}

	merged := reports[0]
	merged.Probes = nil

	probeIndex := make(map[string]int)
	seenWarnings := make(map[string]bool)
	seenFallbacks := make(map[string]bool)

	for _, report := range reports {
		merged.Items += report.Items
		merged.KeptOld = merged.KeptOld || report.KeptOld

		for _, probe := range report.Probes {
			key := probe.Source + "|" + probe.Selector
			if index, exists := probeIndex[key]; exists {
				merged.Probes[index].Found += probe.Found
				continue
			}
			probeIndex[key] = len(merged.Probes)
			merged.Probes = append(merged.Probes, probe)
		}

		for _, warning := range report.Warnings {
			if seenWarnings[warning] {
				continue
			}
			seenWarnings[warning] = true
			merged.Warnings = append(merged.Warnings, warning)
		}

		for _, fallback := range report.Fallbacks {
			if seenFallbacks[fallback] {
				continue
			}
			seenFallbacks[fallback] = true
			merged.Fallbacks = append(merged.Fallbacks, fallback)
		}

		if report.URL == "" {
			continue
		}
		if merged.URL == "" {
			merged.URL = report.URL
			continue
		}
		merged.URL += ", " + report.URL
	}

	merged.At = time.Now()
	return merged
}
