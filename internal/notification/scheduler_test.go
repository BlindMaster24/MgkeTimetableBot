package notification

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/robfig/cron/v3"
)

type mockEventSender struct {
	sent []mockMsg
}

type mockMsg struct {
	chatID  int64
	text    string
	buttons []KeyboardButton
}

func (s *mockEventSender) SendText(chatID int64, text string) error {
	s.sent = append(s.sent, mockMsg{chatID: chatID, text: text})
	return nil
}

func (s *mockEventSender) SendTextWithButtons(chatID int64, text string, buttons []KeyboardButton) error {
	s.sent = append(s.sent, mockMsg{chatID: chatID, text: text, buttons: buttons})
	return nil
}

func (s *mockEventSender) buttoned() []mockMsg {
	var out []mockMsg
	for _, message := range s.sent {
		if len(message.buttons) > 0 {
			out = append(out, message)
		}
	}
	return out
}

type mockEventChatFinder struct {
	byGroups    []*EventChat
	byTeachers  []*EventChat
	subsGroup   []*EventChat
	subsTeacher []*EventChat
	withNotice  []*EventChat
	admins      []*EventChat
}

func (f *mockEventChatFinder) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*EventChat, error) {
	return f.byGroups, nil
}

func (f *mockEventChatFinder) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*EventChat, error) {
	return f.byTeachers, nil
}

func (f *mockEventChatFinder) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*EventChat, error) {
	return f.subsGroup, nil
}

func (f *mockEventChatFinder) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*EventChat, error) {
	return f.subsTeacher, nil
}

func (f *mockEventChatFinder) FindChatsWithNotice(service string, notice string) ([]*EventChat, error) {
	return f.withNotice, nil
}

func (f *mockEventChatFinder) FindAdminChats(service string) ([]*EventChat, error) {
	return f.admins, nil
}

func newTestNotifier(t *testing.T) (*EventNotifier, *mockEventSender, *mockEventChatFinder, *cache.RaspCache) {
	t.Helper()
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	log := logger.New("error", nil)
	sender := &mockEventSender{}
	finder := &mockEventChatFinder{}
	return NewEventNotifier(c, cfg, log, sender, finder), sender, finder, c
}

func todayStr() string {
	return time.Now().Format("02.01.2006")
}

func tomorrowStr() string {
	return time.Now().AddDate(0, 0, 1).Format("02.01.2006")
}

func futureWeekStr() string {
	return time.Now().AddDate(0, 0, 10).Format("02.01.2006")
}

func TestGetDayPhrase_Today(t *testing.T) {
	if got := getDayPhrase(todayStr(), "день"); got != "сегодня" {
		t.Errorf("expected 'сегодня', got %q", got)
	}
}

func TestGetDayPhrase_Tomorrow(t *testing.T) {
	if got := getDayPhrase(tomorrowStr(), "день"); got != "завтра" {
		t.Errorf("expected 'завтра', got %q", got)
	}
}

func TestGetDayPhrase_FutureWeek(t *testing.T) {
	if got := getDayPhrase(futureWeekStr(), "день"); got != "следующую неделю" {
		t.Errorf("expected 'следующую неделю', got %q", got)
	}
}

func TestGetDayPhrase_Other(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3).Format("02.01.2006")
	if got := getDayPhrase(past, "день"); got != "день" {
		t.Errorf("expected fallback phrase, got %q", got)
	}
}

func TestGetDayPhrase_Invalid(t *testing.T) {
	if got := getDayPhrase("not-a-date", "день"); got != "день" {
		t.Errorf("expected fallback phrase, got %q", got)
	}
}

func TestHasAlertableLessons_Simple(t *testing.T) {
	day := map[string]any{
		"lessons": []any{
			map[string]any{"lesson": "Математика", "type": "лек"},
		},
	}
	if !hasAlertableLessons(day, nil) {
		t.Error("expected alertable")
	}
}

