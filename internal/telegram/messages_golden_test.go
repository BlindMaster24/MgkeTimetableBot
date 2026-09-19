package telegram

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/testgolden"
)

var rawDateRe = regexp.MustCompile(`\d{2}\.\d{2}(\.\d{4})?|№\s*\d+`)

const (
	messageGoldenPath = "testdata/messages.golden"
	dayGroup          = "778"
	weekGroup         = "777"
	dayTeacher        = "Иванов И.И."
	weekTeacher       = "Петров П.П."
)

type messageScenario struct {
	name     string
	chat     func(chat *Chat)
	setup    func(b *Bot)
	handle   func(t *testing.T, b *Bot, userID int64)
	weekView bool
}

func TestMessageTextsGolden(t *testing.T) {
	got := renderMessages(t)
	path := filepath.Join("testdata", "messages.golden")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v (run go test ./internal/telegram -update)", err)
	}
	wantText := normalizeLineEndings(string(want))
	if got != wantText {
		t.Errorf("bot message texts changed; review the diff and run:\n  go test ./internal/telegram -update\n\n%s",
			firstDifference(wantText, got))
	}
}

func TestMessageGoldenLeavesNoRawDates(t *testing.T) {
	for _, scenario := range messageScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			rendered := renderScenario(t, scenario)
			if match := rawDateRe.FindString(rendered); match != "" {
				t.Errorf("a raw date or week number %q reached the golden output, the check would fail on another day:\n%s", match, rendered)
			}
		})
	}
}

func renderMessages(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	for _, scenario := range messageScenarios() {
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

	const userID int64 = 5150

	caller := &recordingCaller{}
	bot, repo := setupE2EBotWithCaller(t, caller, 4242)
	seedGoldenCache(t, bot)

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeStudent
	chat.Group = dayGroup
	chat.ShowHints = false
	chat.ShowParserTime = false
	chat.HidePastDays = false
	chat.Formatter = 0
	if scenario.chat != nil {
		scenario.chat(chat)
	}
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}
	if scenario.setup != nil {
		scenario.setup(bot)
	}

	caller.reset()
	scenario.handle(t, bot, userID)

	text, buttons := lastDeliveredMessage(t, caller)
	if strings.TrimSpace(text) == "" {
		t.Fatal("nothing was delivered to the user")
	}

	now := time.Now()
	rendered := testgolden.Normalize(text, now)
	if scenario.weekView {
		rendered = testgolden.NormalizeWeek(text, now)
	}
	if len(buttons) > 0 {
		rendered += "\n--- buttons ---\n" + strings.Join(buttons, "\n")
	}
	return rendered
}

func seedGoldenCache(t *testing.T, b *Bot) {
	t.Helper()

	week := testgolden.RelevantWeek(time.Now())
	var weekDates []string
	weekDates = append(weekDates, testgolden.WeekDates(week, 6)...)
	weekDates = append(weekDates, testgolden.WeekDates(week.Next(), 6)...)

	dayDates := []string{
		time.Now().AddDate(0, 0, 1).Format("02.01.2006"),
		time.Now().AddDate(0, 0, 2).Format("02.01.2006"),
		time.Now().AddDate(0, 0, 3).Format("02.01.2006"),
	}

	b.cache.SetGroups(map[string]any{
		weekGroup: e2eJsonRoundTrip(map[string]any{"group": weekGroup, "days": goldenDays(weekDates, 3)}),
		dayGroup:  e2eJsonRoundTrip(map[string]any{"group": dayGroup, "days": goldenDays(dayDates, 3)}),
	}, "golden-groups")

	b.cache.SetTeachers(map[string]any{
		weekTeacher: e2eJsonRoundTrip(map[string]any{"teacher": weekTeacher, "days": teacherDays(weekDates, 1)}),
		dayTeacher:  e2eJsonRoundTrip(map[string]any{"teacher": dayTeacher, "days": teacherDays(dayDates[:1], 1)}),
	}, "golden-teachers")
}

func goldenDays(dates []string, lessonCount int) []any {
	days := make([]any, 0, len(dates))
	for i, date := range dates {
		lessons := make([]any, 0, lessonCount)
		for j := 0; j < lessonCount; j++ {
			lessons = append(lessons, map[string]any{
				"lesson":  lessonName(i + j),
				"type":    lessonTypes[j%len(lessonTypes)],
				"teacher": "Иванов И.И.",
				"cabinet": cabinets[j%len(cabinets)],
			})
		}
		days = append(days, map[string]any{"day": date, "lessons": lessons})
	}
	return days
}

func teacherDays(dates []string, lessonCount int) []any {
	days := make([]any, 0, len(dates))
	for i, date := range dates {
		lessons := make([]any, 0, lessonCount)
		for j := 0; j < lessonCount; j++ {
			lessons = append(lessons, map[string]any{
				"lesson":  lessonName(i + j),
				"type":    lessonTypes[j%len(lessonTypes)],
				"group":   weekGroup,
				"cabinet": cabinets[j%len(cabinets)],
			})
		}
		days = append(days, map[string]any{"day": date, "lessons": lessons})
	}
	return days
}

