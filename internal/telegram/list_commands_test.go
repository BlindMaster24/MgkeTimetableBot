package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func runList(t *testing.T, b *Bot, userID int64, text string) string {
	t.Helper()

	u := makeUpdate(userID, text)
	u.Bot = b

	var err error
	if text == "/teachers" {
		err = (&getTeachersCmd{bot: b}).Handler(context.Background(), u)
	} else {
		err = (&getGroupsCmd{bot: b}).Handler(context.Background(), u)
	}
	if err != nil {
		t.Fatal(err)
	}

	return ""
}

func TestTeachersListingShowsFullNamesAndPageTimestamps(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	rasp := b.GetRaspCache()
	rasp.SetTeam(t.Context(), map[string]string{"Иванов И.И.": "Иванов Иван Иванович"}, []string{"hash"})

	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(utils.SiteNow()).Value())
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "teacher", "Иванов И.И.", start, map[string]any{"lesson": "Пара", "group": "100"})
	seedArchivedDay(t, archiveRepo, "teacher", "Петров П.П.", start, map[string]any{"lesson": "Пара", "group": "100"})

	caller.reset()
	runList(t, b, userID, "/teachers")

	text := caller.last()
	for _, want := range []string{
		"<b>Преподаватели в кэше</b>",
		"1. Иванов Иван Иванович",
		"2. Петров П.П.",
		"Загружено:",
		"Изменено:",
		"<b>Страницы с учителями/администрацией</b>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the teacher listing is missing %q:\n%s", want, text)
		}
	}
}

func TestGroupsListingUsesTheArchiveAndFallsBackToTheCache(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(utils.SiteNow()).Value())
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", start, groupLesson("Пара"))

	caller.reset()
	runList(t, b, userID, "/groups")
	text := caller.last()

	if !strings.Contains(text, "<b>Группы в кэше</b>") || !strings.Contains(text, "100") {
		t.Errorf("the archived groups are missing:\n%s", text)
	}
	if !strings.Contains(text, "Загружено:") || !strings.Contains(text, "Изменено:") {
		t.Errorf("the cache timestamps are missing:\n%s", text)
	}

	emptyCaller := &capturingCaller{}
	withoutArchive, repo := setupE2EBotWithCaller(t, emptyCaller)
	writeChat(t, repo, userID, ModeStudent, "100", "")

	emptyCaller.reset()
	runList(t, withoutArchive, userID, "/groups")

	got := emptyCaller.last()
	if !strings.Contains(got, "<b>Группы в кэше</b>") {
		t.Errorf("the cache fallback produced no groups: %s", got)
	}
}
