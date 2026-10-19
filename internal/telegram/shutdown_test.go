package telegram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/mymmrac/telego"
)

func TestConsumeUpdatesDrainsUpdatesAndStopsOnCancelledContext(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan telego.Update)

	done := make(chan error, 1)
	go func() { done <- b.consumeUpdates(ctx, updates) }()

	updates <- telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:      "cb",
		From:    telego.User{ID: 7788},
		Data:    "cancel",
		Message: &telego.Message{Chat: telego.Chat{ID: 7788}, MessageID: 3},
	}}

	deadline := time.Now().Add(2 * time.Second)
	for !caller.delivered() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !caller.delivered() {
		t.Fatal("the update loop did not handle the update")
	}

	select {
	case err := <-done:
		t.Fatalf("the loop stopped before the context was cancelled: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("consumeUpdates returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumeUpdates ignored the cancelled context")
	}
}

func TestConsumeUpdatesLogsTheStopSignalWhenCancellationClosesTheChannel(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "bot.log")
	log := logger.New("info", &logger.FileConfig{Enabled: true, Path: logPath, MaxSizeMB: 1, MaxFiles: 1})
	t.Cleanup(func() { _ = log.Close() })

	b, _ := setupE2EBot(t)
	b.log = log

	const rounds = 20
	for i := 0; i < rounds; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		updates := make(chan telego.Update)

		cancel()
		close(updates)

		if err := b.consumeUpdates(ctx, updates); err != nil {
			t.Fatalf("consumeUpdates returned %v", err)
		}
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read the bot log: %v", err)
	}
	stopLines := strings.Count(string(data), "stop signal received, leaving the update loop")
	if stopLines != rounds {
		t.Errorf("the stop signal line appeared %d times after %d cancellations; the loop must log it whether the cancellation or the closed channel wins the select", stopLines, rounds)
	}
}

func TestConsumeUpdatesStopsOnClosedChannel(t *testing.T) {
	b, _ := setupE2EBot(t)

	updates := make(chan telego.Update)
	close(updates)

	done := make(chan error, 1)
	go func() { done <- b.consumeUpdates(context.Background(), updates) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("consumeUpdates returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumeUpdates ignored the closed update channel")
	}
}
