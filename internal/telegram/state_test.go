package telegram

import (
	"sync"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func TestRegistryShimKeepsHandlersWorking(t *testing.T) {
	b, _ := setupE2EBot(t)

	before := len(b.commands)
	b.RegisterCommand(&pingCmd{bot: b})
	if len(b.commands) != before {
		t.Fatalf("commands = %d, want still %d (ping is registered already)", len(b.commands), before)
	}
	if b.commandByName("/ping") == nil {
		t.Fatal("registry must resolve /ping by name")
	}
	if prefix, cb := b.findCallback("answer:test"); cb == nil || prefix != "answer:" {
		t.Fatalf("registry must resolve answer callback, got %q %v", prefix, cb)
	}
	if len(b.botCommands(false)) == 0 {
		t.Fatal("registry must list bot commands")
	}
}

func TestBotStateSurvivesConcurrentUse(t *testing.T) {
	b, _ := setupE2EBot(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.RecordParserReport(parser.Report{Source: parser.SourceGroups, Items: i})
			b.AddParseLog(true, "ok")
			_ = b.ParserReports()
			_ = b.GetParseLogs()
			b.setWebhookStatus(WebhookStatus{CheckedAt: int64(i)})
			_ = b.getWebhookStatus()
		}(i)
	}
	wg.Wait()

	if len(b.GetParseLogs()) != 8 {
		t.Fatalf("parse logs = %d, want 8", len(b.GetParseLogs()))
	}
}
