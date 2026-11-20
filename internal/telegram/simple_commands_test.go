package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
)

// runCommand calls a command handler with the bot attached, the same way the
// dispatcher does.
func runCommand(t *testing.T, b *Bot, caller *capturingCaller, handler func(context.Context, *Update) error, userID int64, text string) string {
	t.Helper()

	u := makeUpdate(userID, text)
	u.Bot = b
	if err := handler(t.Context(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return caller.last()
}

func TestCancelClearsTheScene(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSubAddGroup
	_ = b.chatRepo.Save(chat)

	answer := runCommand(t, b, caller, (&cancelCmd{bot: b}).Handler, userID, "/cancel")

	if !strings.Contains(answer, b.loc("input_cancelled")) {
		t.Fatalf("answer = %q, want the cancel hint", answer)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Fatalf("scene = %q, want it cleared", saved.Scene)
	}
}

func TestEULAAndButtonsReloadAnswerTheirTexts(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	if got := runCommand(t, b, caller, (&eulaCmd{bot: b}).Handler, userID, "/eula"); !strings.Contains(got, b.loc("eula_text")) {
		t.Fatalf("eula answer = %q", got)
	}
	if got := runCommand(t, b, caller, (&buttonsReloadCmd{bot: b}).Handler, userID, "/buttons_reload"); !strings.Contains(got, b.loc("cmdtext_7")) {
		t.Fatalf("reload answer = %q", got)
	}
}

func TestResetCacheClearsTheTimetable(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	if got := runCommand(t, b, caller, (&resetCacheCmd{bot: b}).Handler, userID, "/resetcache"); !strings.Contains(got, b.loc("reset_cache_done")) {
		t.Fatalf("answer = %q", got)
	}
	if groups := b.cache.GetGroups(); len(groups) != 0 {
		t.Fatalf("groups = %v, want the cache cleared", groups)
	}
}

func TestAPIKeyErrorsNameTheReason(t *testing.T) {
	b, _, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	if got := b.apiKeyError(apikey.ErrShortSecret); got != b.loc("api_disabled") {
		t.Fatalf("short secret answer = %q, want api_disabled", got)
	}
	if got := b.apiKeyError(apikey.ErrKeyNotFound); got != b.loc("api_error") {
		t.Fatalf("other error answer = %q, want api_error", got)
	}
}

func TestRegexpListsEveryCommand(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	answer := runCommand(t, b, caller, (&regexpCmd{bot: b}).Handler, userID, "/regexp")
	if !strings.Contains(answer, "/start") || !strings.Contains(answer, "/alias") {
		t.Fatalf("answer = %q, want the command names", answer)
	}
	lines := strings.Split(answer, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i-1] > lines[i] {
			t.Fatalf("commands are not sorted: %q before %q", lines[i-1], lines[i])
		}
	}
}

func TestVanishAnswersTheVacuumResult(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	answer := runCommand(t, b, caller, (&vanishCmd{bot: b}).Handler, userID, "/vanish")
	if !strings.Contains(answer, b.loc("admcmd_4")) {
		t.Fatalf("answer = %q, want the vacuum confirmation", answer)
	}
}
