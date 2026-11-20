package telegram

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func TestAliasAddRejectsInputWithoutBothSides(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = sceneSettingsAlias

	u := makeUpdate(userID, "без разделителя")
	b.handleAliasAdd(context.Background(), u, chat)
	if got := caller.last(); got != b.loc("aliastext_12") {
		t.Fatalf("missing '=' must be explained by aliastext_12, got %q", got)
	}

	u = makeUpdate(userID, " = значение")
	b.handleAliasAdd(context.Background(), u, chat)
	if got := caller.last(); got != b.loc("aliastext_13") {
		t.Fatalf("an empty key must be explained by aliastext_13, got %q", got)
	}
}

func TestAliasAddStoresThePairAndLeavesTheScene(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = sceneSettingsAlias

	u := makeUpdate(userID, "мат = Математика")
	b.handleAliasAdd(context.Background(), u, chat)

	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 1 || aliases[0].Key != "мат" || aliases[0].Value != "Математика" {
		t.Fatalf("aliases = %+v", aliases)
	}

	if got := caller.last(); !strings.Contains(got, "мат") || !strings.Contains(got, "Математика") {
		t.Fatalf("the confirmation must name the pair, got %q", got)
	}

	stored, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if stored.Scene != "" {
		t.Fatalf("the scene must be cleared, got %q", stored.Scene)
	}
}

func TestAliasAddEscapesTheAnglesInTheAnswer(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = sceneSettingsAlias

	b.handleAliasAdd(context.Background(), makeUpdate(userID, "мат = <b>Математика</b> & физика"), chat)

	answer := caller.last()
	if strings.Contains(answer, "<b>") || strings.Contains(answer, "& физика") {
		t.Fatalf("the answer must carry escaped text, got %q", answer)
	}
	if !strings.Contains(answer, "&lt;b&gt;") || !strings.Contains(answer, "&amp;") {
		t.Fatalf("the answer must escape the markup, got %q", answer)
	}
}

func TestAliasListEscapesTheStoredPairs(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.aliasRepo.Add(userID, "мат<", "Математика&Физика"); err != nil {
		t.Fatal(err)
	}
	listUpdate := makeUpdate(userID, "/alias")
	listUpdate.Bot = b
	if err := b.showAliasList(listUpdate, userID); err != nil {
		t.Fatal(err)
	}
	_ = chat

	list := caller.last()
	if strings.Contains(list, "мат<") || strings.Contains(list, "&Физика") {
		t.Fatalf("the list must escape the pairs, got %q", list)
	}
	if !strings.Contains(list, "мат&lt;") || !strings.Contains(list, "Математика&amp;Физика") {
		t.Fatalf("the list must keep the escaped pairs, got %q", list)
	}
}

func TestSendGroupResultFallsBackToTheTextSchedule(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}

	u := makeUpdate(userID, "100")
	u.Bot = b
	if err := b.sendGroupResult(context.Background(), u, chat, "100", ""); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, "100") {
		t.Fatalf("the group schedule must reach the chat, got %q", got)
	}
	if chat.Scene != "" {
		t.Fatalf("the scene must be cleared, got %q", chat.Scene)
	}
}

func TestAliasRemoveKeyboardKeepsThePayloadShort(t *testing.T) {
	b, _, _, _ := setupArchiveBot(t, ModeStudent, "100", "")
	long := strings.Repeat("Длинный-алиас-", 6)
	aliases := []Alias{{ID: 7, Key: long, Value: long}}

	for _, row := range aliasRemoveKeyboard(b, aliases).InlineKeyboard {
		for _, button := range row {
			if len(button.CallbackData) > 64 {
				t.Fatalf("callback data %q is %d bytes, the Bot API allows 64", button.CallbackData, len(button.CallbackData))
			}
			if strings.HasPrefix(button.CallbackData, "alias:del:") && button.CallbackData != "alias:del:7" {
				t.Fatalf("callback data = %q, want the row id", button.CallbackData)
			}
		}
	}
}

func TestAliasRemovalUsesTheRowID(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	if err := b.aliasRepo.Add(userID, "мат", "Математика"); err != nil {
		t.Fatal(err)
	}
	list, err := b.aliasRepo.List(userID)
	if err != nil || len(list) != 1 {
		t.Fatalf("aliases = %v, %v", list, err)
	}

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Data: fmt.Sprintf("alias:del:%d", list[0].ID), Callback: &telego.CallbackQuery{ID: "cb-1"}}
	if err := (&aliasDelCb{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	rest, err := b.aliasRepo.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 0 {
		t.Fatalf("aliases after removal = %v, want none", rest)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("aliastext_9")) {
		t.Fatalf("answer = %q, want the empty list hint", got)
	}
}
