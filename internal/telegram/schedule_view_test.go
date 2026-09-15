package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

func seedDayViewBot(t *testing.T) (*Bot, *recordingCaller) {
	t.Helper()

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)

	today := time.Now().Format("02.01.2006")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("02.01.2006")

	groups := map[string]any{
		"777": e2eJsonRoundTrip(map[string]any{
			"group": "777",
			"days": []any{
				map[string]any{"day": today, "lessons": []any{
					map[string]any{"lesson": "История", "type": "Лек", "teacher": "Орлов О.О.", "cabinet": "404"},
				}},
				map[string]any{"day": tomorrow, "lessons": []any{
					map[string]any{"lesson": "География", "type": "Пр", "teacher": "Соколов С.С.", "cabinet": "505"},
				}},
			},
		}),
	}
	b.cache.SetGroups(groups, "day-view")

	teachers := map[string]any{
		"Орлов О.О.": e2eJsonRoundTrip(map[string]any{
			"teacher": "Орлов О.О.",
			"days": []any{
				map[string]any{"day": today, "lessons": []any{
					map[string]any{"lesson": "История", "type": "Лек", "group": "777", "cabinet": "404"},
				}},
			},
		}),
	}
	b.cache.SetTeachers(teachers, "day-view")

	chat, err := repo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatalf("find chat: %v", err)
	}
	chat.Mode = ModeStudent
	chat.Group = "777"
	chat.ShowHints = false
	if err := repo.Save(chat); err != nil {
		t.Fatalf("save chat: %v", err)
	}
	return b, caller
}

func assertLessonsRendered(t *testing.T, text string) {
	t.Helper()

	if strings.Contains(text, "Нет расписания для отображения") {
		t.Fatalf("the schedule is not dated: %q", text)
	}
	if !strings.Contains(text, "История") && !strings.Contains(text, "География") {
		t.Fatalf("no lesson reached the user: %q", text)
	}
}

func TestE2E_DayViewRendersTheCurrentDay(t *testing.T) {
	b, caller := seedDayViewBot(t)

	cmd := &dayCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: "/day"}); err != nil {
		t.Fatalf("day command: %v", err)
	}

	assertLessonsRendered(t, caller.deliveredText())
}

func TestE2E_WeekViewRendersTheCurrentWeek(t *testing.T) {
	b, caller := seedDayViewBot(t)

	cmd := &weekCmd{bot: b}
	if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: 4242, UserID: 4242, Text: "/week"}); err != nil {
		t.Fatalf("week command: %v", err)
	}

	text := caller.deliveredText()
	if !strings.Contains(text, "Учебная неделя №") {
		t.Errorf("the week view has no academic week label: %q", text)
	}
	assertLessonsRendered(t, text)
}
