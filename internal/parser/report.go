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

const (
	KeepReasonEmpty  = "empty"
	KeepReasonShrink = "shrink"
)

type Keep struct {
	Reason       string `json:"reason"`
	Previous     int    `json:"previous,omitempty"`
	Current      int    `json:"current,omitempty"`
	LimitPercent int    `json:"limitPercent,omitempty"`
}

func (k Keep) Summary() string {
	if k.Reason == KeepReasonEmpty {
		return "the page produced nothing"
	}
	return fmt.Sprintf("%d -> %d (dropped %d%%, limit %d%%)", k.Previous, k.Current, k.DropPercent(), k.LimitPercent)
}

func (k Keep) DropPercent() int {
	if k.Previous <= 0 || k.Current >= k.Previous {
		return 0
	}
	return (k.Previous - k.Current) * 100 / k.Previous
}

type Report struct {
	Source    string    `json:"source"`
	URL       string    `json:"url,omitempty"`
	At        time.Time `json:"at"`
	Items     int       `json:"items"`
	Probes    []Probe   `json:"probes,omitempty"`
	Warnings  []string  `json:"warnings,omitempty"`
	Fallbacks []string  `json:"fallbacks,omitempty"`
	Variants  []string  `json:"variants,omitempty"`
	KeptOld   bool      `json:"keptOld,omitempty"`
	Keep      *Keep     `json:"keep,omitempty"`
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
	if len(r.Variants) > 0 {
		parts = append(parts, "variants: "+strings.Join(r.Variants, ", "))
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

func (r *Report) KeepEmpty(reason string) {
	r.Keep = &Keep{Reason: KeepReasonEmpty}
	r.KeepOld(reason)
}

func (r *Report) KeepShrunk(previous, current, limitPercent int, reason string) {
	r.Keep = &Keep{
		Reason:       KeepReasonShrink,
		Previous:     previous,
		Current:      current,
		LimitPercent: limitPercent,
	}
	r.KeepOld(reason)
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

func (b *reportBuilder) variant(name string) {
	if name == "" {
		return
	}
	for _, existing := range b.report.Variants {
		if existing == name {
			return
		}
	}
	b.report.Variants = append(b.report.Variants, name)
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
	seenVariants := make(map[string]bool)

	for _, report := range reports {
		merged.Items += report.Items
		merged.KeptOld = merged.KeptOld || report.KeptOld
		if merged.Keep == nil {
			merged.Keep = report.Keep
		}

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

		for _, variant := range report.Variants {
			if seenVariants[variant] {
				continue
			}
			seenVariants[variant] = true
			merged.Variants = append(merged.Variants, variant)
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
