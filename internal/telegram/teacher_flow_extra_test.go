package telegram

import (
	"context"
	"strings"
	"testing"
)

func TestFindTeacherWithKeyboardExplainsEveryOutcome(t *testing.T) {
	bot, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	bot.cache.SetTeachers(t.Context(), map[string]any{
		"Иванов И.И.": weekEntryMap("07.09.2026", map[string]any{"lesson": "Физика"}),
		"Иванов П.П.": weekEntryMap("07.09.2026", map[string]any{"lesson": "Химия"}),
	}, "h")

	u := &Update{UserID: userID, ChatID: userID, Bot: bot}
	chat := &Chat{PeerID: userID, Accepted: true}

	if name, ok, ambiguous := bot.findTeacherWithKeyboard(u, chat, "Ив", nil); name != "" || ok || ambiguous {
		t.Fatalf("a short input = %q %v %v, want a hint", name, ok, ambiguous)
	}
	if got := caller.last(); got != bot.loc("cmpgrp_7") {
		t.Fatalf("short input answered %q, want cmpgrp_7", got)
	}

	if name, ok, _ := bot.findTeacherWithKeyboard(u, chat, "Сидоров", nil); name != "" || ok {
		t.Fatalf("an unknown teacher = %q %v, want nothing", name, ok)
	}
	if got := caller.last(); got != bot.loc("cmpgrp_8") {
		t.Fatalf("an unknown teacher answered %q, want cmpgrp_8", got)
	}

	if name, ok, ambiguous := bot.findTeacherWithKeyboard(u, chat, "Иванов", nil); name != "" || ok || !ambiguous {
		t.Fatalf("two matches = %q %v %v, want an ambiguity prompt", name, ok, ambiguous)
	}
	if got := caller.last(); !strings.Contains(got, "Иванов И.И.") || !strings.Contains(got, "Иванов П.П.") {
		t.Fatalf("the ambiguity prompt = %q, want the candidates", got)
	}

	name, ok, ambiguous := bot.findTeacherWithKeyboard(u, chat, "Иванов И.И.", nil)
	if name != "Иванов И.И." || !ok || ambiguous {
		t.Fatalf("a full name = %q %v %v, want a single match", name, ok, ambiguous)
	}
}

func TestSendTeacherResultFallsBackToTheTextSchedule(t *testing.T) {
	bot, caller, _, userID := setupArchiveBot(t, ModeTeacher, "", "Иванов И.И.")
	chat, err := bot.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = "settings"

	u := &Update{UserID: userID, ChatID: userID, Bot: bot}
	if err := bot.sendTeacherResult(context.Background(), u, chat, "Иванов И.И.", "text"); err != nil {
		t.Fatalf("sendTeacherResult: %v", err)
	}
	if got := caller.last(); !strings.Contains(got, "Иванов И.И.") {
		t.Fatalf("the text schedule = %q, want the teacher name", got)
	}
	if chat.Scene != "" {
		t.Fatalf("the scene must be cleared, got %q", chat.Scene)
	}
}

func TestIntValueReadsBothNumberShapes(t *testing.T) {
	if got := intValue(3.0); got == nil || *got != 3 {
		t.Fatalf("intValue(3.0) = %v, want 3", got)
	}
	if got := intValue(4); got == nil || *got != 4 {
		t.Fatalf("intValue(4) = %v, want 4", got)
	}
	if got := intValue("4"); got != nil {
		t.Fatalf("intValue(string) = %v, want nil", got)
	}
	if got := intValue(nil); got != nil {
		t.Fatalf("intValue(nil) = %v, want nil", got)
	}
}

func weekEntryMap(day string, lesson map[string]any) map[string]any {
	return map[string]any{"days": []any{map[string]any{"day": day, "lessons": []any{lesson}}}}
}
