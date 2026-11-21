package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
)

func TestEveryCommandExplainsItselfInTheLocale(t *testing.T) {
	bot, _ := setupE2EBot(t)

	for _, command := range bot.commandOrder {
		name := command.Name()
		if !strings.HasPrefix(name, "/") {
			t.Errorf("command %q must start with a slash", name)
		}
		description := command.Description()
		if description == "" {
			t.Errorf("command %q has no description", name)
			continue
		}
		if description == name {
			t.Errorf("command %q describes itself with its name", name)
		}
		if strings.Contains(description, "_") && strings.Count(description, " ") == 0 {
			t.Errorf("command %q shows a raw locale key: %q", name, description)
		}
	}
}

func TestLessonTextSpeaksForBothShapes(t *testing.T) {
	single := map[string]any{
		"lesson":  "Математика",
		"type":    "лек",
		"cabinet": "101",
		"teacher": "Иванов И.И.",
		"group":   "100",
	}
	text := lessonText(single)
	for _, want := range []string{"Математика", "лек", "101"} {
		if !strings.Contains(text, want) {
			t.Errorf("lessonText(%v) = %q, missing %q", single, text, want)
		}
	}

	list := lessonText([]any{single, map[string]any{"lesson": "Физика"}})
	if !strings.Contains(list, " / ") || !strings.Contains(list, "Физика") {
		t.Fatalf("a lesson array = %q, want both lessons joined", list)
	}
	if got := lessonText("строка"); got != "" {
		t.Fatalf("lessonText(string) = %q, want empty", got)
	}
}

func TestFirstNonEmptySkipsBlanks(t *testing.T) {
	if got := firstNonEmpty("", "", "первое", "второе"); got != "первое" {
		t.Fatalf("firstNonEmpty = %q, want the first real value", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty with only blanks = %q, want empty", got)
	}
}

func TestGoogleProgressFollowsTheSyncedDay(t *testing.T) {
	bounds := archive.Bounds{Min: 100, Max: 200}
	if got := googleProgress(&GoogleCalendar{}, bounds, true); got != 100 {
		t.Fatalf("progress without a synced day = %v, want 100", got)
	}
	if got := googleProgress(&GoogleCalendar{LastManualSyncedDay: 50}, bounds, true); got != 0 {
		t.Fatalf("progress before the first day = %v, want 0", got)
	}
	if got := googleProgress(&GoogleCalendar{LastManualSyncedDay: 150}, bounds, true); got != 50 {
		t.Fatalf("progress in the middle = %v, want 50", got)
	}
	if got := googleProgress(&GoogleCalendar{LastManualSyncedDay: 150}, bounds, false); got != 100 {
		t.Fatalf("progress without bounds = %v, want 100", got)
	}
	if got := googleProgress(&GoogleCalendar{LastManualSyncedDay: 150}, archive.Bounds{Min: 150, Max: 150}, true); got != 100 {
		t.Fatalf("progress with a zero span = %v, want 100", got)
	}
}
