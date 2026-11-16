package telegram

import (
	"context"
	"strings"
	"testing"
)

func TestFormatCommandSwitchesTheChatFormat(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	cmd := &textFormatCmd{bot: b}

	u := makeUpdate(userID, "/format html")
	u.Bot = b
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if chat.TextFormat != FormatHTML {
		t.Fatalf("chat format = %q, want %q", chat.TextFormat, FormatHTML)
	}
	if !strings.Contains(caller.last(), "HTML") {
		t.Fatalf("the answer must name the new format, got %q", caller.last())
	}

	u = makeUpdate(userID, "/format md")
	u.Bot = b
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	chat, err = b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if chat.TextFormat != FormatMarkdown {
		t.Fatalf("chat format = %q, want %q", chat.TextFormat, FormatMarkdown)
	}
}

func TestFormatCommandShowsTheCurrentFormat(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	u := makeUpdate(userID, "/format")
	u.Bot = b
	if err := (&textFormatCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(caller.last(), "Markdown") {
		t.Fatalf("a fresh chat must be Markdown by default, got %q", caller.last())
	}
}

func TestFormatCommandRejectsAnUnknownFormat(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	u := makeUpdate(userID, "/format pdf")
	u.Bot = b
	if err := (&textFormatCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); got != b.loc("format_unknown") {
		t.Fatalf("got %q, want the format_unknown text", got)
	}
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if chat.TextFormat != "" {
		t.Fatalf("an unknown format must not be stored, got %q", chat.TextFormat)
	}
}
