package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func (c *recordingCaller) methods() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	names := make([]string, 0, len(c.calls))
	for _, call := range c.calls {
		names = append(names, call.Method)
	}
	return names
}

func (c *recordingCaller) calledMethod(name string) bool {
	for _, method := range c.methods() {
		if method == name {
			return true
		}
	}
	return false
}

func callbackQuery(userID int64, messageID int, data string) *telego.CallbackQuery {
	return &telego.CallbackQuery{
		ID:   "callback-1",
		From: telego.User{ID: userID},
		Data: data,
		Message: &telego.Message{
			MessageID: messageID,
			Chat:      telego.Chat{ID: userID, Type: "private"},
		},
	}
}

func TestCallbackFromAnUnacceptedChatIsBlocked(t *testing.T) {
	const userID = int64(9301)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	repo.SetDefaultAccepted(false)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Accepted {
		t.Fatal("the fixture needs a chat without access")
	}
	chat.Mode = ModeStudent
	chat.Group = "100"
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	b.handleCallback(context.Background(), callbackQuery(userID, 7, "cancel"))

	if caller.calledMethod("answerCallbackQuery") {
		t.Fatal("the callback must not be answered before the access gate")
	}
	if text := caller.deliveredText(); !strings.Contains(text, "Ключ, который нужно предоставить") {
		t.Fatalf("the chat must be told how to ask for access, got %q", text)
	}
	if strings.Contains(caller.deliveredText(), "Ввод был отменён") {
		t.Fatal("the callback handler must not run for a chat without access")
	}

	after, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Accepted {
		t.Fatal("a blocked callback must not grant access")
	}
}

func TestCallbackFromAnAcceptedChatRuns(t *testing.T) {
	const userID = int64(9302)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	repo.SetDefaultAccepted(true)

	if _, err := repo.FindOrCreate("telegram", userID); err != nil {
		t.Fatal(err)
	}

	b.handleCallback(context.Background(), callbackQuery(userID, 8, "cancel"))

	if !caller.calledMethod("answerCallbackQuery") {
		t.Fatal("an accepted chat must get its callback answered")
	}
	if !strings.Contains(caller.deliveredText(), "Ввод был отменён") {
		t.Fatalf("the handler must run for an accepted chat, got %q", caller.deliveredText())
	}
}

func TestCallbackIsBlockedWhenTheChatCannotBeLoaded(t *testing.T) {
	const userID = int64(9303)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	repo.SetDefaultAccepted(true)

	if _, err := repo.FindOrCreate("telegram", userID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	b.handleCallback(context.Background(), callbackQuery(userID, 9, "cancel"))

	if len(caller.methods()) != 0 {
		t.Fatalf("a callback with an unreadable chat must stop at the gate, got %v", caller.methods())
	}
}

func TestInaccessibleCallbackStillReachesTheGate(t *testing.T) {
	const userID = int64(9304)

	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	repo.SetDefaultAccepted(false)

	if _, err := repo.FindOrCreate("telegram", userID); err != nil {
		t.Fatal(err)
	}

	query := callbackQuery(userID, 10, "cancel")
	query.Message = &telego.InaccessibleMessage{
		Chat:      telego.Chat{ID: userID, Type: "private"},
		MessageID: 10,
	}

	b.handleCallback(context.Background(), query)

	texts := caller.textsFor(userID)
	if len(texts) != 1 {
		t.Fatalf("the gate must answer the right chat, got %q", texts)
	}
	if !strings.Contains(texts[0], "Ключ, который нужно предоставить") {
		t.Fatalf("unexpected answer: %q", texts[0])
	}
}
