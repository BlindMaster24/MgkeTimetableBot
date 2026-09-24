package notification

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/testgolden"
)

var (
	updateMessageGolden = flag.Bool("update", false, "rewrite golden files")
	rawDateRe           = regexp.MustCompile(`\d{2}\.\d{2}(\.\d{4})?|№\s*\d+`)
)

const messageGoldenPath = "testdata/messages.golden"

type messageScenario struct {
	name string
	run  func(t *testing.T, n *EventNotifier, sender *mockEventSender, finder *mockEventChatFinder)
}

func TestNotificationTextsGolden(t *testing.T) {
	got := renderNotifications(t)

	if *updateMessageGolden {
		if err := os.MkdirAll(filepath.Dir(messageGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(messageGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(messageGoldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v (run go test ./internal/notification -update)", err)
	}
	wantText := testgolden.NormalizeLineEndings(string(want))
	if got != wantText {
		t.Errorf("notification texts changed; review the diff and run:\n  go test ./internal/notification -update\n\n%s",
			testgolden.FirstDifference(wantText, got))
	}
}

func TestNotificationGoldenLeavesNoRawDates(t *testing.T) {
	for _, scenario := range notificationScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			rendered := renderScenario(t, scenario)
			if match := rawDateRe.FindString(rendered); match != "" {
				t.Errorf("a raw date or week number %q reached the golden output, the check would fail on another day:\n%s", match, rendered)
			}
		})
	}
}

func renderNotifications(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	for _, scenario := range notificationScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			b.WriteString("=== " + scenario.name + " ===\n")
			b.WriteString(renderScenario(t, scenario))
			b.WriteString("\n")
		})
	}
	return b.String()
}

func renderScenario(t *testing.T, scenario messageScenario) string {
	t.Helper()

	n, sender, finder, _ := newTestNotifier(t)
	scenario.run(t, n, sender, finder)

	now := time.Now()
	week := testgolden.RelevantWeek(now).Value()

	var b strings.Builder
	for _, message := range sender.sent {
		b.WriteString(fmt.Sprintf("[peer %d]\n", message.chatID))
		b.WriteString(testgolden.Normalize(message.text, now))
		b.WriteString("\n")
		for _, button := range message.buttons {
			b.WriteString(fmt.Sprintf("  button: %s -> %s\n", button.Text, testgolden.NormalizeWeekNumber(button.Data, week)))
		}
	}
	if b.Len() == 0 {
		return "(nothing was sent)\n"
	}
	return b.String()
}

func goldenDay(offset int) map[string]any {
	return map[string]any{
		"day": time.Now().AddDate(0, 0, offset).Format("02.01.2006"),
		"lessons": []any{
			map[string]any{"lesson": "Математика", "type": "Лек", "teacher": "Иванов И.И.", "cabinet": "101"},
			map[string]any{"lesson": "Физика", "type": "ЛР", "teacher": "Петров П.П.", "cabinet": "202"},
		},
	}
}

func goldenTeacherDay(offset int) map[string]any {
	return map[string]any{
		"day": time.Now().AddDate(0, 0, offset).Format("02.01.2006"),
		"lessons": []any{
			map[string]any{"lesson": "Математика", "type": "Лек", "group": "63", "cabinet": "101"},
			map[string]any{"lesson": "Физика", "type": "ЛР", "group": "77", "cabinet": "202"},
		},
	}
}

func goldenChats() []*EventChat {
	return []*EventChat{{
		ID:             1,
		PeerID:         1001,
		Mode:           "student",
		Group:          "63",
		NoticeChanges:  true,
		NoticeNextWeek: true,
		NoticeCalls:    true,
		Formatter:      0,
	}}
}

func notificationScenarios() []messageScenario {
	return []messageScenario{
		{
			name: "day/add/today",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byGroups = goldenChats()
				n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: goldenDay(0)})
			},
		},
		{
			name: "day/add/tomorrow",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byGroups = goldenChats()
				n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: goldenDay(1)})
			},
		},
		{
			name: "day/update/tomorrow",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byGroups = goldenChats()
				n.UpdateDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: goldenDay(1)})
			},
		},
		{
			name: "day/add/subscription",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.subsGroup = []*EventChat{{
					ID:      2,
					PeerID:  1002,
					Mode:    "student",
					Group:   "77",
					Teacher: "",
				}}
				n.AddDay(&cache.DayEvent{Kind: cache.KindGroups, Value: "63", Day: goldenDay(1)})
			},
		},
		{
			name: "day/update/teacher",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byTeachers = []*EventChat{{
					ID:            3,
					PeerID:        1003,
					Mode:          "teacher",
					Teacher:       "Сидоров С.С.",
					NoticeChanges: true,
				}}
				n.UpdateDay(&cache.DayEvent{Kind: cache.KindTeachers, Value: "Иванов И.И.", Day: goldenTeacherDay(1)})
			},
		},
		{
			name: "week/new",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				chat := goldenChats()[0]
				finder.byGroups = []*EventChat{chat}
				finder.subsGroup = []*EventChat{{
					ID:             2,
					PeerID:         1002,
					Mode:           "student",
					Group:          "77",
					NoticeNextWeek: true,
				}}
				n.cache.SetGroups(map[string]any{
					"63": map[string]any{"days": []any{goldenDay(1)}},
					"77": map[string]any{"days": []any{goldenDay(1)}},
				}, "golden")
				n.UpdateWeek(cache.KindGroups, testgolden.RelevantWeek(time.Now()).Value())
			},
		},
		{
			name: "week/withdrawn",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byGroups = goldenChats()
				finder.subsGroup = []*EventChat{{
					ID:             2,
					PeerID:         1002,
					Mode:           "student",
					Group:          "63",
					NoticeNextWeek: true,
				}}
				n.WithdrawWeek(cache.KindGroups, testgolden.RelevantWeek(time.Now()).Value(), []string{"63"})
			},
		},
		{
			name: "calls/changed",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.withNotice = goldenChats()
				n.CallsChanged(&cache.CallsEvent{
					WeekdaysChanged: true,
					SaturdayChanged: true,
					Reason:          "Сокращённый день",
					Schedule: cache.CallsSchedule{
						Weekdays: [][2][2]string{{{"10:00", "10:30"}, {"10:40", "11:10"}}},
						Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
					},
				})
			},
		},
		{
			name: "parser/error",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				chat := goldenChats()[0]
				chat.NoticeParserErrs = true
				finder.withNotice = []*EventChat{chat}
				finder.admins = []*EventChat{{ID: 9, PeerID: 1009, Mode: "student", Group: "63"}}
				n.ParserError(errors.New("layout changed: no data for \"th[colspan] with dd.MM.yyyy\""))
			},
		},
		{
			name: "cron/last-lesson",
			run: func(t *testing.T, n *EventNotifier, _ *mockEventSender, finder *mockEventChatFinder) {
				finder.byGroups = goldenChats()
				n.cfg.Parser.LessonIndexIfEmpty = 2
				n.cache.SetGroups(map[string]any{
					"63": map[string]any{"days": []any{
						map[string]any{"day": time.Now().Format("02.01.2006"), "lessons": []any{
							map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101"},
							map[string]any{"lesson": "Физика", "type": "ЛР", "cabinet": "202"},
						}},
						goldenDay(1),
					}},
				}, "golden")
				n.CronDay(cache.KindGroups, 1, true)
			},
		},
	}
}
