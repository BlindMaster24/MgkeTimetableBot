package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	telegoapi "github.com/mymmrac/telego/telegoapi"
)

// failingCaller rejects the sends addressed to one chat.
type broadcastFailingCaller struct {
	*capturingCaller
	failChatID int64
	edited     []string
}

func (c *broadcastFailingCaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	if strings.HasSuffix(url, "/sendMessage") {
		var payload struct {
			ChatID int64 `json:"chat_id"`
		}
		if err := json.Unmarshal(data.BodyRaw, &payload); err == nil && payload.ChatID == c.failChatID {
			return nil, errors.New("blocked")
		}
	}
	if strings.HasSuffix(url, "/editMessageText") {
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data.BodyRaw, &payload); err == nil {
			c.edited = append(c.edited, payload.Text)
		}
	}
	return c.capturingCaller.Call(ctx, url, data)
}

func TestBroadcastReportsSentAndFailedChats(t *testing.T) {
	caller := &broadcastFailingCaller{capturingCaller: &capturingCaller{}, failChatID: 2}
	b, _ := setupE2EBotWithCaller(t, caller)

	chats := []*Chat{
		{Service: "telegram", PeerID: 1},
		{Service: "telegram", PeerID: 2},
		{Service: "telegram", PeerID: 3},
	}

	cmd := &sendCmd{bot: b}
	b.broadcast.Lock()
	cmd.runBroadcast(900, 7, chats, "Привет")

	delivered := map[int64]string{}
	for _, message := range caller.messages {
		delivered[message.chatID] = message.text
	}
	if delivered[1] != "Привет" || delivered[3] != "Привет" {
		t.Fatalf("both reachable chats must get the text, got %+v", delivered)
	}

	if len(caller.edited) == 0 {
		t.Fatal("the progress message must be edited")
	}
	final := caller.edited[len(caller.edited)-1]
	if !strings.Contains(final, "Успешно отправлено!") {
		t.Fatalf("the summary must confirm the broadcast, got %q", final)
	}
	if !strings.Contains(final, "Ошибок: 1") {
		t.Fatalf("the summary must count the failed chat, got %q", final)
	}
}

func TestBroadcastUnlocksEvenWhenEverySendFails(t *testing.T) {
	caller := &broadcastFailingCaller{capturingCaller: &capturingCaller{}, failChatID: 1}
	b, _ := setupE2EBotWithCaller(t, caller)

	cmd := &sendCmd{bot: b}
	b.broadcast.Lock()
	cmd.runBroadcast(900, 7, []*Chat{{Service: "telegram", PeerID: 1}}, "Привет")

	if !b.broadcast.TryLock() {
		t.Fatal("the broadcast mutex must be released after the run")
	}
	b.broadcast.Unlock()

	timeout := make(chan struct{})
	go func() {
		b.broadcast.Lock()
		b.broadcast.Unlock()
		close(timeout)
	}()
	<-timeout
}