func TestHasAlertableLessons_Filtered(t *testing.T) {
	day := map[string]any{
		"lessons": []any{
			map[string]any{"lesson": "Физкультура", "type": "пр"},
		},
	}
	filters := []config.LessonFilter{{Lesson: "Физкультура", Type: "пр"}}
	if hasAlertableLessons(day, filters) {
		t.Error("expected filtered out")
	}
}

func TestHasAlertableLessons_Mixed(t *testing.T) {
	day := map[string]any{
		"lessons": []any{
			map[string]any{"lesson": "Физкультура", "type": "пр"},
			map[string]any{"lesson": "Математика", "type": "лек"},
		},
	}
	filters := []config.LessonFilter{{Lesson: "Физкультура", Type: "пр"}}
	if !hasAlertableLessons(day, filters) {
		t.Error("expected alertable when at least one lesson passes filter")
	}
}

func TestHasAlertableLessons_Subgroups(t *testing.T) {
	day := map[string]any{
		"lessons": []any{
			[]any{
				map[string]any{"lesson": "Физкультура", "type": "пр"},
				map[string]any{"lesson": "Физкультура", "type": "пр"},
			},
		},
	}
	filters := []config.LessonFilter{{Lesson: "Физкультура", Type: "пр"}}
	if hasAlertableLessons(day, filters) {
		t.Error("expected subgroup lessons to be filtered")
	}
}

func TestHasAlertableLessons_Empty(t *testing.T) {
	if hasAlertableLessons(map[string]any{"lessons": []any{}}, nil) {
		t.Error("expected not alertable for empty lessons")
	}
}

func TestFormatCallsLines(t *testing.T) {
	calls := [][2][2]string{
		{{"08:00", "08:45"}, {"08:50", "09:35"}},
		{{"09:45", "10:30"}, {"10:40", "11:25"}},
	}
	got := formatCallsLines(calls)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "1. 08:00 - 08:45 | 08:50 - 09:35" {
		t.Errorf("unexpected line: %q", lines[0])
	}
}

func TestFutureDays(t *testing.T) {
	entry := map[string]any{
		"days": []any{
			map[string]any{"day": todayStr(), "lessons": []any{}},
			map[string]any{"day": tomorrowStr(), "lessons": []any{map[string]any{"lesson": "X"}}},
		},
	}
	future := futureDays(entry)
	if len(future) != 1 {
		t.Fatalf("expected 1 future day, got %d", len(future))
	}
	if dayString(future[0]) != tomorrowStr() {
		t.Errorf("expected tomorrow, got %q", dayString(future[0]))
	}
}

func TestFutureDays_None(t *testing.T) {
	entry := map[string]any{
		"days": []any{
			map[string]any{"day": todayStr(), "lessons": []any{}},
		},
	}
	if got := futureDays(entry); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestTodayDayOf(t *testing.T) {
	entry := map[string]any{
		"days": []any{
			map[string]any{"day": todayStr(), "lessons": []any{map[string]any{"lesson": "A"}, map[string]any{"lesson": "B"}}},
			map[string]any{"day": tomorrowStr(), "lessons": []any{map[string]any{"lesson": "C"}}},
		},
	}
	day, maxLessons := todayDayOf(entry)
	if day == nil {
		t.Fatal("expected today day found")
	}
	if dayString(day) != todayStr() {
		t.Errorf("expected today, got %q", dayString(day))
	}
	if maxLessons != 2 {
		t.Errorf("expected maxLessons=2 (max across all days), got %d", maxLessons)
	}
}

func TestEventNotifier_AddDay_Group(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	day := map[string]any{
		"day": tomorrowStr(),
		"lessons": []any{
			map[string]any{"lesson": "Математика", "type": "лек"},
		},
	}
	n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: day, Type: cache.DayEventAdd})

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if sender.sent[0].chatID != 1001 {
		t.Errorf("expected peerID 1001, got %d", sender.sent[0].chatID)
	}
	if !strings.Contains(sender.sent[0].text, "📢 Расписание на завтра") {
		t.Errorf("unexpected header: %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[0].text, "Математика") {
		t.Errorf("expected lesson in body: %q", sender.sent[0].text)
	}
}

