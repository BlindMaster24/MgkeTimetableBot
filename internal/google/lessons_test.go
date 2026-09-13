package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func intPtr(v int) *int { return &v }

func TestGroupLessonInfoSingleLesson(t *testing.T) {
	title, description, location := GroupLessonInfo([]GroupLesson{{
		Name:    "Математика",
		Type:    "Лек",
		Teacher: "Иванов И.И.",
		Cabinet: "101",
		Comment: "перенос",
	}})

	if title != "Математика" {
		t.Errorf("title = %q", title)
	}
	want := "<b>Предмет:</b> Математика\n<b>Вид:</b> Лек\n<b>Преподаватель:</b> Иванов И.И.\n<b>Кабинет:</b> 101\n<b>Примечание:</b> перенос"
	if description != want {
		t.Errorf("description =\n%q\nwant\n%q", description, want)
	}
	if location != "101" {
		t.Errorf("location = %q", location)
	}
}

func TestGroupLessonInfoSubgroups(t *testing.T) {
	title, description, _ := GroupLessonInfo([]GroupLesson{
		{Subgroup: intPtr(1), Name: "Физика", Cabinet: "202"},
		{Subgroup: intPtr(2), Name: "Физика", Cabinet: "203"},
	})

	if title != "1,2 - Физика" {
		t.Errorf("title = %q", title)
	}
	if !strings.Contains(description, "<i>1-я подгруппа:</i>") || !strings.Contains(description, "<i>2-я подгруппа:</i>") {
		t.Errorf("description missing subgroup headers: %q", description)
	}
}

func TestGroupLessonInfoDifferentLessons(t *testing.T) {
	title, _, _ := GroupLessonInfo([]GroupLesson{
		{Subgroup: intPtr(1), Name: "Физика", Cabinet: "202"},
		{Name: "Информатика", Cabinet: "303"},
	})

	if title != "1. Физика | Информатика" {
		t.Errorf("title = %q", title)
	}
}

func TestGroupLessonInfoMissingCabinet(t *testing.T) {
	_, description, location := GroupLessonInfo([]GroupLesson{{Name: "Математика"}})
	if !strings.Contains(description, "<b>Кабинет:</b> -") {
		t.Errorf("description = %q", description)
	}
	if location != "" {
		t.Errorf("location = %q, want empty", location)
	}
}

func TestTeacherLessonInfo(t *testing.T) {
	title, description, location := TeacherLessonInfo(TeacherLesson{
		Subgroup: intPtr(2),
		Group:    "100",
		Name:     "Физика",
		Type:     "Пр",
		Cabinet:  "202",
	})

	if title != "2. 100-Физика" {
		t.Errorf("title = %q", title)
	}
	if !strings.Contains(description, "<b>Группа:</b> 2. 100") {
		t.Errorf("description = %q", description)
	}
	if location != "202" {
		t.Errorf("location = %q", location)
	}
}

func callsSchedule() Schedule {
	return Schedule{
		Weekdays: [][2][2]string{
			{{"08:00", "08:45"}, {"08:55", "09:40"}},
			{{"09:50", "10:35"}, {"10:45", "11:30"}},
		},
		Saturday: [][2][2]string{
			{{"09:00", "09:45"}, {"09:55", "10:40"}},
		},
	}
}

func TestLessonTimesUseBellSchedule(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)

	start, end, err := lessonTimes(callsSchedule(), day, 0)
	if err != nil {
		t.Fatalf("lessonTimes: %v", err)
	}
	if start.Format(clockLayout) != "08:00" || end.Format(clockLayout) != "09:40" {
		t.Errorf("lesson 1 = %s..%s", start.Format(clockLayout), end.Format(clockLayout))
	}

	start, end, err = lessonTimes(callsSchedule(), day, 1)
	if err != nil {
		t.Fatalf("lessonTimes: %v", err)
	}
	if start.Format(clockLayout) != "09:50" || end.Format(clockLayout) != "11:30" {
		t.Errorf("lesson 2 = %s..%s", start.Format(clockLayout), end.Format(clockLayout))
	}
}

func TestLessonTimesSaturday(t *testing.T) {
	saturday := time.Date(2026, time.September, 19, 0, 0, 0, 0, moscow)
	start, end, err := lessonTimes(callsSchedule(), saturday, 0)
	if err != nil {
		t.Fatalf("lessonTimes: %v", err)
	}
	if start.Format(clockLayout) != "09:00" || end.Format(clockLayout) != "10:40" {
		t.Errorf("saturday lesson = %s..%s", start.Format(clockLayout), end.Format(clockLayout))
	}
}

func TestLessonTimesFallBackToLastSlot(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	start, _, err := lessonTimes(callsSchedule(), day, 9)
	if err != nil {
		t.Fatalf("lessonTimes: %v", err)
	}
	if start.Format(clockLayout) != "09:50" {
		t.Errorf("out of range lesson should use the last slot, got %s", start.Format(clockLayout))
	}
}

func TestLessonTimesWithoutSchedule(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	if _, _, err := lessonTimes(Schedule{}, day, 0); err == nil {
		t.Error("expected an error when the calls schedule is empty")
	}
}

func TestBuildEvent(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	start, end, _ := lessonTimes(callsSchedule(), day, 0)

	event := buildEvent(DayLesson{
		Index:       0,
		Title:       "Математика",
		Description: "<b>Предмет:</b> Математика",
		Location:    "101",
	}, start, end)

	if event.Summary != "Математика" || event.Location != "101" {
		t.Errorf("event = %+v", event)
	}
	if event.Start.TimeZone != timeZoneName || event.End.TimeZone != timeZoneName {
		t.Errorf("timezone = %q/%q", event.Start.TimeZone, event.End.TimeZone)
	}
	if !strings.HasPrefix(event.Start.DateTime, "2026-09-14T08:00:00") {
		t.Errorf("start = %q", event.Start.DateTime)
	}
	if !strings.HasPrefix(event.End.DateTime, "2026-09-14T09:40:00") {
		t.Errorf("end = %q", event.End.DateTime)
	}
}

func TestClearDayRemovesExistingEvents(t *testing.T) {
	deleted := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events"):
			json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": "event-1"},
					map[string]any{"id": "event-2"},
				},
			})
		case r.Method == http.MethodDelete:
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(server.Client()), option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := clearDay(context.Background(), service, "cal-1", time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)); err != nil {
		t.Fatalf("clearDay: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted %d events, want 2", len(deleted))
	}
	for _, path := range deleted {
		if !strings.Contains(path, "event-1") && !strings.Contains(path, "event-2") {
			t.Errorf("unexpected delete path %q", path)
		}
	}
}
