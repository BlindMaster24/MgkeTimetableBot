package preflight

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (r *Report) add(check Check) {
	if check.Level == "" {
		check.Level = LevelOK
	}
	r.Checks = append(r.Checks, check)
}

func (r *Report) finish(started time.Time) {
	r.ElapsedMS = time.Since(started).Milliseconds()
}

func (r Report) Count(level string) int {
	total := 0
	for _, check := range r.Checks {
		if check.Level == level {
			total++
		}
	}
	return total
}

func (r Report) Failed() bool {
	return r.Count(LevelFail) > 0
}

func (r Report) Warnings() int {
	return r.Count(LevelWarn)
}

func (r Report) Summary() string {
	return fmt.Sprintf("%d ok, %d warn, %d fail, %d skipped in %s",
		r.Count(LevelOK), r.Count(LevelWarn), r.Count(LevelFail), r.Count(LevelSkip),
		(time.Duration(r.ElapsedMS) * time.Millisecond).Round(time.Millisecond))
}

func (r Report) Text() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s %s %s=%s\n\n", r.title(), r.Started.Format("02.01.2006 15:04:05"), r.subjectLabel(), r.subject()))

	width := 0
	for _, check := range r.Checks {
		if len(check.Name) > width {
			width = len(check.Name)
		}
	}

	for _, check := range r.Checks {
		b.WriteString(fmt.Sprintf("[%s] %-*s  %s\n", check.Level, width, check.Name, check.Detail))
		for _, hint := range check.Hints {
			b.WriteString(fmt.Sprintf("       %s\n", hint))
		}
	}

	b.WriteString("\n")
	b.WriteString(r.Summary())
	if r.Incomplete {
		b.WriteString("\nresult: NOT VERIFIED, a skipped check above did not run")
	}
	b.WriteString("\n")
	return b.String()
}

func (r Report) title() string {
	if strings.TrimSpace(r.Title) == "" {
		return "preflight"
	}
	return r.Title
}

func (r Report) subjectLabel() string {
	if strings.TrimSpace(r.SubjectLabel) == "" {
		return "config"
	}
	return r.SubjectLabel
}

func (r Report) subject() string {
	if strings.TrimSpace(r.Subject) == "" {
		return r.ConfigPath
	}
	return r.Subject
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