func TestEventNotifier_AddDay_Subscribers(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	finder.subsGroup = []*EventChat{{ID: 2, PeerID: 2002, Group: "", Mode: "guest"}}

	n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: map[string]any{
		"day":     tomorrowStr(),
		"lessons": []any{map[string]any{"lesson": "Математика"}},
	}, Type: cache.DayEventAdd})

	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 messages (base + subscriber), got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "📢 Расписание на завтра") {
		t.Errorf("own-chat header mismatch: %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[1].text, "📢 Группа 63: расписание на завтра") {
		t.Errorf("subscriber header mismatch: %q", sender.sent[1].text)
	}
}

func TestEventNotifier_UpdateDay_Today(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	day := map[string]any{
		"day":     todayStr(),
		"lessons": []any{map[string]any{"lesson": "Физика"}},
	}
	n.UpdateDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: day})

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "🆕 Изменено расписание сегодня") {
		t.Errorf("unexpected header: %q", sender.sent[0].text)
	}
}

func TestEventNotifier_UpdateDay_Teacher(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byTeachers = []*EventChat{{ID: 1, PeerID: 1001, Teacher: "Иванов", Mode: "teacher"}}

	day := map[string]any{
		"day":     tomorrowStr(),
		"lessons": []any{map[string]any{"lesson": "Химия", "group": "63"}},
	}
	n.UpdateDay(&cache.DayEvent{Kind: cache.KindTeachers, Value: "Иванов", Day: day})

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "🆕 Изменено расписание завтра") {
		t.Errorf("unexpected header: %q", sender.sent[0].text)
	}
}

func TestEventNotifier_AddDay_FilteredOut(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	c2 := c
	_ = c2
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	n.cfg.Parser.AlertableIgnoreFilter.Group = []config.LessonFilter{{Lesson: "Физкультура", Type: "пр"}}
	day := map[string]any{
		"day":     tomorrowStr(),
		"lessons": []any{map[string]any{"lesson": "Физкультура", "type": "пр"}},
	}
	n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: day, Type: cache.DayEventAdd})

	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages for filtered lessons, got %d", len(sender.sent))
	}
}

func TestEventNotifier_CronDay_SendsNextFutureDay(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{
					"day": todayStr(),
					"lessons": []any{
						map[string]any{"lesson": "A"}, map[string]any{"lesson": "B"}, map[string]any{"lesson": "C"},
					},
				},
				map[string]any{
					"day":     tomorrowStr(),
					"lessons": []any{map[string]any{"lesson": "D"}},
				},
			},
		},
	}, "h")

	n.CronDay(cache.KindGroups, 2, false)

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "📢 Расписание на завтра") {
		t.Errorf("unexpected header: %q", sender.sent[0].text)
	}
	if last := c.LastNoticedDay(cache.KindGroups, "63"); last == 0 {
		t.Error("expected lastNoticedDay to be set")
	}
}

func TestEventNotifier_CronDay_Dedup(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{
					"day":     todayStr(),
					"lessons": []any{map[string]any{"lesson": "A"}},
				},
				map[string]any{
					"day":     tomorrowStr(),
					"lessons": []any{map[string]any{"lesson": "D"}},
				},
			},
		},
	}, "h")

	n.CronDay(cache.KindGroups, 0, false)
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message on first cron, got %d", len(sender.sent))
	}

	n.CronDay(cache.KindGroups, 0, false)
	if len(sender.sent) != 1 {
		t.Errorf("expected no duplicate message, got %d total", len(sender.sent))
	}
}

func TestEventNotifier_CronDay_IndexMatch(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{
					"day":     todayStr(),
					"lessons": []any{map[string]any{"lesson": "A"}, map[string]any{"lesson": "B"}},
				},
				map[string]any{
					"day":     tomorrowStr(),
					"lessons": []any{map[string]any{"lesson": "C"}},
				},
			},
		},
		"64": map[string]any{
			"days": []any{
				map[string]any{
					"day":     todayStr(),
					"lessons": []any{map[string]any{"lesson": "A"}},
				},
			},
		},
	}, "h")

	n.CronDay(cache.KindGroups, 1, false)

	if len(sender.sent) != 1 {
		t.Fatalf("expected only group with exactly 2 lessons (index 1), got %d", len(sender.sent))
	}
}

