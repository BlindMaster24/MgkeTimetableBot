package telegram

import (
	"context"
	"strings"
	"testing"
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
