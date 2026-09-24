package telegram

import (
	"strconv"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func imageCallbackUpdate(bot *Bot, userID int64, messageID int, data string) *Update {
	u := makeUpdate(userID, "")
	u.Bot = bot
	u.Data = data
	u.MessageID = messageID
	u.Callback = callbackQuery(userID, messageID, data)
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

	u := imageCallbackUpdate(b, userID, 55, "image_g:100:"+week)
	b.handleCallback(t.Context(), u.Callback)
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("a group week with archive days must render an image, got %q", caller.last())
	}
	if toast := caller.toasts(); len(toast) != 1 || toast[0] != "Изображение было отправлено" {
		t.Fatalf("the callback must answer the render toast, got %v", toast)
	}
	if replied := caller.photoReplyTo(); replied != 55 {
		t.Fatalf("the image must be sent as a reply to the button message, got %d", replied)
	}

	caller.reset()
	u = imageCallbackUpdate(b, userID, 56, "image_t:Иванов И.И.:"+week)
	b.handleCallback(t.Context(), u.Callback)
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
		name  string
		data  string
		toast string
	}{
		{"unknown letter", "image_x:100:" + week, b.loc("image_failed")},
		{"group without cache entry", "image_g:999:" + week, b.loc("group_not_exists")},
		{"teacher without cache entry", "image_t:Петров П.П.:" + week, b.loc("teacher_not_exists")},
		{"week without archive days", "image_g:100:1", b.loc("no_timetable")},
		{"payload without a value", "image", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller.reset()
			u := imageCallbackUpdate(b, userID, 60, tc.data)
			b.handleCallback(t.Context(), u.Callback)
			if caller.calledMethod("sendPhoto") {
				t.Fatal("the fallback must not render an image")
			}
			got := ""
			if toasts := caller.toasts(); len(toasts) > 0 {
				got = toasts[0]
			}
			if got != tc.toast {
				t.Fatalf("got toast %q, want %q", got, tc.toast)
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
	u := imageCallbackUpdate(b, userID, 57, "image_g:100:"+strconv.Itoa(week))
	b.handleCallback(t.Context(), u.Callback)
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("without an archive the cached days must be used, got %q", caller.last())
	}
}
