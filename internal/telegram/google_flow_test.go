package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
	"github.com/mymmrac/telego"
)

type fakeGoogle struct {
	configured  bool
	syncEnabled bool
	authURL     string
	user        googleUserAPI
	userErr     error
}

func (f *fakeGoogle) Configured() bool { return f.configured }

func (f *fakeGoogle) AuthURL(state string) string { return f.authURL + "?state=" + state }

func (f *fakeGoogle) Exchange(context.Context, string) (google.Credentials, string, error) {
	return google.Credentials{}, "", errors.New("not used in this test")
}

func (f *fakeGoogle) UserClient(context.Context, google.Credentials, func(google.Credentials)) (googleUserAPI, error) {
	if f.userErr != nil {
		return nil, f.userErr
	}
	return f.user, nil
}

func (f *fakeGoogle) SyncDay(context.Context, string, string, []google.DayLesson, google.Schedule) error {
	return nil
}

func (f *fakeGoogle) SyncEnabled() bool { return f.syncEnabled }

func googleCallback(b *Bot, userID int64, data string) *telego.CallbackQuery {
	return &telego.CallbackQuery{ID: "cb-1", Data: data, From: telego.User{ID: userID}}
}

func TestGoogleCommandExplainsAMissingIntegration(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	cmd := &googleCalendarCmd{bot: b}
	if err := cmd.Handler(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/google_calendar"}); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("gcal_2")) {
		t.Fatalf("answer = %q, want the not-configured hint", got)
	}

	b.google = &fakeGoogle{configured: false}
	if err := cmd.Handler(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/google_calendar"}); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("gcal_2")) {
		t.Fatalf("answer = %q, want the not-configured hint", got)
	}
}

func TestGoogleMenuAsksToLinkTheAccountFirst(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	fake := &fakeGoogle{configured: true, authURL: "https://accounts.example/auth"}
	b.google = fake

	b.handleCallback(t.Context(), googleCallback(b, userID, "gcal:menu"))

	got := caller.last()
	if !strings.Contains(got, b.loc("gcal_5")) {
		t.Fatalf("answer = %q, want the sign-in hint", got)
	}
}

func TestGoogleStateRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	state := EncodeGoogleState("secret", "telegram", 7712, now)
	service, peer, err := DecodeGoogleState("secret", state, now)
	if err != nil {
		t.Fatalf("state %q does not decode: %v", state, err)
	}
	if service != "telegram" || peer != 7712 {
		t.Fatalf("state = %q/%d, want telegram/7712", service, peer)
	}

	if _, _, err := DecodeGoogleState("secret", "not base64", now); err == nil {
		t.Fatal("broken base64 must be rejected")
	}
	if _, _, err := DecodeGoogleState("secret", EncodeGoogleState("secret", "", 0, now), now); err == nil {
		t.Fatal("an empty state must be rejected")
	}
}

func TestGoogleMenuShowsTheLinkedEmail(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.google = &fakeGoogle{configured: true, authURL: "https://accounts.example/auth"}

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.GoogleEmail = "user@example.com"
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}

	b.handleCallback(t.Context(), googleCallback(b, userID, "gcal:menu"))
	if got := caller.last(); !strings.Contains(got, "user@example.com") {
		t.Fatalf("answer = %q, want the linked email", got)
	}
}

func TestGoogleAddWithoutAGroupAnswersWithAHint(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "", "")
	b.google = &fakeGoogle{configured: true, authURL: "https://accounts.example/auth"}

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.GoogleEmail = "user@example.com"
	_ = b.chatRepo.Save(chat)

	b.handleCallback(t.Context(), googleCallback(b, userID, "gcal:add"))
	if got := caller.lastAnswer(); !strings.Contains(got, b.loc("gcal_36")) {
		t.Fatalf("answer = %q, want the missing-group hint", got)
	}
}

func TestGoogleDropWithoutAnIDFallsBackToTheList(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.google = &fakeGoogle{configured: true, authURL: "https://accounts.example/auth", userErr: errors.New("not linked")}

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.GoogleEmail = "user@example.com"
	_ = b.chatRepo.Save(chat)

	b.handleCallback(t.Context(), googleCallback(b, userID, "gcal:drop"))
	if got := caller.last(); !strings.Contains(got, b.loc("gcal_5")) {
		t.Fatalf("answer = %q, want the fallback listing", got)
	}
}