func TestEventNotifier_CronDay_EmptyLessonIndexIfEmpty(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	n.cfg.Parser.LessonIndexIfEmpty = 1

	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{"day": todayStr(), "lessons": []any{}},
				map[string]any{"day": tomorrowStr(), "lessons": []any{map[string]any{"lesson": "X"}}},
			},
		},
	}, "h")

	n.CronDay(cache.KindGroups, 0, false)
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message for empty-day group, got %d", len(sender.sent))
	}
}

func TestEventNotifier_CronDay_AllFutureEmpty(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}

	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{"day": todayStr(), "lessons": []any{map[string]any{"lesson": "A"}}},
				map[string]any{"day": tomorrowStr(), "lessons": []any{}},
			},
		},
	}, "h")

	n.CronDay(cache.KindGroups, 0, false)
	if len(sender.sent) != 0 {
		t.Errorf("expected no messages when all future days empty, got %d", len(sender.sent))
	}
}

func TestEventNotifier_UpdateWeek(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student", NoticeNextWeek: true}}

	nextWeek := utils.WeekIndexFromDate(time.Now()).Value() + 1
	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{
					"day":     futureWeekStr(),
					"lessons": []any{map[string]any{"lesson": "X"}},
				},
			},
		},
	}, "h")

	n.UpdateWeek(cache.KindGroups, nextWeek)

	if len(sender.buttoned()) != 1 {
		t.Fatalf("expected 1 buttoned message, got %d", len(sender.buttoned()))
	}
	if !strings.Contains(sender.buttoned()[0].text, "🆕 Доступно расписание на следующую неделю") {
		t.Errorf("unexpected text: %q", sender.buttoned()[0].text)
	}
	if len(sender.buttoned()[0].buttons) != 1 {
		t.Fatalf("expected 1 button")
	}
	btn := sender.buttoned()[0].buttons[0]
	if btn.Text != "📃 Показать" {
		t.Errorf("unexpected button label: %q", btn.Text)
	}
	expected := fmt.Sprintf("timetable_g:63:%d:0:1", nextWeek)
	if btn.Data != expected {
		t.Errorf("expected callback %q, got %q", expected, btn.Data)
	}
}

func TestEventNotifier_UpdateWeek_NoNotice(t *testing.T) {
	n, sender, finder, c := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student", NoticeNextWeek: false}}

	nextWeek := utils.WeekIndexFromDate(time.Now()).Value() + 1
	c.SetGroups(map[string]any{
		"63": map[string]any{
			"days": []any{
				map[string]any{"day": futureWeekStr(), "lessons": []any{map[string]any{"lesson": "X"}}},
			},
		},
	}, "h")

	n.UpdateWeek(cache.KindGroups, nextWeek)
	if len(sender.buttoned()) != 0 {
		t.Errorf("expected 0 messages when noticeNextWeek off, got %d", len(sender.buttoned()))
	}
}

func TestEventNotifier_WithdrawWeek(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student", NoticeNextWeek: true}}
	finder.subsGroup = []*EventChat{{ID: 2, PeerID: 2002, NoticeNextWeek: true}}

	n.WithdrawWeek(cache.KindGroups, 99999, []string{"63"})

	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].text, "❌ Расписание на следующую неделю отозвано.") {
		t.Errorf("unexpected base text: %q", sender.sent[0].text)
	}
	if !strings.Contains(sender.sent[1].text, "❌ Группа 63: расписание на следующую неделю отозвано.") {
		t.Errorf("unexpected scoped text: %q", sender.sent[1].text)
	}
}

func TestEventNotifier_WithdrawWeek_Empty(t *testing.T) {
	n, sender, _, _ := newTestNotifier(t)
	n.WithdrawWeek(cache.KindGroups, 99999, nil)
	if len(sender.sent) != 0 {
		t.Errorf("expected 0 messages, got %d", len(sender.sent))
	}
}

