package telegram

import (
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestHistoryCommandStartsTheTeacherScene(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.GetRaspCache().SetTeachers(t.Context(), map[string]any{
		"Сидорова Мария Петровна": map[string]any{"days": []any{}},
	}, "hash")

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/history"}
	if err := (&historyCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	prompt := strings.Split(b.loc("histtext_2"), "%s")[0]
	if got := caller.last(); !strings.Contains(got, prompt) {
		t.Fatalf("answer = %q, want the teacher prompt", got)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Scene != "history_teacher" {
		t.Fatalf("scene = %q, want history_teacher", saved.Scene)
	}
}

func TestHistoryCommandNeedsTeachersInTheCache(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	empty, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b.cache = empty

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/history"}
	if err := (&historyCmd{bot: b}).Handler(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("data_not_loaded")) {
		t.Fatalf("answer = %q, want the data hint", got)
	}
}

func TestHistoryTeacherSceneValidatesInput(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.GetRaspCache().SetTeachers(t.Context(), map[string]any{
		"Сидорова Мария Петровна": map[string]any{"days": []any{}},
	}, "hash")

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = "history_teacher"
	_ = b.chatRepo.Save(chat)
	scene := &historyTeacherScene{bot: b}

	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Си"}, chat); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("histtext_3")) {
		t.Fatalf("answer = %q, want the short-input hint", got)
	}

	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Некто Неизвестный"}, chat); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("histtext_4")) {
		t.Fatalf("answer = %q, want the not-found hint", got)
	}

	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "Сидорова Мария Петровна"}, chat); err != nil {
		t.Fatal(err)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneHistoryWeek {
		t.Fatalf("scene = %q, want %q", saved.Scene, sceneHistoryWeek)
	}
	if saved.Teacher != "Сидорова Мария Петровна" || saved.Mode != ModeTeacher {
		t.Fatalf("chat = %+v, want the teacher mode with the matched name", saved)
	}
}

func TestHistoryWeekSceneRejectsABadWeek(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneHistoryWeek
	chat.Teacher = "Сидорова Мария Петровна"
	_ = b.chatRepo.Save(chat)

	scene := &historyWeekScene{bot: b}
	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "abc"}, chat); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("history_invalid_week")) {
		t.Fatalf("answer = %q, want the week hint", got)
	}

	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "50"}, chat); err != nil {
		t.Fatal(err)
	}
	saved, _ := b.chatRepo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Fatalf("scene = %q, want cleared", saved.Scene)
	}
}

func TestHistoryWeekSceneShowsDaysFromTheArchive(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")
	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(utils.SiteNow()).Value())
	minIdx, _ := week.WeekDayIndexRange()
	seedArchiveDay(t, archiveRepo, "100", utils.DayIndexToDate(minIdx).Format("02.01.2006"))

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Scene = sceneHistoryWeek
	chat.Teacher = "Сидорова Мария Петровна"
	chat.Mode = ModeTeacher
	_ = b.chatRepo.Save(chat)

	scene := &historyWeekScene{bot: b}
	if err := scene.Handle(t.Context(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: week.String()}, chat); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); !strings.Contains(got, b.loc("histtext_7")) && !strings.Contains(got, "Сидорова") {
		t.Logf("answer = %q", got)
	}
}
