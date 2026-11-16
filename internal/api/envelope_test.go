package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
)

func TestGroupsEndpointSortsNumericFirstLikeTS(t *testing.T) {
	srv := setupTestServerWith(t, health.NewDefaultTracker())
	srv.cache.SetGroups(t.Context(), map[string]any{
		"100": map[string]any{"days": []any{}},
		"99":  map[string]any{"days": []any{}},
		"ПСМ": map[string]any{"days": []any{}},
		"10":  map[string]any{"days": []any{}},
	}, "sorted")

	w := call(t, srv, "GET", "/api/groups")
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var body struct {
		Groups []string `json:"groups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := []string{"10", "63", "64", "99", "100", "ПСМ"}
	if len(body.Groups) != len(want) {
		t.Fatalf("groups = %v, want %v", body.Groups, want)
	}
	for i := range want {
		if body.Groups[i] != want[i] {
			t.Fatalf("groups = %v, want %v", body.Groups, want)
		}
	}
}

func TestWeekdayName(t *testing.T) {
	cases := map[time.Weekday]string{
		time.Monday:    "Понедельник",
		time.Tuesday:   "Вторник",
		time.Wednesday: "Среда",
		time.Thursday:  "Четверг",
		time.Friday:    "Пятница",
		time.Saturday:  "Суббота",
		time.Sunday:    "Воскресенье",
	}
	server := &Server{loc: i18n.New("ru")}
	for weekday, want := range cases {
		date := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(weekday)-int(time.Monday))
		if got, ok := server.weekdayName(date); !ok || got != want {
			t.Errorf("weekdayName(%v) = %q, want %q", weekday, got, want)
		}
	}
}

func TestDecorateDays(t *testing.T) {
	entry := map[string]any{"days": []any{map[string]any{"day": "01.09.2026", "subject": "Математика"}}}
	server := &Server{loc: i18n.New("ru")}
	days := decorateDays(entry, server.weekdayName)
	if len(days) != 1 {
		t.Fatalf("days = %v", days)
	}
	day, ok := days[0].(map[string]any)
	if !ok {
		t.Fatalf("day is not a map: %v", days[0])
	}
	if day["weekday"] != "Вторник" {
		t.Errorf("weekday = %v, want Вторник", day["weekday"])
	}
	if day["subject"] != "Математика" || day["day"] != "01.09.2026" {
		t.Errorf("original keys lost: %v", day)
	}

	override := map[string]any{"days": []any{map[string]any{"day": "01.09.2026", "weekday": "Своя"}}}
	day = decorateDays(override, server.weekdayName)[0].(map[string]any)
	if day["weekday"] != "Своя" {
		t.Errorf("the day's own weekday must win, got %v", day["weekday"])
	}

	if got := decorateDays(map[string]any{}, server.weekdayName); got != nil {
		t.Errorf("entry without days = %v, want nil", got)
	}

	raw := decorateDays(map[string]any{"days": []any{"raw", map[string]any{"day": "вчера"}}}, server.weekdayName)
	if raw[0] != "raw" {
		t.Errorf("non-map items must pass through, got %v", raw[0])
	}
	bad, ok := raw[1].(map[string]any)
	if !ok {
		t.Fatalf("bad day is not a map: %v", raw[1])
	}
	if _, exists := bad["weekday"]; exists {
		t.Errorf("unparseable date must not gain a weekday: %v", bad)
	}
}