var lessonNames = []string{"Математика", "Информатика", "Английский язык", "География", "Химия", "Биология", "Литература"}

var lessonTypes = []string{"Лек", "ЛР", "Пр"}

var cabinets = []string{"101", "202", "303"}

func lessonName(index int) string {
	return lessonNames[index%len(lessonNames)]
}

func lastDeliveredMessage(t *testing.T, caller *recordingCaller) (string, []string) {
	t.Helper()

	caller.mu.Lock()
	defer caller.mu.Unlock()

	for i := len(caller.calls) - 1; i >= 0; i-- {
		call := caller.calls[i]
		switch call.Method {
		case "sendMessage", "editMessageText":
			return call.Text, renderReplyMarkup(call.Raw)
		}
	}
	return "", nil
}

func renderReplyMarkup(raw string) []string {
	var payload struct {
		ReplyMarkup struct {
			InlineKeyboard [][]struct {
				Text         string `json:"text"`
				CallbackData string `json:"callback_data"`
			} `json:"inline_keyboard"`
			Keyboard [][]struct {
				Text string `json:"text"`
			} `json:"keyboard"`
		} `json:"reply_markup"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	var lines []string
	for _, row := range payload.ReplyMarkup.InlineKeyboard {
		cells := make([]string, 0, len(row))
		for _, button := range row {
			cells = append(cells, button.Text+" -> "+testgolden.NormalizeWeekNumber(button.CallbackData, testgolden.RelevantWeek(time.Now()).Value()))
		}
		lines = append(lines, strings.Join(cells, " | "))
	}
	for _, row := range payload.ReplyMarkup.Keyboard {
		cells := make([]string, 0, len(row))
		for _, button := range row {
			cells = append(cells, button.Text)
		}
		lines = append(lines, strings.Join(cells, " | "))
	}
	return lines
}

func messageScenarios() []messageScenario {
	day := func(t *testing.T, b *Bot, userID int64) {
		t.Helper()
		cmd := &dayCmd{bot: b}
		if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/day"}); err != nil {
			t.Fatal(err)
		}
	}
	week := func(t *testing.T, b *Bot, userID int64) {
		t.Helper()
		cmd := &weekCmd{bot: b}
		if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/week"}); err != nil {
			t.Fatal(err)
		}
	}
	calls := func(t *testing.T, b *Bot, userID int64) {
		t.Helper()
		cmd := &callsCmd{bot: b}
		if err := cmd.Handler(context.Background(), &Update{Bot: b, ChatID: userID, UserID: userID, Text: "/calls"}); err != nil {
			t.Fatal(err)
		}
	}
	callsFull := func(t *testing.T, b *Bot, userID int64) {
		t.Helper()
		chat, err := b.chatRepo.FindOrCreate("telegram", userID)
		if err != nil {
			t.Fatal(err)
		}
		b.showCallsFullFull(&Update{Bot: b, ChatID: userID, UserID: userID}, chat)
	}
	manualCalls := func(b *Bot) {
		b.cache.SetCallsManualNotify(
			[][2][2]string{{{"10:00", "10:30"}, {"10:40", "11:10"}}},
			[][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
			"Сокращённый день", false)
	}

	weekStudent := func(chat *Chat) {
		chat.Group = weekGroup
	}
	teacher := func(chat *Chat) {
		chat.Mode = ModeTeacher
		chat.Group = ""
		chat.Teacher = dayTeacher
	}
	weekTeacherChat := func(chat *Chat) {
		chat.Mode = ModeTeacher
		chat.Group = ""
		chat.Teacher = weekTeacher
	}
	withoutGroup := func(chat *Chat) {
		chat.Group = ""
	}
	guest := func(chat *Chat) {
		chat.Mode = ""
	}
	unknownGroup := func(chat *Chat) {
		chat.Group = "404"
	}
	parserError := func(b *Bot) {
		b.cache.SetSuccessUpdate(false)
	}
	formatter := func(index int) func(chat *Chat) {
		return func(chat *Chat) {
			chat.Formatter = index
		}
	}
	showParserTime := func(chat *Chat) {
		chat.ShowParserTime = true
	}

	return []messageScenario{
		{name: "day/student/default", handle: day},
		{name: "day/student/visual", chat: formatter(1), handle: day},
		{name: "day/student/compact", chat: formatter(2), handle: day},
		{name: "day/student/litolax", chat: formatter(3), handle: day},
		{name: "day/student/parser-error", setup: parserError, handle: day},
		{name: "day/student/parser-time", chat: showParserTime, handle: day},
		{name: "day/teacher/default", chat: teacher, handle: day},
		{name: "day/guest", chat: guest, handle: day},
		{name: "day/unknown-group", chat: unknownGroup, handle: day},
		{name: "week/student", chat: weekStudent, handle: week, weekView: true},
		{name: "week/teacher", chat: weekTeacherChat, handle: week, weekView: true},
		{name: "calls/short", chat: withoutGroup, handle: calls},
		{name: "calls/student/full", chat: weekStudent, handle: callsFull},
		{name: "calls/manual-source", setup: manualCalls, chat: withoutGroup, handle: calls},
	}
}
