package notification

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
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

func TestFormatLesson_String(t *testing.T) {
	result := formatLesson("invalid", 1)
	if result != "" {
		t.Errorf("expected empty for string, got %q", result)
	}
}

func TestFormatLesson_EmptyMap(t *testing.T) {
	result := formatLesson(map[string]any{}, 1)
	if result != "" {
		t.Errorf("expected empty for empty map, got %q", result)
	}
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

func TestFormatSingleLesson_NoSubgroup(t *testing.T) {
	m := map[string]any{
		"lesson": "Алгебра",
		"type":   "пр",
	}
	result := formatSingleLesson(m, 1)
	if result != "1. Алгебра (пр)" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatSingleLesson_Empty(t *testing.T) {
	result := formatSingleLesson(map[string]any{}, 1)
	if result != "" {
		t.Errorf("expected empty for empty map, got %q", result)
	}
}

func TestFormatSingleLesson_OnlyComment(t *testing.T) {
	m := map[string]any{
		"comment": "замена",
	}
	result := formatSingleLesson(m, 2)
	if result != "2. [замена]" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestBuildGroupNotification_EmptyCache(t *testing.T) {
	s := &Scheduler{}
	result := s.buildGroupNotification("63", nil, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for nil data, got %q", result)
	}
}

func TestBuildGroupNotification_WrongType(t *testing.T) {
	s := &Scheduler{}
	result := s.buildGroupNotification("63", "not a map", "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for wrong type, got %q", result)
	}
}

func TestBuildGroupNotification_NoDays(t *testing.T) {
	s := &Scheduler{}
	result := s.buildGroupNotification("63", map[string]any{}, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for no days, got %q", result)
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

func TestBuildGroupNotification_EmptyLessons(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day":     "01.09.2025",
				"lessons": []any{},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildGroupNotification("63", data, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for empty lessons, got %q", result)
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
	if !contains(result, "Группа 63") {
		t.Error("expected group name in notification")
	}
	t.Logf("notification:\n%s", result)
}

func TestBuildGroupNotification_WithSubgroups(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day": "01.09.2025",
				"lessons": []any{
					[]any{
						map[string]any{"lesson": "Физика", "type": "пр", "subgroup": float64(1)},
						map[string]any{"lesson": "Физика", "type": "пр", "subgroup": float64(2)},
					},
				},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildGroupNotification("63", data, "01.09.2025")
	if result == "" {
		t.Error("expected non-empty notification with subgroups")
	}
}

func TestBuildTeacherNotification_EmptyCache(t *testing.T) {
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов", nil, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildTeacherNotification_WrongType(t *testing.T) {
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов", "not map", "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for wrong type, got %q", result)
	}
}

func TestBuildTeacherNotification_NoMatchingDay(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{"day": "02.09.2025", "lessons": []any{}},
		},
	}
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов", data, "01.09.2025")
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
	if !contains(result, "Преподаватель Иванов А.А.") {
		t.Error("expected teacher name in notification")
	}
}

func TestBuildTeacherNotification_EmptyLessons(t *testing.T) {
	data := map[string]any{
		"days": []any{
			map[string]any{
				"day":     "01.09.2025",
				"lessons": []any{},
			},
		},
	}
	s := &Scheduler{}
	result := s.buildTeacherNotification("Иванов", data, "01.09.2025")
	if result != "" {
		t.Errorf("expected empty for empty lessons, got %q", result)
	}
}

func TestSendNotifications_NoChats(t *testing.T) {
	sender := &mockSender{}
	finder := &mockChatFinder{chats: nil}
	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")
	if len(sender.sent) != 0 {
		t.Errorf("expected no messages sent, got %d", len(sender.sent))
	}
}

func TestSendNotifications_WithChats(t *testing.T) {
	sender := &mockSender{}
	now_str := time.Now().Format("02.01.2006")
	c, _ := cache.New(t.TempDir())
	c.SetGroups(map[string]any{
		"100": map[string]any{
			"days": []any{
				map[string]any{
					"day": now_str,
					"lessons": []any{
						map[string]any{"lesson": "Математика", "type": "лекция"},
					},
				},
			},
		},
	}, "h")

	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "student", Group: "100", NoticeChanges: true},
	}}

	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)

	s.sendNotifications("weekday")

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message sent, got %d", len(sender.sent))
	}
}

func TestSendNotifications_NoticeDisabled(t *testing.T) {
	sender := &mockSender{}
	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "student", Group: "100", NoticeChanges: false},
	}}

	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")

	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages for disabled notice, got %d", len(sender.sent))
	}
}

func TestSendNotifications_EmptyGroup(t *testing.T) {
	sender := &mockSender{}
	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "student", Group: "", NoticeChanges: true},
	}}

	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")

	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages for empty group, got %d", len(sender.sent))
	}
}

func TestSendNotifications_TeacherMode(t *testing.T) {
	sender := &mockSender{}
	nowStr := time.Now().Format("02.01.2006")
	c, _ := cache.New(t.TempDir())
	c.SetTeachers(map[string]any{
		"Иванов": map[string]any{
			"days": []any{
				map[string]any{
					"day": nowStr,
					"lessons": []any{
						map[string]any{"lesson": "Физика", "type": "лекция", "group": "100"},
					},
				},
			},
		},
	}, "h")

	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "teacher", Teacher: "Иванов", NoticeChanges: true},
	}}

	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message for teacher, got %d", len(sender.sent))
	}
}

func TestSendNotifications_TeacherEmptyName(t *testing.T) {
	sender := &mockSender{}
	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "teacher", Teacher: "", NoticeChanges: true},
	}}

	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")

	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages for empty teacher, got %d", len(sender.sent))
	}
}

func TestSendNotifications_ParentMode(t *testing.T) {
	sender := &mockSender{}
	nowStr := time.Now().Format("02.01.2006")
	c, _ := cache.New(t.TempDir())
	c.SetGroups(map[string]any{
		"100": map[string]any{
			"days": []any{
				map[string]any{
					"day": nowStr,
					"lessons": []any{
						map[string]any{"lesson": "Русский", "type": "лекция"},
					},
				},
			},
		},
	}, "h")

	finder := &mockChatFinder{chats: []*ChatInfo{
		{ID: 1001, Mode: "parent", Group: "100", NoticeChanges: true},
	}}

	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, sender, finder)
	s.sendNotifications("weekday")

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 message for parent, got %d", len(sender.sent))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
