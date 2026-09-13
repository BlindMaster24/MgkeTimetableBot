package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func TestParserDiagnosticsRenderFailingSelectors(t *testing.T) {
	b := setupTestBot(t)

	report := parser.Report{Source: parser.SourceGroups, URL: "https://example.by/groups", Items: 3}
	report.Warn("group 84: day columns found, but every day is empty")
	report.Fallbacks = append(report.Fallbacks, "group label without the 'Группа -' prefix: 84")
	report.Probes = append(report.Probes, parser.Probe{
		Source:   parser.SourceGroups,
		Selector: "td lesson cells",
		Expected: "groups with at least one lesson",
		Found:    0,
		Required: true,
	})

	b.RecordParserReport(report)

	lines := strings.Join(b.parserDiagnostics(), "\n")
	for _, want := range []string{
		"groups",
		"items=3",
		"td lesson cells",
		"https://example.by/groups",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("diagnostics omit %q:\n%s", want, lines)
		}
	}
}

func TestParserDiagnosticsReportCleanState(t *testing.T) {
	b := setupTestBot(t)

	b.RecordParserReport(parser.Report{Source: parser.SourceCalls, Items: 7})

	lines := strings.Join(b.parserDiagnostics(), "\n")
	if !strings.Contains(lines, b.loc("parser_logs_clean")) {
		t.Errorf("expected the clean marker, got:\n%s", lines)
	}
}

func TestParserDiagnosticsEmptyWithoutReports(t *testing.T) {
	b := setupTestBot(t)

	if lines := b.parserDiagnostics(); len(lines) != 0 {
		t.Errorf("expected no diagnostics before the first parse, got %v", lines)
	}
}

func TestParserLogsCommandShowsDiagnostics(t *testing.T) {
	b := setupTestBot(t)
	b.cfg.Telegram.AdminIDs = []int64{42}

	report := parser.Report{Source: parser.SourceTeachers, Items: 0}
	report.Probes = append(report.Probes, parser.Probe{
		Source:   parser.SourceTeachers,
		Selector: "heading: Преподаватель - <ФИО>",
		Found:    0,
		Required: true,
	})
	b.RecordParserReport(report)

	handler := &parserLogsCmd{bot: b}
	if !handler.MatchText("/parserlogs") {
		t.Fatal("the command should match /parserlogs")
	}

	text := strings.Join(b.parserDiagnostics(), "\n")
	if !strings.Contains(text, "heading: Преподаватель - <ФИО>") {
		t.Errorf("teacher selector is missing from diagnostics:\n%s", text)
	}
}
