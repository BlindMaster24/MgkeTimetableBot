package telegram

import (
	"strings"
	"testing"
)

func TestSendRefusesATextTelegramWouldReject(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Telegram.AdminIDs = []int64{userID}

	long := strings.Repeat("я", telegramTextLimit+1)
	u := makeUpdate(userID, "/send "+long)
	u.Bot = b
	if err := (&sendCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if got := caller.last(); !strings.Contains(got, b.loc("admintext_too_long")) {
		t.Fatalf("answer = %q, want the length hint", got)
	}
	if !b.broadcast.TryLock() {
		t.Fatal("the broadcast lock was taken for a rejected text")
	}
	b.broadcast.Unlock()
}
