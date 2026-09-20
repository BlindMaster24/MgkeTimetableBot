package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

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
	rasp.SetTeam(map[string]string{"Иванов И.И.": "Иванов Иван Иванович"}, []string{"hash"})

	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(time.Now()).Value())
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "teacher", "Иванов И.И.", start, map[string]any{"lesson": "Пара", "group": "100"})
	seedArchivedDay(t, archiveRepo, "teacher", "Петров П.П.", start, map[string]any{"lesson": "Пара", "group": "100"})

	caller.reset()
	runList(t, b, userID, "/teachers")

	text := caller.last()
	for _, want := range []string{
		"__ Преподаватели в кэше __",
		"1. Иванов Иван Иванович",
		"2. Петров П.П.",
		"Загружено:",
		"Изменено:",
		"__ Страницы с учителями/администрацией __",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the teacher listing is missing %q:\n%s", want, text)
		}
	}
}

func TestGroupsListingUsesTheArchiveAndFallsBackToTheCache(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(time.Now()).Value())
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", start, groupLesson("Пара"))

	caller.reset()
	runList(t, b, userID, "/groups")
	text := caller.last()

	if !strings.Contains(text, "__ Группы в кэше __") || !strings.Contains(text, "100") {
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
	if !strings.Contains(got, "__ Группы в кэше __") {
		t.Errorf("the cache fallback produced no groups: %s", got)
	}
}
