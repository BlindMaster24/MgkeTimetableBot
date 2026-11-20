package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func icsChat(t *testing.T, b *Bot, userID int64, mode ChatMode, group, teacher string) *Chat {
	t.Helper()

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = mode
	chat.Group = group
	chat.Teacher = teacher
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}
	return chat
}

func seedArchiveDay(t *testing.T, repo *archive.Repository, group, date string) {
	t.Helper()

	if err := repo.AppendDays(t.Context(), []archive.AppendDay{{
		Type:  "group",
		Value: group,
		Day: map[string]any{"day": date, "lessons": []any{
			map[string]any{"lesson": "Математика", "cabinet": "101"},
		}},
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestICSUploadNeedsTheFeatureTurnedOn(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = false

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/ics"}
	if err := (&icsCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("icstext_3")) {
		t.Fatalf("answer = %q, want the disabled hint", got)
	}
}

func TestICSUploadAsksForAGroupWhenMissing(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "", "")
	b.cfg.Calendar.ICS.Enabled = true
	icsChat(t, b, userID, ModeStudent, "", "")

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/ics"}
	if err := (&icsCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("icstext_4")) {
		t.Fatalf("answer = %q, want the group hint", got)
	}
}

func TestICSUploadSendsTheCalendarDocument(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = true
	icsChat(t, b, userID, ModeStudent, "100", "")

	week := b.relevantWeekIndex()
	minIdx, _ := week.WeekDayIndexRange()
	seedArchiveDay(t, archiveRepo, "100", utils.DayIndexToDate(minIdx).Format("02.01.2006"))

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/ics"}
	if err := (&icsCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	doc := caller.documentPayload()
	if !strings.Contains(doc, "BEGIN:VCALENDAR") || !strings.Contains(doc, "Математика") {
		t.Fatalf("document = %q, want a calendar with the lesson", doc)
	}
}

func TestICSUploadExplainsAnEmptyArchive(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = true
	icsChat(t, b, userID, ModeStudent, "100", "")

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/ics"}
	if err := (&icsCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("icstext_8")) {
		t.Fatalf("answer = %q, want the empty-archive hint", got)
	}
}

func TestICSHandlerNeedsAKnownMode(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = true
	icsChat(t, b, userID, "", "", "")

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/ics"}
	if err := (&icsCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("icstext_6")) {
		t.Fatalf("answer = %q, want the mode hint", got)
	}
}
