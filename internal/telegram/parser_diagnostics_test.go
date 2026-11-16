package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func TestParserDiagnosticsIsSilentWithoutReports(t *testing.T) {
	b, _ := setupE2EBot(t)
	if got := b.parserDiagnostics(); got != nil {
		t.Fatalf("no reports must mean no diagnostics, got %v", got)
	}
}

func TestParserDiagnosticsSaysTheSourcesAreClean(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.RecordParserReport(parser.Report{
		Source: "groups",
		Items:  31,
		Probes: []parser.Probe{{Source: "groups", Selector: "table tr", Expected: "lessons", Found: 12, Missing: 0}},
	})

	lines := b.parserDiagnostics()
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	if !strings.Contains(lines[0], "groups: items=31") {
		t.Fatalf("first line = %q, want the source summary", lines[0])
	}
	if lines[1] != b.loc("parser_logs_clean") {
		t.Fatalf("last line = %q, want the clean verdict", lines[1])
	}
}

func TestParserDiagnosticsListsEveryKindOfTrouble(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.RecordParserReport(parser.Report{
		Source: "teachers",
		URL:    "https://example.com/teachers",
		Items:  0,
		Probes: []parser.Probe{
			{Source: "teachers", Selector: ".teacher", Expected: "name", Found: 0, Missing: 3, Inverted: true},
			{Source: "teachers", Selector: "table", Expected: "rows", Found: 0, Required: true},
			{Source: "teachers", Selector: ".advisory", Advisory: true, Found: 0},
		},
		Warnings:  []string{"empty page"},
		Fallbacks: []string{"kept the previous cache"},
	})

	lines := b.parserDiagnostics()
	joined := strings.Join(lines, "\n")
	if lines[0] != b.loc("parser_logs_diagnostics") {
		t.Fatalf("first line = %q, want the diagnostics header", lines[0])
	}
	for _, want := range []string{
		"teachers: items=0",
		b.loc("parser_logs_no_data"),
		"table (found 0)",
		"3",
		b.loc("parser_logs_advisory_prefix"),
		".advisory (found 0)",
		b.loc("parser_logs_warning_prefix") + "empty page",
		b.loc("parser_logs_fallback_prefix") + "kept the previous cache",
		"https://example.com/teachers",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the diagnostics lost %q in:\n%s", want, joined)
		}
	}
}

func TestParserDiagnosticsSkipsTheInvertedProbeText(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.RecordParserReport(parser.Report{
		Source: "calls",
		Items:  8,
		Probes: []parser.Probe{
			{Source: "calls", Selector: "table.calls", Expected: "no table", Found: 4, Missing: 2, Inverted: true},
		},
	})

	joined := strings.Join(b.parserDiagnostics(), "\n")
	if !strings.Contains(joined, "table.calls") || !strings.Contains(joined, "пропущено") || !strings.Contains(joined, "2") {
		t.Fatalf("an inverted probe must report the skipped count, got %q", joined)
	}
}
