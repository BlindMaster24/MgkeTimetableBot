package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestCabinetMatchesTheNumberInsideTheRoomName(t *testing.T) {
	cases := []struct {
		cabinet string
		query   string
		want    bool
	}{
		{"101", "101", true},
		{"101а", "101", true},
		{"101", "101а", true},
		{"101-а", "101", true},
		{"202", "101", false},
		{"Спортзал", "спортзал", true},
		{"Спортзал", "актовый зал", false},
	}

	for _, c := range cases {
		if got := cabinetMatches(c.cabinet, c.query); got != c.want {
			t.Errorf("cabinetMatches(%q, %q) = %v, want %v", c.cabinet, c.query, got, c.want)
		}
	}
}

func TestCabinetCommandListsLessonsInOrder(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	teacher := e2eJsonRoundTrip(map[string]any{
		"teacher": "Иванов И.И.",
		"days": []any{
			map[string]any{"day": "02.09.2026", "lessons": []any{
				map[string]any{"lesson": "Физика", "type": "Пр", "group": "101", "cabinet": "101а"},
			}},
			map[string]any{"day": "01.09.2026", "lessons": []any{
				map[string]any{"lesson": "Математика", "type": "Лек", "group": "100", "cabinet": "101"},
			}},
		},
	})
	b.cache.SetTeachers(map[string]any{"Иванов И.И.": teacher}, "tHash")

	u := makeUpdate(4242, "/cabinet 101")
	u.Bot = b
	if err := (&getCabinetCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	if !strings.Contains(text, "Кабинет: 101") {
		t.Fatalf("the room header is missing: %q", text)
	}
	if !strings.Contains(text, "Кабинет: 101а") {
		t.Fatalf("the query must also find the room suffix: %q", text)
	}

	firstDay := strings.Index(text, "Вторник, 01.09.2026")
	secondDay := strings.Index(text, "Среда, 02.09.2026")
	if firstDay == -1 || secondDay == -1 || firstDay > secondDay {
		t.Errorf("the days must be ordered by date: %q", text)
	}

	room101 := strings.Index(text, "Кабинет: 101\n")
	room101a := strings.Index(text, "Кабинет: 101а")
	if room101 == -1 || room101a == -1 || room101 > room101a {
		t.Errorf("the rooms must be listed in a stable order: %q", text)
	}
}

func groupWeek(group string, days ...any) map[string]any {
	return e2eJsonRoundTrip(map[string]any{"group": group, "days": days}).(map[string]any)
}

func groupDay(date string, lessons int) map[string]any {
	items := make([]any, 0, lessons)
	for i := 0; i < lessons; i++ {
		items = append(items, map[string]any{"lesson": "Предмет", "type": "Лек"})
	}
	return map[string]any{"day": date, "lessons": items}
}

func TestEndingsCountsOnlyTheNearestDayOfEachGroup(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")

	now := time.Now()
	past := now.AddDate(0, 0, -3).Format("02.01.2006")
	next := now.AddDate(0, 0, 1).Format("02.01.2006")
	next2 := now.AddDate(0, 0, 2).Format("02.01.2006")

	groups := map[string]any{
		"100": groupWeek("100", groupDay(past, 9), groupDay(next, 2)),
		"200": groupWeek("200", groupDay(next, 2), groupDay(next2, 5)),
		"300": groupWeek("300", groupDay(next2, 5)),
	}
	b.cache.SetGroups(groups, "gHash")

	for i := 0; i < 5; i++ {
		caller.reset()
		u := makeUpdate(4242, "/endings")
		u.Bot = b
		if err := (&endingsCmd{bot: b}).Handler(context.Background(), u); err != nil {
			t.Fatal(err)
		}

		text := caller.last()
		first := strings.Index(text, "__ "+next+" __")
		second := strings.Index(text, "__ "+next2+" __")
		if first == -1 || second == -1 || first > second {
			t.Fatalf("the days must be ordered by date: %q", text)
		}
		if !strings.Contains(text, "2 групп заканчивают к 2 паре") {
			t.Fatalf("the groups ending together are missing: %q", text)
		}
		if !strings.Contains(text, "1 групп заканчивают к 5 паре") {
			t.Fatalf("the ending of the last group is missing: %q", text)
		}
		if strings.Contains(text, past) || strings.Contains(text, "9 паре") {
			t.Fatalf("the past days must be skipped: %q", text)
		}
	}
}

func TestBrovkaExportsAWindows1251Csv(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeTeacher, "", "Иванов И.И.")

	week := utils.WeekIndexFromNumber(utils.WeekIndexFromDate(time.Now()).Value())
	start, _ := week.WeekRange()
	seedArchivedDay(t, archiveRepo, "teacher", "Иванов И.И.", start, map[string]any{
		"lesson": "Матем", "type": "лек", "group": "100", "cabinet": "101",
	})

	u := makeUpdate(userID, "/vychetkaDlyaBrovkiDSOnline "+start.Format("02.01.2006"))
	u.Bot = b
	if err := (&brovkaCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	payload := caller.documentPayload()
	if payload == "" {
		t.Fatalf("the document must be delivered, got %q docs=%d", caller.last(), len(caller.documents))
	}

	header := []byte{0xC4, 0xE5, 0xED, 0xFC, 0x2C, 0xCF, 0xEE, 0xE4, 0xE3, 0xF0, 0xF3, 0xEF, 0xEF, 0xE0}
	if !strings.Contains(payload, string(header)) {
		t.Error("the csv header must be encoded in windows-1251")
	}
	if !strings.Contains(payload, start.Format("02.01")) {
		t.Error("the day must be written without the year")
	}
}

func TestBrovkaCsvUsesTheTeacherFields(t *testing.T) {
	typeValue := "лек"
	subgroup := 2
	lines := brovkaCSV([]model.TeacherDay{{
		Day: "01.09.2026",
		Lessons: []model.TeacherLesson{
			{Lesson: "Матем", Type: &typeValue, Group: "100", Subgroup: &subgroup},
			nil,
		},
	}})

	want := []string{
		"День,Подгруппа,Группа,Тип,Предмет",
		"01.09,2,100,ЛЕК," + utils.GetFullSubjectName("Матем"),
	}
	if len(lines) != len(want) {
		t.Fatalf("unexpected csv lines: %#v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}
