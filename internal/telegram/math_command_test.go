package telegram

import (
	"strings"
	"testing"
)

func mathUpdate(b *Bot, chatID int64, text string) *Update {
	return &Update{Bot: b, ChatID: chatID, UserID: chatID, Text: text}
}

func TestMathCommandAnswersWithTheResult(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	cases := map[string]string{
		"/math 2+2":     "4",
		"/math 2+3*4":   "14",
		"/math 5к":      "5000",
		"/math 1,5+1,5": "3",
		"/math 2×3":     "6",
		"/math 10:4":    "2.5",
		"/math (1+2)*3": "9",
	}
	for text, want := range cases {
		if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, text)); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if got := caller.last(); !strings.Contains(got, want) {
			t.Errorf("%q answered %q, want %q", text, got, want)
		}
	}
}

func TestMathCommandAsksForAnExpression(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, "/math")); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_21")) {
		t.Fatalf("answer = %q, want the expression hint", got)
	}
}

func TestMathCommandRejectsUnsupportedCharacters(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, "/math 2+абв")); err != nil {
		t.Fatal(err)
	}
	got := caller.last()
	if !strings.Contains(got, b.loc("cmdtext_22")) || !strings.Contains(got, "0123456789") {
		t.Fatalf("answer = %q, want the allowed characters", got)
	}
}

func TestMathCommandReportsBrokenExpressions(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, "/math (2+3")); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_24")) {
		t.Fatalf("answer = %q, want the invalid-expression hint", got)
	}

	if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, "/math (2+3)")); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, "5") {
		t.Fatalf("answer = %q, want 5", got)
	}
}

func TestMathCommandRefusesInfinity(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	if err := (&mathCmd{bot: b}).Handler(t.Context(), mathUpdate(b, 4242, "/math 2/0")); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("cmdtext_26")) {
		t.Fatalf("answer = %q, want the infinity message", got)
	}
}
