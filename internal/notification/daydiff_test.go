package notification

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
)

func diffUpdateText(t *testing.T, first, second []any) string {
	t.Helper()
	n, sender, finder, c := newTestNotifier(t)
	n.SetLocalizer(i18n.New("ru"))
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	today := todayStr()
	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(today, first)),
	}, "hash1")
	n.HandleEvents(c.DrainEvents(t.Context()))

	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(today, second)),
	}, "hash2")
	n.HandleEvents(c.DrainEvents(t.Context()))

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 update notification, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "🆕 Изменено расписание") {
		t.Fatalf("expected update header, got %q", sender.sent[0].text)
	}
	return sender.sent[0].text
}

func changeLesson(subject, room, teacher string) map[string]any {
	return map[string]any{"lesson": subject, "cabinet": room, "teacher": teacher, "time": "08:00 - 08:45"}
}

func TestIntegration_UpdateSurfacesAddedLesson(t *testing.T) {
	text := diffUpdateText(t,
		[]any{changeLesson("Математика", "101", "А")},
		[]any{changeLesson("Математика", "101", "А"), changeLesson("Физика", "102", "Б")},
	)
	for _, want := range []string{"🔍 Изменения:", "➕ Физика"} {
		if !strings.Contains(text, want) {
			t.Errorf("update must contain %q, got %q", want, text)
		}
	}
}

func TestIntegration_UpdateSurfacesRemovedLesson(t *testing.T) {
	text := diffUpdateText(t,
		[]any{changeLesson("Математика", "101", "А"), changeLesson("Физика", "102", "Б")},
		[]any{changeLesson("Математика", "101", "А")},
	)
	for _, want := range []string{"🔍 Изменения:", "➖ Физика"} {
		if !strings.Contains(text, want) {
			t.Errorf("update must contain %q, got %q", want, text)
		}
	}
}

func TestIntegration_UpdateSurfacesMovedLesson(t *testing.T) {
	text := diffUpdateText(t,
		[]any{changeLesson("Математика", "101", "А"), changeLesson("Физика", "102", "Б")},
		[]any{changeLesson("Физика", "102", "Б"), changeLesson("Математика", "101", "А")},
	)
	for _, want := range []string{"🔍 Изменения:", "🔀 Физика", "🔀 Математика"} {
		if !strings.Contains(text, want) {
			t.Errorf("update must contain %q, got %q", want, text)
		}
	}
}

func TestIntegration_UpdateSurfacesRoomChange(t *testing.T) {
	text := diffUpdateText(t,
		[]any{changeLesson("Математика", "101", "А")},
		[]any{changeLesson("Математика", "205", "А")},
	)
	for _, want := range []string{"🔍 Изменения:", "🚪", "205"} {
		if !strings.Contains(text, want) {
			t.Errorf("update must contain %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "➕") || strings.Contains(text, "➖") {
		t.Errorf("room change must not report add/remove, got %q", text)
	}
}

func TestIntegration_UpdateSurfacesTeacherChange(t *testing.T) {
	text := diffUpdateText(t,
		[]any{changeLesson("Математика", "101", "Иванов И.И.")},
		[]any{changeLesson("Математика", "101", "Петров П.П.")},
	)
	for _, want := range []string{"🔍 Изменения:", "Петров П.П."} {
		if !strings.Contains(text, want) {
			t.Errorf("update must contain %q, got %q", want, text)
		}
	}
}

func TestIntegration_UpdateSurfacesTimeChange(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	today := todayStr()
	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(today, []any{
			map[string]any{"lesson": "Математика", "time": "08:00 - 08:45"},
		})),
	}, "hash1")
	n.HandleEvents(c.DrainEvents(t.Context()))

	c.SetGroups(t.Context(), map[string]any{
		"63": entryWithDays(dayMap(today, []any{
			map[string]any{"lesson": "Математика", "time": "10:00 - 10:45"},
		})),
	}, "hash2")
	n.HandleEvents(c.DrainEvents(t.Context()))

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 update notification, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "🕒") {
		t.Errorf("update must contain time change, got %q", sender.sent[0].text)
	}
}

func TestIntegration_UpdateWithoutDiffSkipsSummary(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	n.UpdateDay(&cache.DayEvent{
		Kind:  cache.KindGroups,
		Value: "63",
		Day:   dayMap(todayStr(), []any{changeLesson("Математика", "101", "А")}),
	})
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 update notification, got %d", len(sender.sent))
	}
	if strings.Contains(sender.sent[0].text, "🔍 Изменения:") {
		t.Errorf("update without diff must skip the summary, got %q", sender.sent[0].text)
	}
}
