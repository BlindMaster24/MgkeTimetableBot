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

func spanStrings(spans []eventSpan) []string {
	out := make([]string, 0, len(spans))
	for _, span := range spans {
		out = append(out, span.start.Format(clockLayout)+".."+span.end.Format(clockLayout))
	}
	return out
}

func TestLessonTimeSpansUseBellSchedule(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)

	spans, err := lessonTimeSpans(callsSchedule(), day, 0)
	if err != nil {
		t.Fatalf("lessonTimeSpans: %v", err)
	}
	got := spanStrings(spans)
	want := []string{"08:00..08:45", "08:55..09:40"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("lesson 1 spans = %v, want %v", got, want)
	}

	spans, err = lessonTimeSpans(callsSchedule(), day, 1)
	if err != nil {
		t.Fatalf("lessonTimeSpans: %v", err)
	}
	got = spanStrings(spans)
	want = []string{"09:50..10:35", "10:45..11:30"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("lesson 2 spans = %v, want %v", got, want)
	}
}

func TestLessonTimeSpansSaturday(t *testing.T) {
	saturday := time.Date(2026, time.September, 19, 0, 0, 0, 0, moscow)
	spans, err := lessonTimeSpans(callsSchedule(), saturday, 0)
	if err != nil {
		t.Fatalf("lessonTimeSpans: %v", err)
	}
	got := spanStrings(spans)
	want := []string{"09:00..09:45", "09:55..10:40"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("saturday spans = %v, want %v", got, want)
	}
}

func TestLessonTimeSpansFallBackToLastSlot(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	spans, err := lessonTimeSpans(callsSchedule(), day, 9)
	if err != nil {
		t.Fatalf("lessonTimeSpans: %v", err)
	}
	if len(spans) == 0 || spans[0].start.Format(clockLayout) != "09:50" {
		t.Errorf("out of range lesson should use the last slot, got %v", spanStrings(spans))
	}
}

func TestLessonTimeSpansSkipEmptyBounds(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	calls := Schedule{Weekdays: [][2][2]string{{{"08:00", "10:40"}, {"", ""}}}}
	spans, err := lessonTimeSpans(calls, day, 0)
	if err != nil {
		t.Fatalf("lessonTimeSpans: %v", err)
	}
	got := spanStrings(spans)
	if len(got) != 1 || got[0] != "08:00..10:40" {
		t.Errorf("spans = %v, want one full-pair span", got)
	}
}

func TestLessonTimeSpansWithoutSchedule(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	if _, err := lessonTimeSpans(Schedule{}, day, 0); err == nil {
		t.Error("expected an error when the calls schedule is empty")
	}
}

func TestBuildEvent(t *testing.T) {
	day := time.Date(2026, time.September, 14, 0, 0, 0, 0, moscow)
	spans, _ := lessonTimeSpans(callsSchedule(), day, 0)

	event := buildEvent(DayLesson{
		Index:       0,
		Title:       "Математика",
		Description: "<b>Предмет:</b> Математика",
		Location:    "101",
	}, spans[0].start, spans[0].end)

	if event.Summary != "Математика" || event.Location != "101" {
		t.Errorf("event = %+v", event)
	}
	if event.Start.TimeZone != timeZoneName || event.End.TimeZone != timeZoneName {
		t.Errorf("timezone = %q/%q", event.Start.TimeZone, event.End.TimeZone)
	}
	if !strings.HasPrefix(event.Start.DateTime, "2026-09-14T08:00:00") {
		t.Errorf("start = %q", event.Start.DateTime)
	}
	if !strings.HasPrefix(event.End.DateTime, "2026-09-14T08:45:00") {
		t.Errorf("end = %q", event.End.DateTime)
	}
}

func TestSyncDayCreatesEventPerBound(t *testing.T) {
	type insert struct {
		calendarID string
		payload    map[string]any
	}
	var inserts []insert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events"):
			json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/events"):
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			inserts = append(inserts, insert{calendarID: r.URL.Path, payload: payload})
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": "event-new"})
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
	svc := &CalendarService{client: service}

	err = svc.SyncDay(context.Background(), "cal-1", "14.09.2026", []DayLesson{{
		Index:    0,
		Title:    "Математика",
		Location: "101",
	}}, callsSchedule())
	if err != nil {
		t.Fatalf("SyncDay: %v", err)
	}
	if len(inserts) != 2 {
		t.Fatalf("inserted %d events, want 2 (one per bound)", len(inserts))
	}
	starts := []string{}
	ends := []string{}
	for _, ins := range inserts {
		start := ins.payload["start"].(map[string]any)["dateTime"].(string)
		end := ins.payload["end"].(map[string]any)["dateTime"].(string)
		starts = append(starts, start[11:16])
		ends = append(ends, end[11:16])
	}
	if starts[0] != "08:00" || ends[0] != "08:45" || starts[1] != "08:55" || ends[1] != "09:40" {
		t.Errorf("event times = %v..%v, want 08:00..08:45 and 08:55..09:40", starts, ends)
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
