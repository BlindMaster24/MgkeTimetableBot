package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func cacheWithGroups(t *testing.T, b *Bot, names ...string) {
	t.Helper()

	day := utils.SiteNow().Format("02.01.2006")
	groups := make(map[string]any, len(names))
	for _, name := range names {
		groups[name] = map[string]any{"days": []any{
			map[string]any{"day": day, "lessons": []any{map[string]any{"lesson": "Математика"}}},
		}}
	}
	b.GetRaspCache().SetGroups(t.Context(), groups, "hash")
}

func TestSubscriptionAddStoresAGroupAndLeavesTheScene(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	cacheWithGroups(t, b, "100", "200")

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = sceneSubAddGroup
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}

	b.handleSubAddGroup(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "200"}, chat)

	if got := caller.last(); !strings.Contains(got, "200") {
		t.Fatalf("answer = %q, want the added group", got)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Fatalf("scene = %q, want cleared", saved.Scene)
	}
	subs, _ := b.chatRepo.GetSubscriptions(userID)
	if len(subs) != 1 || subs[0].Value != "200" || subs[0].Type != "group" {
		t.Fatalf("subscriptions = %+v, want group 200", subs)
	}
}

func TestSubscriptionAddReportsAnExistingPair(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	cacheWithGroups(t, b, "200")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubAddGroup
	_ = b.chatRepo.Save(chat)

	b.handleSubAddGroup(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "200"}, chat)
	b.handleSubAddGroup(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "200"}, chat)

	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_31")) {
		t.Fatalf("answer = %q, want the duplicate notice", got)
	}
}

func TestSubscriptionRemoveNeedsAValidNumber(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubRemove

	b.handleSubRemove(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "1"}, chat)
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_34")) {
		t.Fatalf("answer = %q, want the empty-list hint", got)
	}

	if _, err := b.chatRepo.AddSubscription(userID, "group", "100"); err != nil {
		t.Fatal(err)
	}
	_ = b.chatRepo.Save(chat)

	b.handleSubRemove(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "x"}, chat)
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_35")) {
		t.Fatalf("answer = %q, want the invalid-number hint", got)
	}

	b.handleSubRemove(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "1"}, chat)
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_36")) {
		t.Fatalf("answer = %q, want the removed notice", got)
	}
	if subs, _ := b.chatRepo.GetSubscriptions(userID); len(subs) != 0 {
		t.Fatalf("subscriptions = %+v, want none", subs)
	}
}

func TestSubscriptionAddTeacherFindsASingleMatch(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.GetRaspCache().SetTeachers(t.Context(), map[string]any{
		"Сидорова Мария Петровна": map[string]any{"days": []any{}},
	}, "hash")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubAddTeacher
	_ = b.chatRepo.Save(chat)

	b.handleSubAddTeacher(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Сидорова"}, chat)

	got := caller.last()
	if !strings.Contains(got, "Сидорова Мария Петровна") {
		t.Fatalf("answer = %q, want the matched teacher", got)
	}
	subs, _ := b.chatRepo.GetSubscriptions(userID)
	if len(subs) != 1 || subs[0].Type != "teacher" {
		t.Fatalf("subscriptions = %+v, want one teacher", subs)
	}
}

func TestSubscriptionSaveFailureIsReported(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	cacheWithGroups(t, b, "200")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubAddGroup
	_ = b.chatRepo.Save(chat)

	// Closing the database makes the insert fail the way a broken store would.
	if err := b.chatRepo.Close(); err != nil {
		t.Fatal(err)
	}

	b.handleSubAddGroup(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "200"}, chat)

	if got := caller.last(); !strings.Contains(got, b.loc("data_not_loaded")) {
		t.Fatalf("answer = %q, want the storage hint", got)
	}
}

func TestSetTeacherRefusesToGuess(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.GetRaspCache().SetTeachers(t.Context(), map[string]any{
		"Сидорова Мария Петровна": map[string]any{"days": []any{}},
		"Сидоров Пётр Иванович":   map[string]any{"days": []any{}},
	}, "hash")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Сидоров"}
	b.handleSetTeacher(t.Context(), u, chat)

	got := caller.last()
	if !strings.Contains(got, b.loc("cmpgrp_10")) {
		t.Fatalf("answer = %q, want the ambiguity hint", got)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeStudent || saved.Teacher != "" {
		t.Fatalf("the chat was changed to %+v, want the student mode kept", saved)
	}
}

func TestSetTeacherStillAcceptsAUniqueName(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.GetRaspCache().SetTeachers(t.Context(), map[string]any{
		"Сидорова Мария Петровна": map[string]any{"days": []any{}},
	}, "hash")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Сидорова"}
	b.handleSetTeacher(t.Context(), u, chat)

	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Mode != ModeTeacher || saved.Teacher != "Сидорова Мария Петровна" {
		t.Fatalf("chat = %+v, want the teacher mode", saved)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("about_bot")) {
		t.Fatalf("answer = %q, want the about text the scene answers with", got)
	}
}

func TestSubscriptionRemoveReportsAStorageFailure(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	if _, err := b.chatRepo.AddSubscription(userID, "group", "100"); err != nil {
		t.Fatal(err)
	}
	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubRemove
	_ = b.chatRepo.Save(chat)

	if err := b.chatRepo.Close(); err != nil {
		t.Fatal(err)
	}

	b.handleSubRemove(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "1"}, chat)
	if got := caller.last(); !strings.Contains(got, b.loc("data_not_loaded")) {
		t.Fatalf("answer = %q, want the storage hint", got)
	}
}