func TestEventNotifier_CallsChanged(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.withNotice = []*EventChat{
		{ID: 1, PeerID: 1001, Group: "63", NoticeCalls: true},
		{ID: 2, PeerID: 2002, Teacher: "Иванов", NoticeCalls: true},
	}

	n.CallsChanged(&cache.CallsEvent{
		WeekdaysChanged: true,
		SaturdayChanged: false,
		Reason:          "тест",
		Schedule: cache.CallsSchedule{
			Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:50", "09:35"}}},
		},
	})

	if len(sender.buttoned()) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sender.buttoned()))
	}
	text := sender.buttoned()[0].text
	if !strings.Contains(text, "🔔 Изменено расписание звонков") {
		t.Errorf("unexpected text: %q", text)
	}
	if !strings.Contains(text, "Причина: тест") {
		t.Errorf("expected reason line: %q", text)
	}
	if !strings.Contains(text, "1. 08:00 - 08:45 | 08:50 - 09:35") {
		t.Errorf("expected calls line: %q", text)
	}
	btn := sender.buttoned()[0].buttons[0]
	if btn.Text != "📊 Показать" || btn.Data != "calls_full" {
		t.Errorf("unexpected button: %+v", btn)
	}
}

func TestEventNotifier_CallsChanged_NoChange(t *testing.T) {
	n, sender, _, _ := newTestNotifier(t)
	n.CallsChanged(&cache.CallsEvent{})
	if len(sender.buttoned()) != 0 {
		t.Errorf("expected 0 messages, got %d", len(sender.buttoned()))
	}
}

func TestEventNotifier_ParserError(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.withNotice = []*EventChat{{ID: 1, PeerID: 1001, NoticeParserErrs: true}}
	finder.admins = []*EventChat{{ID: 2, PeerID: 2002}}

	n.ParserError(errors.New("boom"))

	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 messages (subscriber + admin), got %d", len(sender.sent))
	}
	for _, m := range sender.sent {
		if !strings.HasPrefix(m.text, "Parser error\n") {
			t.Errorf("unexpected text: %q", m.text)
		}
		if !strings.Contains(m.text, "boom") {
			t.Errorf("expected error text: %q", m.text)
		}
	}
}

func TestEventNotifier_HandleEvents_Dispatch(t *testing.T) {
	n, sender, finder, _ := newTestNotifier(t)
	finder.byGroups = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", Mode: "student"}}
	finder.withNotice = []*EventChat{{ID: 1, PeerID: 1001, Group: "63", NoticeCalls: true}}

	tomorrow := map[string]any{
		"day":     tomorrowStr(),
		"lessons": []any{map[string]any{"lesson": "X"}},
	}
	events := []cache.Event{
		{Day: &cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: tomorrow, Type: cache.DayEventAdd}},
		{Calls: &cache.CallsEvent{WeekdaysChanged: true, Schedule: cache.CallsSchedule{
			Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:50", "09:35"}}},
		}}},
	}
	n.HandleEvents(events)

	total := len(sender.sent)
	if total < 2 {
		t.Errorf("expected day + calls notifications, got %d", total)
	}
}

func TestScheduler_Registration(t *testing.T) {
	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	cfg.Timetable.Weekdays = [][2][2]string{
		{{"08:00", "08:45"}, {"08:50", "09:35"}},
		{{"09:45", "10:30"}, {"10:40", "11:25"}},
	}
	cfg.Timetable.Saturday = [][2][2]string{
		{{"08:00", "08:45"}, {"08:50", "09:35"}},
	}

	log := logger.New("error", nil)
	sender := &mockEventSender{}
	finder := &mockEventChatFinder{}

	s := NewScheduler(cfg, c, log, sender, finder, nil, nil)
	s.Start()
	defer s.Stop()

	entries := s.cron.Entries()
	if len(entries) != 3 {
		t.Errorf("expected 3 cron entries (2 weekday + 1 saturday), got %d", len(entries))
	}
	_ = fmt.Sprint()
}

