package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

func freeTestDay(lesson string, room string) map[string]any {
	return map[string]any{"lesson": lesson, "cabinet": room}
}

func freeTestGroups(t *testing.T, b *Bot, groups map[string][]any) {
	t.Helper()
	entries := map[string]any{}
	for name, lessons := range groups {
		entries[name] = e2eJsonRoundTrip(map[string]any{"group": name, "days": []any{
			map[string]any{"day": nextTestNow.Format("02.01.2006"), "lessons": lessons},
		}})
	}
	b.cache.SetGroups(context.Background(), entries, "free-test")
}

func freeTestTeachers(t *testing.T, b *Bot, teachers map[string][]any) {
	t.Helper()
	entries := map[string]any{}
	for name, lessons := range teachers {
		entries[name] = e2eJsonRoundTrip(map[string]any{"teacher": name, "days": []any{
			map[string]any{"day": nextTestNow.Format("02.01.2006"), "lessons": lessons},
		}})
	}
	b.cache.SetTeachers(context.Background(), entries, "free-teachers")
}

func sendFree(t *testing.T, b *Bot, caller *recordingCaller, text string) string {
	t.Helper()
	caller.reset()
	cmd := &freeRoomsCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: text}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	answer, _ := lastDeliveredMessage(t, caller)
	return answer
}

func TestFreeMatchText(t *testing.T) {
	b, _ := nextTestBot(t)
	cmd := &freeRoomsCmd{bot: b}

	for _, text := range []string{"/free", "/FREE", " /free ", "/free 3"} {
		if !cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = false, want true", text)
		}
	}
	for _, text := range []string{"/freet", "free", "/day", "/free3"} {
		if cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = true, want false", text)
		}
	}
}

func TestFreeReportsEveryRoomBusyAtTheRunningPair(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{
		"901": {freeTestDay("Математика", "101"), freeTestDay("Физика", "202")},
		"902": {freeTestDay("Информатика", "202")},
	})

	want := b.locData("free_busy", map[string]any{"Num": 1, "Start": "10:50", "End": "12:30"})
	if got := sendFree(t, b, caller, "/free"); got != want {
		t.Errorf("answer = %q, want %q", got, want)
	}
}

func TestFreeAcceptsAnExplicitPair(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{
		"901": {freeTestDay("Математика", "101"), freeTestDay("Физика", "202")},
		"902": {freeTestDay("Информатика", "202")},
	})

	want := b.locData("free_header", map[string]any{"Num": 2, "Start": "12:40", "End": "14:20", "Rooms": "101"})
	if got := sendFree(t, b, caller, "/free 2"); got != want {
		t.Errorf("answer = %q, want %q", got, want)
	}
}

func TestFreeCountsRoomsFromTeacherPages(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{
		"901": {freeTestDay("Математика", "101"), freeTestDay("Физика", "202")},
	})
	freeTestTeachers(t, b, map[string][]any{
		"Иванов И.И.": {freeTestDay("Химия", "303")},
	})

	want := b.locData("free_header", map[string]any{"Num": 2, "Start": "12:40", "End": "14:20", "Rooms": "101, 303"})
	if got := sendFree(t, b, caller, "/free 2"); got != want {
		t.Errorf("answer = %q, want %q", got, want)
	}
}

func TestFreeMergesTheSameRoomFromDifferentSources(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{
		"901": {freeTestDay("Математика", "101"), freeTestDay("Физика", "3-212")},
	})
	freeTestTeachers(t, b, map[string][]any{
		"Петров П.П.": {freeTestDay("Химия", "3-212 (к)")},
	})

	want := b.locData("free_header", map[string]any{"Num": 2, "Start": "12:40", "End": "14:20", "Rooms": "101"})
	if got := sendFree(t, b, caller, "/free 2"); got != want {
		t.Errorf("a room marked with a decoration must not be counted twice: got %q, want %q", got, want)
	}

	answer := sendFree(t, b, caller, "/free 1")
	if strings.Contains(answer, "3-212") {
		t.Errorf("a room used by a teacher page must not look free, got %q", answer)
	}
}

func TestFreeRejectsAPairOutsideTheBellSchedule(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{"901": {freeTestDay("Математика", "101")}})

	for _, text := range []string{"/free 9", "/free 0", "/free abc"} {
		if got := sendFree(t, b, caller, text); got != b.loc("free_no_pair") {
			t.Errorf("answer for %q = %q, want the missing pair message", text, got)
		}
	}
}

func TestFreeReportsTheEndOfTheDay(t *testing.T) {
	b, caller := nextTestBot(t)
	b.SetNow(func() time.Time { return nextTestNow.Add(4 * time.Hour) })
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{"901": {freeTestDay("Математика", "101")}})

	if got := sendFree(t, b, caller, "/free"); got != b.loc("free_no_pairs") {
		t.Errorf("answer = %q, want the end-of-day message", got)
	}
}

func TestFreeAnswersWithoutATimetableForToday(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)

	if got := sendFree(t, b, caller, "/free"); got != b.loc("free_no_data") {
		t.Errorf("answer = %q, want the missing timetable message", got)
	}
}

func TestFreeFallsBackToTheConfiguredBellSchedule(t *testing.T) {
	b, caller := nextTestBot(t)
	freeTestGroups(t, b, map[string][]any{"901": {freeTestDay("Математика", "101")}})

	want := b.locData("free_header", map[string]any{"Num": 2, "Start": "09:50", "End": "11:30", "Rooms": "101"})
	if got := sendFree(t, b, caller, "/free"); got != want {
		t.Errorf("answer = %q, want %q", got, want)
	}
}

func TestFreeKeepsTheLaterPairsWithoutRoomsEmpty(t *testing.T) {
	b, caller := nextTestBot(t)
	nextTestCalls(t, b)
	freeTestGroups(t, b, map[string][]any{"901": {freeTestDay("Математика", "101")}})

	want := b.locData("free_header", map[string]any{"Num": 2, "Start": "12:40", "End": "14:20", "Rooms": "101"})
	if got := sendFree(t, b, caller, "/free 2"); got != want {
		t.Errorf("answer = %q, want %q", got, want)
	}
}

func TestFreePairArgumentReadsThePairNumber(t *testing.T) {
	cases := map[string]struct {
		num       int
		requested bool
	}{
		"/free":     {0, false},
		"/free 3":   {3, true},
		"/free 12":  {12, true},
		"/free abc": {0, true},
		"/free -1":  {-1, true},
	}
	for text, want := range cases {
		num, requested := freePairArgument(text)
		if num != want.num || requested != want.requested {
			t.Errorf("freePairArgument(%q) = %d %v, want %d %v", text, num, requested, want.num, want.requested)
		}
	}
}
