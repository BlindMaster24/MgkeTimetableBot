package telegram

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestCompareGroupsPrintsPairsSubjectsAndWindows(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Timetable.Weekdays = [][2][2]string{
		{{"09:00", "09:45"}, {"09:55", "10:40"}},
		{{"10:50", "11:35"}, {"11:55", "12:40"}},
		{{"13:00", "13:45"}, {"13:55", "14:40"}},
	}

	groups := b.cache.GetGroups()
	groups["101"] = map[string]any{"group": "101", "days": []any{}}
	b.cache.SetGroups(groups, "gHash")

	week := b.relevantWeekIndex()
	first, _ := week.WeekRange()

	lessonsA := []any{
		map[string]any{"lesson": "Математика", "type": "Лек", "cabinet": "101", "teacher": "Иванов И.И."},
		map[string]any{"lesson": "Физика", "type": "Пр", "cabinet": "202"},
	}
	lessonsB := []any{
		map[string]any{"lesson": "математика", "type": "Пр", "cabinet": "303"},
		map[string]any{"lesson": "Химия", "type": "Лек", "cabinet": "404"},
	}

	seedArchivedLessons(t, archiveRepo, "100", first, lessonsA)
	seedArchivedLessons(t, archiveRepo, "101", first, lessonsB)

	pusher := newMessagePusher(b, userID)
	pusher.send("/comparegroups")
	if text := caller.last(); !strings.Contains(text, "Введите номер первой группы") {
		t.Fatalf("the first prompt is missing: %q", text)
	}

	pusher.send("100")
	if text := caller.last(); !strings.Contains(text, "Введите номер второй группы") {
		t.Fatalf("the second prompt is missing: %q", text)
	}

	pusher.send("101")
	text := caller.last()

	for _, want := range []string{
		"Сравнение групп 100 и 101",
		"Учебная неделя №",
		"__ ",
		"Совпадающие пары: 1",
		"1) Математика Лек {101} - Иванов И.И. | математика Пр {303} — совпадает",
		"Общие предметы: 1) Математика",
		"Общие окна: 3 (13:00-14:40)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the comparison is missing %q:\n%s", want, text)
		}
	}
}

func seedArchivedLessons(t *testing.T, repo *archive.Repository, group string, date time.Time, lessons []any) {
	t.Helper()

	payload, err := json.Marshal(lessons)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.DB().Exec(
		`INSERT INTO timetable_archive (day, "group", data) VALUES (?, ?, ?)
		 ON CONFLICT(day, "group") DO UPDATE SET data = excluded.data`,
		int64(utils.DayIndexFromDate(date)), group, string(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompareGroupsRejectsTheSameGroup(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	week := b.relevantWeekIndex()
	first, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", first, groupLesson("Математика"))

	pusher := newMessagePusher(b, userID)
	pusher.send("/comparegroups")
	pusher.send("100")
	pusher.send("100")

	if text := caller.last(); !strings.Contains(text, "Выберите две разные группы") {
		t.Fatalf("the same group should be rejected, got %q", text)
	}
}

func TestCompareGroupsValidatesTheGroupNumber(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	pusher := newMessagePusher(b, userID)

	cases := []struct {
		input string
		want  string
	}{
		{"абв", "Это не число"},
		{"12345", "Номер группы введён неверно"},
		{"999", "Данной учебной группы не существует"},
	}

	for _, c := range cases {
		pusher.send("/comparegroups")
		caller.reset()
		pusher.send(c.input)
		if text := caller.last(); !strings.Contains(text, c.want) {
			t.Errorf("%q → %q, want it to mention %q", c.input, text, c.want)
		}
	}
}

func TestFindGroupTrimsTheAsterisks(t *testing.T) {
	b, _, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	u := makeUpdate(userID, "100**")
	u.Bot = b

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}

	group, ok := b.findGroup(u, chat, "100**")
	if !ok || group != "100" {
		t.Errorf("findGroup(100**) = %q, %v", group, ok)
	}

	if _, ok := b.findGroup(u, chat, ""); ok {
		t.Error("an empty group must be rejected")
	}
}

func TestCompareGroupsKeepsTheWeekInsideTheRange(t *testing.T) {
	b, _, archiveRepo, _ := setupArchiveBot(t, ModeStudent, "100", "")

	week := b.relevantWeekIndex()
	first, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", first, groupLesson("Математика"))
	seedArchivedDay(t, archiveRepo, "group", "101", first.AddDate(0, 0, 6), groupLesson("Химия"))

	text := b.compareGroupsMessage("100", "101")
	if !strings.Contains(text, "Сравнение групп 100 и 101") {
		t.Fatalf("the header is missing: %q", text)
	}
	if strings.Count(text, "__ ") != 6 {
		t.Errorf("the comparison must cover exactly six weekdays, got:\n%s", text)
	}
}
