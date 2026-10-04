package telegram

import (
	"sort"
	"sync"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

type botState struct {
	mu            sync.Mutex
	reports       map[string]parser.Report
	parseLogs     []parseLogEntry
	webhookStatus WebhookStatus
}

func (s *botState) recordReport(report parser.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reports == nil {
		s.reports = make(map[string]parser.Report)
	}
	s.reports[report.Source] = report
}

func (s *botState) reportsSnapshot() []parser.Report {
	s.mu.Lock()
	defer s.mu.Unlock()

	sources := make([]string, 0, len(s.reports))
	for source := range s.reports {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	reports := make([]parser.Report, 0, len(sources))
	for _, source := range sources {
		reports = append(reports, s.reports[source])
	}
	return reports
}

func (s *botState) appendParseLog(entry parseLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.parseLogs = append(s.parseLogs, entry)
	if len(s.parseLogs) > 50 {
		s.parseLogs = s.parseLogs[len(s.parseLogs)-50:]
	}
}

func (s *botState) parseLogsSnapshot() []parseLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]parseLogEntry(nil), s.parseLogs...)
}

func (s *botState) setWebhookStatus(status WebhookStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.webhookStatus = status
}

func (s *botState) getWebhookStatus() WebhookStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.webhookStatus
}
