package telegram

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/mymmrac/telego/telegoapi"
)

type failingCaller struct{}

func (failingCaller) Call(context.Context, string, *telegoapi.RequestData) (*telegoapi.Response, error) {
	return nil, errors.New("telegram is down")
}

func TestBotCountsCommandsAndCallbacks(t *testing.T) {
	const userID = int64(9401)

	b, repo := setupE2EBot(t)
	repo.SetDefaultAccepted(true)
	counters := health.NewTelegramCounters()
	b.SetCounterSink(counters)

	if _, err := repo.FindOrCreate("telegram", userID); err != nil {
		t.Fatal(err)
	}

	chat := &Chat{Mode: ModeStudent, Group: "100", Accepted: true, ShowDaily: true}
	if !b.dispatchTextCommand(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/day"}, chat) {
		t.Fatal("the /day command must be dispatched")
	}
	if !b.dispatchTextCommand(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/DAY"}, chat) {
		t.Fatal("an upper-case command must be dispatched")
	}

	b.handleCallback(context.Background(), callbackQuery(userID, 7, "cancel"))
	b.handleCallback(context.Background(), callbackQuery(userID, 8, "unknown:payload\n\"evil\""))

	stats := counters.TelegramStats()
	if !maps.Equal(stats.Commands, map[string]int64{"day": 2}) {
		t.Errorf("commands = %v, want {day: 2}", stats.Commands)
	}
	if !maps.Equal(stats.Callbacks, map[string]int64{"cancel": 1}) {
		t.Errorf("callbacks = %v, want {cancel: 1}", stats.Callbacks)
	}
	if len(stats.Notifications) != 0 || len(stats.Transports) != 0 {
		t.Errorf("the bot must only feed its own dictionaries: %+v", stats)
	}
}

func TestBotCountsFailedTransports(t *testing.T) {
	b, _ := setupE2EBotWithCaller(t, failingCaller{})
	counters := health.NewTelegramCounters()
	b.SetCounterSink(counters)

	if err := b.SendText(1, "hello"); err == nil {
		t.Fatal("the failing caller must surface an error")
	}
	if err := b.SendTextWithReplyKeyboard(1, "hello", nil); err == nil {
		t.Fatal("the failing caller must surface a reply keyboard error")
	}
	if _, err := b.sendMessageForID(1, "progress"); err == nil {
		t.Fatal("the failing caller must surface a progress error")
	}
	if err := b.SendGoogleLinked(1, "user@example.com"); err == nil {
		t.Fatal("the failing caller must surface a google link error")
	}
	if err := b.SendDocument(1, "timetable.txt", []byte("data"), ""); err == nil {
		t.Fatal("the failing caller must surface a document error")
	}
	if err := b.EditMessageText(1, 5, "text", nil); err == nil {
		t.Fatal("the failing caller must surface an edit error")
	}

	stats := counters.TelegramStats()
	want := map[string]int64{"send_message": 4, "send_document": 1, "edit_message": 1}
	if !maps.Equal(stats.Transports, want) {
		t.Errorf("transports = %v, want %v", stats.Transports, want)
	}
	if len(stats.Commands) != 0 || len(stats.Callbacks) != 0 {
		t.Errorf("a transport failure must not count as traffic: %+v", stats)
	}
}

func TestBotIgnoresSuccessfulSends(t *testing.T) {
	b, _ := setupE2EBot(t)
	counters := health.NewTelegramCounters()
	b.SetCounterSink(counters)

	if err := b.SendText(1, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := b.SendTextWithKeyboard(1, "hello", nil); err != nil {
		t.Fatal(err)
	}

	if stats := counters.TelegramStats(); stats.Transports != nil {
		t.Errorf("a delivered message is not a transport error: %v", stats.Transports)
	}
}
