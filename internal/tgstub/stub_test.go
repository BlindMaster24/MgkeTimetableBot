package tgstub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mymmrac/telego"
)

const (
	stubToken   = "123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	stubChatID  = int64(4242)
	stubWebhook = "https://bot.example.test/telegram/webhook"
)

func startStub(t *testing.T) (*Stub, *httptest.Server) {
	t.Helper()

	stub := New()
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	return stub, server
}

func recordedCalls(t *testing.T, server *httptest.Server) []Call {
	t.Helper()

	response, err := http.Get(server.URL + callsPath)
	if err != nil {
		t.Fatalf("read the recorded calls: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("calls endpoint answered %d", response.StatusCode)
	}

	var payload struct {
		Calls []Call `json:"calls"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode the recorded calls: %v", err)
	}

	return payload.Calls
}

func callsOf(t *testing.T, server *httptest.Server, method string) []Call {
	t.Helper()

	var found []Call
	for _, call := range recordedCalls(t, server) {
		if call.Method == method {
			found = append(found, call)
		}
	}

	return found
}

func TestStubServesTheBotAPIAOverHTTP(t *testing.T) {
	stub, server := startStub(t)

	client, err := telego.NewBot(stubToken, telego.WithAPIServer(server.URL), telego.WithDiscardLogger())
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}

	ctx := context.Background()
	if err := client.SetWebhook(ctx, &telego.SetWebhookParams{URL: stubWebhook, SecretToken: "s3cret"}); err != nil {
		t.Fatalf("setWebhook: %v", err)
	}

	info, err := client.GetWebhookInfo(ctx)
	if err != nil {
		t.Fatalf("getWebhookInfo: %v", err)
	}
	if info.URL != stubWebhook {
		t.Errorf("getWebhookInfo url = %q, want %q", info.URL, stubWebhook)
	}

	sent, err := client.SendMessage(ctx, &telego.SendMessageParams{ChatID: telego.ChatID{ID: stubChatID}, Text: "ci"})
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	if sent.Chat.ID != stubChatID {
		t.Errorf("reply chat id = %d, want %d", sent.Chat.ID, stubChatID)
	}
	if sent.Text != "ci" {
		t.Errorf("reply text = %q, want %q", sent.Text, "ci")
	}

	if stub.WebhookURL() != stubWebhook {
		t.Errorf("registered webhook = %q, want %q", stub.WebhookURL(), stubWebhook)
	}

	setWebhooks := callsOf(t, server, "setWebhook")
	if len(setWebhooks) != 1 {
		t.Fatalf("recorded %d setWebhook calls, want 1", len(setWebhooks))
	}
	if setWebhooks[0].Token != stubToken {
		t.Errorf("recorded token = %q, want %q", setWebhooks[0].Token, stubToken)
	}
	if setWebhooks[0].Body["secret_token"] != "s3cret" {
		t.Errorf("recorded secret token = %v", setWebhooks[0].Body["secret_token"])
	}
	if setWebhooks[0].At.IsZero() {
		t.Error("a recorded call must carry the time it arrived")
	}

	if len(callsOf(t, server, "sendMessage")) != 1 {
		t.Error("the stub must record the sendMessage call the bot made")
	}
	if len(callsOf(t, server, "getWebhookInfo")) != 1 {
		t.Error("the stub must record the getWebhookInfo call the bot made")
	}
}

func TestStubEchoesTheEditedMessageIDAndAnswersCallbacks(t *testing.T) {
	_, server := startStub(t)

	client, err := telego.NewBot(stubToken, telego.WithAPIServer(server.URL), telego.WithDiscardLogger())
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}

	ctx := context.Background()
	edit := &telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: stubChatID},
		MessageID: 909090,
		Text:      "calls",
	}
	edited, err := client.EditMessageText(ctx, edit)
	if err != nil {
		t.Fatalf("editMessageText: %v", err)
	}
	if edited.MessageID != edit.MessageID {
		t.Errorf("reply message id = %d, want the edited id %d", edited.MessageID, edit.MessageID)
	}
	if edited.Chat.ID != stubChatID {
		t.Errorf("reply chat id = %d, want %d", edited.Chat.ID, stubChatID)
	}

	if err := client.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: "ci-callback-1"}); err != nil {
		t.Fatalf("answerCallbackQuery: %v", err)
	}

	edits := callsOf(t, server, "editMessageText")
	if len(edits) != 1 {
		t.Fatalf("recorded %d editMessageText calls, want 1", len(edits))
	}
	if edits[0].Body["message_id"] != float64(909090) {
		t.Errorf("recorded message_id = %v, want the id of the edited message", edits[0].Body["message_id"])
	}

	answers := callsOf(t, server, "answerCallbackQuery")
	if len(answers) != 1 {
		t.Fatalf("recorded %d answerCallbackQuery calls, want 1", len(answers))
	}
	if answers[0].Body["callback_query_id"] != "ci-callback-1" {
		t.Errorf("recorded callback_query_id = %v", answers[0].Body["callback_query_id"])
	}

	sent, err := client.SendMessage(ctx, &telego.SendMessageParams{ChatID: telego.ChatID{ID: stubChatID}, Text: "ci"})
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	if sent.MessageID != 1 {
		t.Errorf("a fresh message id = %d, want the default 1", sent.MessageID)
	}
}

func TestStubForgetsTheWebhookWhenItIsDeleted(t *testing.T) {
	stub, server := startStub(t)

	client, err := telego.NewBot(stubToken, telego.WithAPIServer(server.URL), telego.WithDiscardLogger())
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}

	ctx := context.Background()
	if err := client.SetWebhook(ctx, &telego.SetWebhookParams{URL: stubWebhook}); err != nil {
		t.Fatalf("setWebhook: %v", err)
	}
	if err := client.DeleteWebhook(ctx, &telego.DeleteWebhookParams{}); err != nil {
		t.Fatalf("deleteWebhook: %v", err)
	}

	if stub.WebhookURL() != "" {
		t.Errorf("webhook = %q after deleteWebhook, want it cleared", stub.WebhookURL())
	}

	info, err := client.GetWebhookInfo(ctx)
	if err != nil {
		t.Fatalf("getWebhookInfo: %v", err)
	}
	if info.URL != "" {
		t.Errorf("getWebhookInfo url = %q after deleteWebhook, want it empty", info.URL)
	}
}

func TestStubAnswersHealthAndRejectsUnknownPaths(t *testing.T) {
	_, server := startStub(t)

	response, err := http.Get(server.URL + healthPath)
	if err != nil {
		t.Fatalf("health endpoint: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("health answered %d, want 200", response.StatusCode)
	}

	var health struct {
		OK    bool `json:"ok"`
		Calls int  `json:"calls"`
	}
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatalf("decode the health payload: %v", err)
	}
	if !health.OK {
		t.Error("health must report ok so a CI wait loop can poll it")
	}

	for _, path := range []string{"/nonsense", "/bot" + stubToken, "/bot" + stubToken + "/"} {
		unknown, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		unknown.Body.Close()

		if unknown.StatusCode != http.StatusNotFound {
			t.Errorf("%s answered %d, want 404", path, unknown.StatusCode)
		}
	}
}
