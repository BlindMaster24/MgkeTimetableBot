package notification

import (
	"testing"
)

type mockSender struct {
	sent []sentMsg
}

type sentMsg struct {
	chatID int64
	text   string
}

func (s *mockSender) SendText(chatID int64, text string) error {
	s.sent = append(s.sent, sentMsg{chatID: chatID, text: text})
	return nil
}

type mockChatFinder struct {
	chats []*ChatInfo
}

func (f *mockChatFinder) FindAllWithNotifications(service string) ([]*ChatInfo, error) {
	return f.chats, nil
}

func TestFormatLesson_Nil(t *testing.T) {
	result := formatLesson(nil, 1)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestFormatLesson_Map(t *testing.T) {
	lesson := map[string]any{
		"lesson": "Математика",
		"type":   "лекция",
	}
	result := formatLesson(lesson, 1)
	if result == "" {
		t.Error("expected non-empty")
	}
	t.Logf("lesson: %s", result)
}

func TestFormatLesson_Array(t *testing.T) {
	lesson := []any{
		map[string]any{
			"lesson":   "Физика",
			"type":     "пр-ка",
			"subgroup": float64(1),
		},
		map[string]any{
			"lesson":   "Физика",
			"type":     "пр-ка",
			"subgroup": float64(2),
		},
	}
	result := formatLesson(lesson, 1)
	if result == "" {
		t.Error("expected non-empty")
	}
	t.Logf("lesson: %s", result)
}

func TestFormatSingleLesson(t *testing.T) {
	m := map[string]any{
		"lesson":   "Математика",
		"type":     "лекция",
		"comment":  "доп. занятие",
		"subgroup": float64(2),
	}
	result := formatSingleLesson(m, 3)
	if result == "" {
		t.Error("expected non-empty")
	}
	if result != "3. 2. Математика (лекция) [доп. занятие]" {
		t.Errorf("unexpected format: %q", result)
	}
}

func TestBuildGroupNotification_EmptyCache(t *testing.T) {
	s := &Scheduler{}
	result := s.buildGroupNotification("63", nil, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for nil data, got %q", result)
	}
}

func TestBuildGroupNotification_NoMatchingDay(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day":     "02.09.2025",
				"lessons": []any{map[string]any{"lesson": "Math"}},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildGroupNotification("63", data, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty when no matching day, got %q", result)
	}
}

func TestBuildGroupNotification_WithLessons(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day": "01.09.2025",
				"lessons": []any{
					map[string]any{"lesson": "Математика", "type": "лекция"},
					map[string]any{"lesson": "Физика", "type": "пр-ка"},
				},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildGroupNotification("63", data, "01.09.2025")
	if result == "" {
		t.Error("expected non-empty notification")
	}
	t.Logf("notification:\n%s", result)
}

func TestBuildTeacherNotification_EmptyCache(t *testing.T) {
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов", nil, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildTeacherNotification_WithLessons(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day": "01.09.2025",
				"lessons": []any{
					map[string]any{"lesson": "Математика", "group": "63ТП", "type": "лекция"},
				},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов А.А.", data, "01.09.2025")
	if result == "" {
		t.Error("expected non-empty notification")
	}
	t.Logf("notification:\n%s", result)
}