func TestScheduler_Registration_EmptyTimetable(t *testing.T) {
	c, _ := cache.New(t.TempDir())
	cfg := &config.Config{}
	log := logger.New("error", nil)
	s := NewScheduler(cfg, c, log, &mockEventSender{}, &mockEventChatFinder{}, nil, nil)
	s.Start()
	defer s.Stop()

	if entries := s.cron.Entries(); len(entries) != 0 {
		t.Errorf("expected 0 entries for empty timetable, got %d", len(entries))
	}
}

func TestSlotCronExprUsesTheBellScheduleTimes(t *testing.T) {
	for _, testCase := range []struct {
		endTime  string
		weekdays string
		want     string
		ok       bool
	}{
		{endTime: "08:45", weekdays: "1-5", want: "0 45 08 * * 1-5", ok: true},
		{endTime: "09:45", weekdays: "6", want: "0 45 09 * * 6", ok: true},
		{endTime: "13:00", weekdays: "1-5", want: "0 00 13 * * 1-5", ok: true},
		{endTime: "8:5", weekdays: "1-5", want: "0 5 8 * * 1-5", ok: true},
		{endTime: "", weekdays: "1-5", ok: false},
		{endTime: "08:45:00", weekdays: "1-5", ok: false},
		{endTime: "08:xx", weekdays: "1-5", ok: false},
		{endTime: "abc", weekdays: "1-5", ok: false},
	} {
		expr, ok := slotCronExpr(testCase.endTime, testCase.weekdays)
		if ok != testCase.ok {
			t.Errorf("slotCronExpr(%q) ok = %v, want %v", testCase.endTime, ok, testCase.ok)
			continue
		}
		if ok && expr != testCase.want {
			t.Errorf("slotCronExpr(%q) = %q, want %q", testCase.endTime, expr, testCase.want)
		}
	}
}

func TestNotificationTimeIsInterpretedInTheEvaluationZone(t *testing.T) {
	minsk := time.FixedZone("Europe/Minsk", 3*60*60)
	utc := time.UTC

	expr, ok := slotCronExpr("08:45", "1-5")
	if !ok {
		t.Fatal("expected a cron expression")
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expr)
	if err != nil {
		t.Fatalf("parse %q: %v", expr, err)
	}

	monday := time.Date(2026, 9, 14, 6, 0, 0, 0, utc)

	utcNext := schedule.Next(monday.In(utc))
	localNext := schedule.Next(monday.In(minsk))

	if utcNext.Hour() != 8 || utcNext.Minute() != 45 || utcNext.Location() != utc {
		t.Errorf("a UTC process would notify at %s", utcNext)
	}
	if localNext.Hour() != 8 || localNext.Minute() != 45 || localNext.Location() != minsk {
		t.Errorf("a local process would notify at %s", localNext)
	}
	if utcNext.Equal(localNext) {
		t.Error("08:45 UTC and 08:45 in the local zone must be different instants")
	}
	if got := localNext.UTC().Hour(); got != 5 {
		t.Errorf("08:45 in a +03:00 process is %02d:45 UTC, got %02d:45", 5, got)
	}
}

func TestSchedulerBindsTheCronToTheProcessTimezone(t *testing.T) {
	original := time.Local
	minsk := time.FixedZone("Europe/Minsk", 3*60*60)
	time.Local = minsk
	t.Cleanup(func() { time.Local = original })

	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	cfg := &config.Config{}
	cfg.Timetable.Weekdays = [][2][2]string{{{"08:00", "08:45"}, {"08:50", "09:35"}}}

	s := NewScheduler(cfg, c, logger.New("error", nil), &mockEventSender{}, &mockEventChatFinder{}, nil, nil)

	if s.Location() != minsk {
		t.Errorf("the scheduler must follow the process zone, got %v", s.Location())
	}
}

func TestEventChatFinder_InterfaceCompliance(t *testing.T) {
	var _ EventChatFinder = (*mockEventChatFinder)(nil)
	var _ EventSender = (*mockEventSender)(nil)
}
