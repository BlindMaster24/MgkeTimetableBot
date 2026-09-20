package telegram

import (
	"strconv"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func imageCallbackUpdate(bot *Bot, userID int64) *Update {
	u := makeUpdate(userID, "")
	u.Bot = bot
	return u
}

func weeklyArchiveDays(t *testing.T, repo *archive.Repository) string {
	t.Helper()

	week := utils.WeekIndexFromDate(time.Now())
	minIdx, _ := week.WeekDayIndexRange()
	day := utils.DayIndexToDate(minIdx).Format("02.01.2006")

	if err := repo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: map[string]any{"day": day, "lessons": []any{
			map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101"},
		}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: map[string]any{"day": day, "lessons": []any{
			map[string]any{"lesson": "Математика", "type": "Лек", "group": "100", "cabinet": "101"},
		}}},
	}); err != nil {
		t.Fatal(err)
	}

	return strconv.Itoa(week.Value())
}

func TestImageCallbackRendersTheRequestedWeek(t *testing.T) {
	const userID = int64(9401)
	t.Chdir(t.TempDir())

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller)
	repo := setupArchive(t)
	b.archive = repo

	week := weeklyArchiveDays(t, repo)

	u := imageCallbackUpdate(b, userID)
	if err := b.handleImagePayload(u, "g", "100:"+week); err != nil {
		t.Fatal(err)
	}
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("a group week with archive days must render an image, got %q", caller.last())
	}

	caller.reset()
	if err := b.handleImagePayload(u, "t", "Иванов И.И.:"+week); err != nil {
		t.Fatal(err)
	}
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("a teacher week with archive days must render an image, got %q", caller.last())
	}
}

func TestImageCallbackKeepsItsFallbacks(t *testing.T) {
	const userID = int64(9402)
	t.Chdir(t.TempDir())

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller)
	repo := setupArchive(t)
	b.archive = repo

	week := weeklyArchiveDays(t, repo)

	cases := []struct {
		name       string
		typeLetter string
		value      string
		want       string
	}{
		{"unknown letter", "x", "100:" + week, b.loc("no_timetable")},
		{"group without cache entry", "g", "999:" + week, b.loc("group_not_exists")},
		{"teacher without cache entry", "t", "Петров П.П.:" + week, b.loc("teacher_not_exists")},
		{"week without archive days", "g", "100:1", "Нет расписания для отображения"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller.reset()
			u := imageCallbackUpdate(b, userID)
			if err := b.handleImagePayload(u, tc.typeLetter, tc.value); err != nil {
				t.Fatal(err)
			}
			if caller.calledMethod("sendPhoto") {
				t.Fatal("the fallback must not render an image")
			}
			if got := caller.last(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestImageCallbackWithoutAnArchiveFallsBackToTheCache(t *testing.T) {
	const userID = int64(9403)
	t.Chdir(t.TempDir())

	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller)
	if b.archive != nil {
		t.Fatal("the harness must start without an archive")
	}

	week := b.relevantWeekIndex().Value()
	u := imageCallbackUpdate(b, userID)
	if err := b.handleImagePayload(u, "g", "100:"+strconv.Itoa(week)); err != nil {
		t.Fatal(err)
	}
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("without an archive the cached days must be used, got %q", caller.last())
	}
}
