package telegram

import (
	"context"
	"testing"

	"github.com/mymmrac/telego"
)

var menuButtonExceptions = map[string]string{
	"Начать": "wireframe start button of the first-run reply keyboard; /start opens the setup scene and the button is asserted in the setup tests",
	"Отмена": "cancels the current scene; the cancel command has its own test",
}

type menuKeyboardScenario struct {
	name     string
	scene    string
	keyboard func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup
}

func menuKeyboards(b *Bot) []menuKeyboardScenario {
	scenarios := []menuKeyboardScenario{
		{name: "main", scene: "", keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return replyMainMenu(b, chat) }},
		{name: "setup", scene: sceneSetup, keyboard: func(b *Bot, _ *Chat) *telego.ReplyKeyboardMarkup { return b.replySelectMode() }},
		{name: "calls", scene: sceneSettingsCalls, keyboard: func(b *Bot, chat *Chat) *telego.ReplyKeyboardMarkup { return b.replyCallsSettings(chat, true) }},
	}
	for _, spec := range b.menuSpecs() {
		if spec.keyboard == nil {
			continue
		}
		scenarios = append(scenarios, menuKeyboardScenario{name: spec.id, scene: spec.scene, keyboard: spec.keyboard})
	}
	return scenarios
}

func menuTestChat() *Chat {
	return &Chat{
		Mode:          ModeStudent,
		Group:         "100",
		ShowDaily:     true,
		ShowWeekly:    true,
		ShowCalls:     true,
		ShowAbout:     true,
		ShowFastGroup: true,
		NoticeChanges: true,
		DiffEnabled:   true,
	}
}

func TestE2E_EveryMenuButtonHasAnAction(t *testing.T) {
	b, repo := setupE2EBot(t, 4242)

	scenarios := menuKeyboards(b)
	if len(scenarios) < 10 {
		t.Fatalf("expected the menu table to expose keyboards, got %d", len(scenarios))
	}

	seen := map[string]bool{}
	userID := int64(4242)
	checked := 0

	for _, scenario := range scenarios {
		sample := menuTestChat()
		sample.PeerID = userID
		keyboard := scenario.keyboard(b, sample)
		if keyboard == nil {
			t.Errorf("menu %q rendered no keyboard", scenario.name)
			continue
		}

		for _, row := range keyboard.Keyboard {
			for _, button := range row {
				label := button.Text
				if label == "" || seen[scenario.name+"|"+label] {
					continue
				}
				seen[scenario.name+"|"+label] = true
				checked++

				chat, err := repo.FindOrCreate("telegram", userID)
				if err != nil {
					t.Fatalf("find chat: %v", err)
				}
				chat.Mode = ModeStudent
				chat.Group = "100"
				chat.Scene = scenario.scene
				if err := repo.Save(chat); err != nil {
					t.Fatalf("save chat: %v", err)
				}

				u := makeUpdate(userID, label)
				u.Bot = b
				if b.dispatchTextCommand(context.Background(), u, chat) || b.dispatchInputScene(context.Background(), u, chat) {
					continue
				}
				if reason, ok := menuButtonExceptions[label]; ok {
					if reason == "" {
						t.Errorf("menu %q button %q is excepted without a reason", scenario.name, label)
					}
					continue
				}
				t.Errorf("menu %q button %q is not handled by any command in scene %q", scenario.name, label, scenario.scene)
			}
		}
	}

	if checked < 60 {
		t.Errorf("expected to press every menu button, only checked %d", checked)
	}
}
