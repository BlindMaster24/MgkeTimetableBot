package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

var nextTestNow = time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC)

func nextTestCalls(t *testing.T, b *Bot) {
	t.Helper()
	b.cache.SetCalls(context.Background(), cache.Schedule{}, cache.Schedule{Weekdays: [][2][2]string{
		{{"10:50", "10:50"}, {"12:30", "12:30"}},
		{{"12:40", "12:40"}, {"14:20", "14:20"}},
	}}, "manual")
}

func nextTestDay(t *testing.T, b *Bot, group string, lessons []any) {
	t.Helper()
	b.cache.SetGroups(context.Background(), map[string]any{
		group: e2eJsonRoundTrip(map[string]any{"group": group, "days": []any{
			map[string]any{"day": nextTestNow.Format("02.01.2006"), "lessons": lessons},
		}}),
	}, "next-test")
}

func nextTestBot(t *testing.T) (*Bot, *recordingCaller) {
	t.Helper()
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetNow(func() time.Time { return nextTestNow })
	return b, caller
}

func sendNext(t *testing.T, b *Bot, caller *recordingCaller, userID int64) string {
	t.Helper()
	caller.reset()
	cmd := &nextLessonCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/next"}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	text, _ := lastDeliveredMessage(t, caller)
	return text
}

func TestNextMatchText(t *testing.T) {
	b, _ := nextTestBot(t)
	cmd := &nextLessonCmd{bot: b}

	for _, text := range []string{"/next", "/NEXT", " /next "} {
		if !cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = false, want true", text)
		}
	}
	for _, text := range []string{"/nextt", "next", "/next 3", "/day"} {
		if cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = true, want false", text)
		}
	}
}

func TestNextShowsTheRunningAndTheFollowingPair(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3101)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "901"
	b.chatRepo.Save(chat)

	nextTestCalls(t, b)
	nextTestDay(t, b, "901", []any{
		map[string]any{"lesson": "Математика", "cabinet": "101"},
		map[string]any{"lesson": "Физика", "cabinet": "202"},
	})

	text := sendNext(t, b, caller, userID)
	for _, want := range []string{"Сейчас", "Математика", "101", "12:30", "Дальше", "Физика", "12:40", "202"} {
		if !strings.Contains(text, want) {
			t.Errorf("answer must contain %q, got %q", want, text)
		}
	}
}

func TestNextReportsOnlyTheUpcomingPairBeforeItStarts(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3102)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "902"
	b.chatRepo.Save(chat)

	b.SetNow(func() time.Time { return nextTestNow.Add(-time.Hour) })
	nextTestCalls(t, b)
	nextTestDay(t, b, "902", []any{map[string]any{"lesson": "Химия", "cabinet": "303"}})

	text := sendNext(t, b, caller, userID)
	if strings.Contains(text, "Сейчас") {
		t.Errorf("a pair that has not started must not be reported as running, got %q", text)
	}
	if !strings.Contains(text, "Дальше") || !strings.Contains(text, "Химия") {
		t.Errorf("answer must name the upcoming pair, got %q", text)
	}
}

func TestNextTeacherUsesTheTeacherTimetable(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3103)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeTeacher
	chat.Teacher = "Иванов И.И."
	b.chatRepo.Save(chat)

	nextTestCalls(t, b)
	b.cache.SetTeachers(context.Background(), map[string]any{
		"Иванов И.И.": e2eJsonRoundTrip(map[string]any{"teacher": "Иванов И.И.", "days": []any{
			map[string]any{"day": nextTestNow.Format("02.01.2006"), "lessons": []any{
				map[string]any{"lesson": "География", "group": "901", "cabinet": "404"},
			}},
		}}),
	}, "next-teachers")

	text := sendNext(t, b, caller, userID)
	if !strings.Contains(text, "География") || !strings.Contains(text, "404") {
		t.Errorf("teacher answer must name the lesson and the room, got %q", text)
	}
}

func TestNextAsksForATargetWhenNoneIsChosen(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3104)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	b.chatRepo.Save(chat)

	nextTestCalls(t, b)
	if text := sendNext(t, b, caller, userID); !strings.Contains(text, "/setGroup") {
		t.Errorf("a chat without a group must be told how to pick one, got %q", text)
	}

	guest, _ := b.chatRepo.FindOrCreate("telegram", int64(3105))
	guest.Mode = ""
	b.chatRepo.Save(guest)
	if text := sendNext(t, b, caller, int64(3105)); text != b.loc("next_no_target") {
		t.Errorf("a guest must be told to pick a target, got %q", text)
	}
}

func TestNextAnswersWhenTodayHasNoTimetable(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3106)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "903"
	b.chatRepo.Save(chat)

	nextTestCalls(t, b)
	b.cache.SetGroups(context.Background(), map[string]any{
		"903": e2eJsonRoundTrip(map[string]any{"group": "903", "days": []any{
			map[string]any{"day": nextTestNow.AddDate(0, 0, 1).Format("02.01.2006"), "lessons": []any{
				map[string]any{"lesson": "Математика", "cabinet": "101"},
			}},
		}}),
	}, "next-tomorrow")

	if text := sendNext(t, b, caller, userID); text != b.loc("next_no_data") {
		t.Errorf("a day without a timetable must be answered, got %q", text)
	}
}

func TestNextAnswersWhenTheDayHasNoLessons(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3107)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "904"
	b.chatRepo.Save(chat)

	nextTestCalls(t, b)
	nextTestDay(t, b, "904", []any{})

	if text := sendNext(t, b, caller, userID); text != b.loc("next_none") {
		t.Errorf("a day without lessons must be answered, got %q", text)
	}
}

func TestNextHandlesAMissingBellSchedule(t *testing.T) {
	b, caller := nextTestBot(t)
	const userID = int64(3108)

	chat, _ := b.chatRepo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "905"
	b.chatRepo.Save(chat)

	nextTestDay(t, b, "905", []any{map[string]any{"lesson": "Математика", "cabinet": "101"}})

	if text := sendNext(t, b, caller, userID); text != b.loc("next_none") {
		t.Errorf("a missing bell schedule must be answered, got %q", text)
	}
}
