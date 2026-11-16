package telegram

import (
	"context"
	"errors"
	"testing"
)

type answeringCallback struct {
	prefix  string
	err     error
	handled bool
}

func (c *answeringCallback) Prefix() string { return c.prefix }
func (c *answeringCallback) Handler(ctx context.Context, u *Update) error {
	c.handled = true
	return c.err
}

func TestCallbackWithoutAnAnswerStopsTheSpinner(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.RegisterCallback(&answeringCallback{prefix: "silent"})

	b.handleCallback(context.Background(), callbackQuery(userID, 7, "silent:1"))

	if got := caller.answerCount(); got != 1 {
		t.Fatalf("the dispatcher must answer once, got %d answers", got)
	}
	if got := caller.lastAnswer(); got != "" {
		t.Fatalf("the fallback answer must be empty, got %q", got)
	}
}

func TestCallbackErrorIsAnswered(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.RegisterCallback(&answeringCallback{prefix: "broken", err: errors.New("boom")})

	b.handleCallback(context.Background(), callbackQuery(userID, 7, "broken:1"))

	if got := caller.lastAnswer(); got != b.loc("callback_error") {
		t.Fatalf("answer = %q, want the callback_error text", got)
	}
}

func TestUnknownCallbackIsAnswered(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	b.handleCallback(context.Background(), callbackQuery(userID, 7, "nope:1"))

	if got := caller.lastAnswer(); got != b.loc("callback_not_found") {
		t.Fatalf("answer = %q, want the callback_not_found text", got)
	}
}

func TestCommandErrorReachesTheUser(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.RegisterCommand(&failingCommand{})

	u := makeUpdate(userID, "/fail")
	u.Bot = b
	if !b.dispatchTextCommand(context.Background(), u, chatOf(t, b, userID)) {
		t.Fatal("the failing command must be dispatched")
	}
	if got := caller.last(); got != b.loc("handler_error") {
		t.Fatalf("the user must hear about the failure, got %q", got)
	}
}

type failingCommand struct{}

func (c *failingCommand) Name() string        { return "/fail" }
func (c *failingCommand) Description() string { return "test" }
func (c *failingCommand) Handler(ctx context.Context, u *Update) error {
	return errors.New("boom")
}

func chatOf(t *testing.T, b *Bot, userID int64) *Chat {
	t.Helper()
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	return chat
}

func TestAMessageRefreshesTheLastMessageTime(t *testing.T) {
	b, _, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	u := makeUpdate(userID, "/start")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if chat.LastMsgTime == 0 {
		t.Fatal("an accepted message must update the last message time")
	}
}
