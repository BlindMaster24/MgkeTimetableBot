package telegram

import (
	"context"
	"testing"
	"time"

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
