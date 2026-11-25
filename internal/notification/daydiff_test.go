package notification

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/schedulediff"
	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func TestChangeSummaryDoesNotLeaveDanglingCabinet(t *testing.T) {
	n := &EventNotifier{}
	ev := &cache.DayEvent{Diff: &schedulediff.Result{Changes: []schedulediff.Change{
		{Kind: schedulediff.RoomChanged,
			Old: timetable.Lesson{Subject: "Тест ПО", Room: "Кн.1-107"},
			New: timetable.Lesson{Subject: "Тест ПО"}},
		{Kind: schedulediff.TeacherChanged,
			Old: timetable.Lesson{Subject: "ПСС internet", Teacher: "Иванов И. И."},
			New: timetable.Lesson{Subject: "ПСС internet"}},
	}}}

	got := n.changeSummary(ev)
	if strings.Contains(got, "кабинет \n") || strings.HasSuffix(strings.TrimSpace(got), "кабинет") {
		t.Fatalf("a cleared room left a dangling word: %q", got)
	}
	if !strings.Contains(got, "Тест ПО: кабинет убран") {
		t.Fatalf("a cleared room must say so: %q", got)
	}
	if !strings.Contains(got, "ПСС internet: преподаватель убран") {
		t.Fatalf("a cleared teacher must say so: %q", got)
	}
}

func TestChangeSummaryKeepsANewRoom(t *testing.T) {
	n := &EventNotifier{}
	ev := &cache.DayEvent{Diff: &schedulediff.Result{Changes: []schedulediff.Change{
		{Kind: schedulediff.RoomChanged,
			Old: timetable.Lesson{Subject: "Комп сети"},
			New: timetable.Lesson{Subject: "Комп сети", Room: "503 (к)"}},
	}}}

	if got := n.changeSummary(ev); !strings.Contains(got, "кабинет 503 (к)") {
		t.Fatalf("a new room must be named: %q", got)
	}
}
