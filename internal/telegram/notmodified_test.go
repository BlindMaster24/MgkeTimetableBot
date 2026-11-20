package telegram

import (
	"errors"
	"testing"
)

func TestMessageNotModifiedMatchesOnlyTheBotAPIAnswer(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"exact", errors.New("message is not modified"), true},
		{"wrapped", errors.New("telego: editMessageText: api: 400 \"Bad Request: message is not modified\""), true},
		{"other api error", errors.New("api: 400 \"Bad Request: chat not found\""), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := messageNotModified(c.err); got != c.want {
			t.Errorf("%s: messageNotModified(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

func TestAnUnchangedEditIsNotASendFailure(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	caller.editNotModified = true

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.LastMsgID = 77
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}

	u := &Update{Bot: b, ChatID: userID, UserID: userID, MessageID: 41}
	if err := b.sendScheduleText(u, chat, "Группа 100", nil); err != nil {
		t.Fatalf("an unchanged edit must not fail: %v", err)
	}
	if len(caller.messages) != 0 {
		t.Fatalf("the bot sent %d new messages, want none", len(caller.messages))
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.LastMsgID != 77 {
		t.Fatalf("last message id = %d, want the edited one to stay", saved.LastMsgID)
	}
}
