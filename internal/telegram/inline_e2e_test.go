package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func chatSnapshot(t *testing.T, chat *Chat) string {
	t.Helper()
	data, err := json.Marshal(chat)
	if err != nil {
		t.Fatalf("marshal chat: %v", err)
	}
	return string(data)
}

func inlineScenarios(t *testing.T, b *Bot, repo *Repository) []CallbackLayout {
	t.Helper()

	layouts := SurfaceCallbackLayouts()
	if len(layouts) == 0 {
		t.Fatal("no inline keyboards found")
	}

	aliases := []Alias{{Key: "Математика", Value: "Матем"}}
	layouts = append(layouts, CallbackLayout{
		Builder: "aliasRemoveKeyboard",
		Rows:    callbackRows(aliasRemoveKeyboard(aliases)),
	})
	return layouts
}

func callbackRows(kb *telego.InlineKeyboardMarkup) [][]string {
	if kb == nil {
		return nil
	}
	var rows [][]string
	for _, row := range kb.InlineKeyboard {
		var data []string
		for _, button := range row {
			if button.CallbackData != "" {
				data = append(data, button.CallbackData)
			}
		}
		if len(data) > 0 {
			rows = append(rows, data)
		}
	}
	return rows
}

func inlineTestChat() *Chat {
	return &Chat{
		Mode:          ModeStudent,
		Group:         "100",
		ShowDaily:     true,
		ShowWeekly:    true,
		ShowCalls:     true,
		AllowSendMess: true,
		Accepted:      true,
	}
}

func TestE2E_EveryInlineButtonReachesAHandler(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)

	seen := map[string]bool{}
	pressed := 0

	for _, layout := range inlineScenarios(t, b, repo) {
		for _, row := range layout.Rows {
			for _, data := range row {
				if seen[data] {
					continue
				}
				seen[data] = true
				pressed++

				prefix, handler := b.findCallback(data)
				if handler == nil {
					t.Errorf("%s: callback %q has no handler (registered prefixes: %v)", layout.Builder, data, callbackPrefixes(b))
					continue
				}
				if !strings.HasPrefix(data, prefix) {
					t.Errorf("%s: callback %q matched prefix %q that is not its prefix", layout.Builder, data, prefix)
				}

				chat, err := repo.FindOrCreate("telegram", 4242)
				if err != nil {
					t.Fatalf("find chat: %v", err)
				}
				configured := inlineTestChat()
				configured.ID = chat.ID
				configured.Service = chat.Service
				configured.PeerID = chat.PeerID
				configured.LastMsgID = 7
				if err := repo.Save(configured); err != nil {
					t.Fatalf("save chat: %v", err)
				}
				before := chatSnapshot(t, configured)

				caller.reset()
				u := &Update{
					Bot:    b,
					ChatID: 4242,
					UserID: 4242,
					Data:   data,
					Callback: &telego.CallbackQuery{
						ID:      "cb",
						From:    telego.User{ID: 4242},
						Message: &telego.Message{Chat: telego.Chat{ID: 4242}, MessageID: 7},
					},
				}
				if err := handler.Handler(context.Background(), u); err != nil {
					t.Errorf("%s: callback %q returned error: %v", layout.Builder, data, err)
					continue
				}

				after, err := repo.FindOrCreate("telegram", 4242)
				if err != nil {
					t.Fatalf("reload chat: %v", err)
				}
				if chatSnapshot(t, after) == before && !caller.delivered() {
					t.Errorf("%s: callback %q neither changed the chat nor delivered anything", layout.Builder, data)
				}
			}
		}
	}

	if pressed < 15 {
		t.Errorf("expected to press every inline button, only pressed %d", pressed)
	}
}

func TestE2E_EveryRegisteredCallbackIsReachable(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)

	produced := map[string]bool{}
	for _, layout := range inlineScenarios(t, b, repo) {
		for _, row := range layout.Rows {
			for _, data := range row {
				prefix, _ := b.findCallback(data)
				if prefix != "" {
					produced[prefix] = true
				}
			}
		}
	}

	for prefix := range b.callbacks {
		if !produced[prefix] {
			t.Errorf("callback %q is registered but no keyboard produces its payload", prefix)
		}
	}
}

func callbackPrefixes(b *Bot) []string {
	prefixes := make([]string, 0, len(b.callbacks))
	for prefix := range b.callbacks {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
