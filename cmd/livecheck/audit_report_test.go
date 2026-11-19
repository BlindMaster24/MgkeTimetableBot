package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func TestReportAuditCountsEveryFinding(t *testing.T) {
	var out bytes.Buffer
	audit := &reportAudit{out: &out}

	audit.onReport(parser.Report{
		Source: "groups",
		Items:  3,
		Probes: []parser.Probe{
			{Source: "groups", Selector: "table", Expected: "tables", Found: 1, Required: true},
			{Source: "groups", Selector: "td.day", Expected: "days", Found: 0, Required: true},
		},
		Warnings:  []string{"shift header skipped"},
		Fallbacks: []string{"whole document"},
		KeptOld:   true,
	})

	if audit.Issues() != 3 {
		t.Fatalf("Issues = %d, want the failed probe, the warning and the kept cache", audit.Issues())
	}
	for _, want := range []string{
		"report groups: items=3",
		"probe failed: groups td.day",
		"warning: shift header skipped",
		"fallback: whole document",
		"kept previous cache entry",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output must contain %q, got %q", want, out.String())
		}
	}
}

func TestReportAuditCountsAFailedRequest(t *testing.T) {
	audit := &reportAudit{out: &bytes.Buffer{}}
	audit.fail()
	if audit.Issues() != 1 {
		t.Fatalf("Issues = %d, want one", audit.Issues())
	}
}

func TestCallsProblemsFindsEmptySchedulesAndSlots(t *testing.T) {
	if problems := callsProblems(cache.CallsSchedule{}); len(problems) != 1 || problems[0] != "active calls schedule is empty" {
		t.Fatalf("problems = %v, want the empty schedule", problems)
	}

	broken := cache.CallsSchedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}, {"08:55", "09:40"}},
			{{"", "09:45"}, {"09:55", "10:40"}},
			{{"10:45", "11:30"}, {"11:40", ""}},
		},
		Saturday: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
	}
	problems := callsProblems(broken)
	if len(problems) != 2 || !strings.Contains(problems[0], "slot 1") || !strings.Contains(problems[1], "slot 2") {
		t.Fatalf("problems = %v, want slots 1 and 2", problems)
	}

	healthy := cache.CallsSchedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		Saturday: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
	}
	if problems := callsProblems(healthy); problems != nil {
		t.Fatalf("problems = %v, want none", problems)
	}
}
