package telegram

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego/telegoapi"
)

type capturedMessage struct {
	chatID int64
	text   string
}

type capturingCaller struct {
	mu       sync.Mutex
	messages []capturedMessage
}

func (c *capturingCaller) Call(ctx context.Context, url string, data *telegoapi.RequestData) (*telegoapi.Response, error) {
	if strings.HasSuffix(url, "/sendMessage") {
		var payload struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.Unmarshal(data.BodyRaw, &payload); err == nil {
			c.mu.Lock()
			c.messages = append(c.messages, capturedMessage{chatID: payload.ChatID, text: payload.Text})
			c.mu.Unlock()
		}
	}

	return &telegoapi.Response{Ok: true, Result: []byte(`{"message_id": 1}`)}, nil
}

func (c *capturingCaller) last() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return ""
	}
	return c.messages[len(c.messages)-1].text
}

func (c *capturingCaller) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = nil
}

func setupArchiveBot(t *testing.T, mode ChatMode, group, teacher string) (*Bot, *capturingCaller, *archive.Repository, int64) {
	t.Helper()

	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)

	archiveRepo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archiveRepo.Close() })
	b.archive = archiveRepo

	userID := int64(6200)
	writeChat(t, repo, userID, mode, group, teacher)

	return b, caller, archiveRepo, userID
}

func writeChat(t *testing.T, repo *Repository, userID int64, mode ChatMode, group, teacher string) {
	t.Helper()

	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = mode
	chat.Group = group
	chat.Teacher = teacher
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}
}

func seedArchivedDay(t *testing.T, repo *archive.Repository, kind, value string, date time.Time, lesson map[string]any) {
	t.Helper()

	lessons, err := json.Marshal([]any{lesson})
	if err != nil {
		t.Fatal(err)
	}

	column := `"group"`
	if kind == "teacher" {
		column = "teacher"
	}

	_, err = repo.DB().Exec(
		`INSERT INTO timetable_archive (day, `+column+`, data) VALUES (?, ?, ?)
		 ON CONFLICT(day, `+column+`) DO UPDATE SET data = excluded.data`,
		int64(utils.DayIndexFromDate(date)), value, string(lessons),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func groupLesson(lesson string) map[string]any {
	return map[string]any{"lesson": lesson, "type": "Лек", "teacher": "Иванов И.И.", "cabinet": "101"}
}

func runArchive(t *testing.T, b *Bot, userID int64, text string) {
	t.Helper()

	u := makeUpdate(userID, text)
	u.Bot = b
	if err := (&archiveCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCommandRendersAWeek(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	weekNumber := utils.WeekIndexFromDate(time.Now()).Value() - 3
	week := utils.WeekIndexFromNumber(weekNumber)
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", start, groupLesson("АрхивнаяМатематика"))

	runArchive(t, b, userID, "/archive week "+strconv.Itoa(weekNumber))

	text := caller.last()
	if !strings.Contains(text, "АрхивнаяМатематика") {
		t.Fatalf("the archived lesson is missing from the answer: %q", text)
	}
	if !strings.Contains(text, start.Format("02.01.2006")) {
		t.Errorf("the answer should carry the archived day: %q", text)
	}
}

func TestArchiveCommandRendersASingleDay(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	day, _ := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(time.Now()).Value() - 2).WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", day, groupLesson("ДеньИзАрхива"))

	runArchive(t, b, userID, "/archive "+day.Format("02.01.2006"))

	text := caller.last()
	if !strings.Contains(text, "ДеньИзАрхива") {
		t.Fatalf("the archived day was not rendered: %q", text)
	}
	if !strings.Contains(text, day.Format("02.01.2006")) {
		t.Errorf("the answer must carry the requested date %s: %q", day.Format("02.01.2006"), text)
	}
}

func TestArchiveCommandRendersTeacherDays(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeTeacher, "", "Иванов И.И.")

	day, _ := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(time.Now()).Value() - 2).WeekRange()
	seedArchivedDay(t, archiveRepo, "teacher", "Иванов И.И.", day, map[string]any{
		"lesson": "ПараУчителя", "type": "Лек", "group": "100", "cabinet": "101",
	})

	runArchive(t, b, userID, "/archive "+day.Format("02.01.2006"))

	if text := caller.last(); !strings.Contains(text, "ПараУчителя") {
		t.Fatalf("teacher day was not rendered: %q", text)
	}
}

func TestArchiveCommandInputErrors(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	cases := []struct {
		text string
		want string
	}{
		{"/archive", "День не указан"},
		{"/archive week 0", "Неверный номер недели"},
		{"/archive 32.13", "Неверная дата"},
		{"/archive 1", "Неверный формат"},
		{"/archive 12.02.2026.01", "Неверный формат"},
	}

	for _, c := range cases {
		caller.reset()
		runArchive(t, b, userID, c.text)
		if text := caller.last(); !strings.Contains(text, c.want) {
			t.Errorf("%q → %q, want it to mention %q", c.text, text, c.want)
		}
	}
}

func TestArchiveCommandWithoutArchive(t *testing.T) {
	caller := &capturingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	userID := int64(6300)
	writeChat(t, repo, userID, ModeStudent, "100", "")

	runArchive(t, b, userID, "/archive week 5")

	if text := caller.last(); !strings.Contains(text, "Архив недоступен") {
		t.Errorf("an unavailable archive should be reported, got %q", text)
	}
}
