package telegram

import (
	"testing"

	"github.com/mymmrac/telego"
)

// TestEveryCallbackSurvivesGarbageData answers every registered callback with a
// payload it did not expect: a stale or hand-crafted button must end in a hint,
// never in a panic.
func TestEveryCallbackSurvivesGarbageData(t *testing.T) {
	const userID = int64(777)

	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, userID)
	b.GetRaspCache().Reset()

	for prefix, callback := range b.callbacks {
		t.Run(prefix, func(t *testing.T) {
			u := makeUpdate(userID, "")
			u.Bot = b
			u.Data = prefix + "garbage:1:x"
			u.Callback = &telego.CallbackQuery{ID: "cb-" + prefix, From: telego.User{ID: userID}}
			if err := callback.Handler(t.Context(), u); err != nil {
				t.Fatalf("%s returned %v", prefix, err)
			}
		})
	}
}
