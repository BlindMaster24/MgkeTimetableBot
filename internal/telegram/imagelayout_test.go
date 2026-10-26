package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/image"
)

const imageLayoutAdminID = int64(6161)

func imageLayoutTestBot(t *testing.T) (*Bot, *recordingCaller) {
	t.Helper()

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, imageLayoutAdminID)
	return b, caller
}

func sendImageLayout(t *testing.T, b *Bot, caller *recordingCaller, userID int64) string {
	t.Helper()

	caller.reset()
	cmd := &imageLayoutCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/imagelayout"}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	text, _ := lastDeliveredMessage(t, caller)
	return text
}

func TestImageLayoutMatchText(t *testing.T) {
	b, _ := imageLayoutTestBot(t)
	cmd := &imageLayoutCmd{bot: b}

	for _, text := range []string{"/imagelayout", "/IMAGELAYOUT", " /imagelayout "} {
		if !cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = false, want true", text)
		}
	}
	for _, text := range []string{"/image", "/image_layout", "imagelayout", "/imagelayouts", "/day"} {
		if cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = true, want false", text)
		}
	}
}

func TestImageLayoutCommandIsAdminOnly(t *testing.T) {
	b, _ := imageLayoutTestBot(t)
	cmd := &imageLayoutCmd{bot: b}

	if !cmd.AdminOnly() {
		t.Error("/imagelayout must be an admin command")
	}
	if cmd.Description() == "" || cmd.Description()[0] == '[' {
		t.Errorf("the command must carry a locale description, got %q", cmd.Description())
	}
}

func TestImageLayoutRefusesANonAdmin(t *testing.T) {
	b, caller := imageLayoutTestBot(t)

	const strangerID = int64(7171)
	if text := sendImageLayout(t, b, caller, strangerID); !strings.Contains(text, "Доступ запрещён") {
		t.Errorf("answer = %q, want the access refusal", text)
	}
}

func TestImageLayoutReportsTheCommittedLayout(t *testing.T) {
	b, caller := imageLayoutTestBot(t)

	text := sendImageLayout(t, b, caller, imageLayoutAdminID)
	for _, want := range []string{
		b.loc("image_layout_header"),
		b.loc("image_layout_sample_group"),
		b.loc("image_layout_sample_teacher"),
		b.loc("image_layout_ok"),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("answer %q misses %q", text, want)
		}
	}
	if strings.Contains(text, b.loc("image_layout_drift")) {
		t.Errorf("the committed layout must not report drift: %q", text)
	}
	if strings.Contains(text, "[image_layout_") {
		t.Errorf("every layout string must be localized, got %q", text)
	}
}

func TestImageLayoutReportsDriftAndAMovedLayout(t *testing.T) {
	b, _ := imageLayoutTestBot(t)

	checks := []image.GoldenLayout{
		{Name: "group", Width: 1280, Height: 724, WantWidth: 1280, WantHeight: 724, Differing: 12},
		{Name: "teacher", Width: 1002, Height: 556, WantWidth: 1000, WantHeight: 556, Differing: -1},
	}
	lines := b.imageLayoutLines("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", checks, nil)

	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		b.loc("image_layout_sample_group") + ": 1280×724, расходятся 12 px",
		"1002×556 вместо 1000×556",
		b.loc("image_layout_drift"),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("answer %q misses %q", text, want)
		}
	}
	if strings.Contains(text, b.loc("image_layout_ok")) {
		t.Errorf("a drifted layout must not report success: %q", text)
	}
}

func TestImageLayoutSurfacesARenderFailure(t *testing.T) {
	b, _ := imageLayoutTestBot(t)

	lines := b.imageLayoutLines("arial.ttf", nil, errors.New("no suitable font found on this system"))

	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "no suitable font found on this system") {
		t.Errorf("answer must name the failure, got %q", text)
	}
	if !strings.Contains(text, b.loc("image_layout_drift")) {
		t.Errorf("a failed render must not read as healthy, got %q", text)
	}
	if strings.Contains(text, b.loc("image_layout_ok")) {
		t.Errorf("a failed render must not report success: %q", text)
	}
}

func TestImageLayoutSaysWhenTheFontIsMissing(t *testing.T) {
	b, _ := imageLayoutTestBot(t)

	checks := []image.GoldenLayout{
		{Name: "group", Width: 10, Height: 10, WantWidth: 10, WantHeight: 10},
	}
	text := strings.Join(b.imageLayoutLines("", checks, nil), "\n")

	if !strings.Contains(text, b.loc("image_layout_font_missing")) {
		t.Errorf("answer must report the missing font, got %q", text)
	}
	if !strings.Contains(text, b.loc("image_layout_no_font")) {
		t.Errorf("answer must warn that the PNG cannot render, got %q", text)
	}
	if strings.Contains(text, b.loc("image_layout_ok")) {
		t.Errorf("a missing font must not report success: %q", text)
	}
}
