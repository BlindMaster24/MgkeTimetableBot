package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestInfoCarriesTSCacheEnvelope(t *testing.T) {
	srv := setupTestServer(t)
	srv.cache.SetGroups(map[string]any{"63": map[string]any{"days": []any{}}}, "groups-hash")
	srv.cache.SetTeachers(map[string]any{"T": map[string]any{"days": []any{}}}, "teachers-hash")
	srv.cache.SetTeam(map[string]string{"A": "A Full"}, []string{"team-hash"})

	w := call(t, srv, "GET", "/api/info")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)

	groups, ok := body["groups"].(map[string]any)
	if !ok {
		t.Fatalf("groups block missing: %v", body)
	}
	if groups["hash"] != "groups-hash" {
		t.Errorf("groups.hash = %v", groups["hash"])
	}
	if groups["update"] == nil || groups["changed"] == nil || groups["lastWeekIndex"] == nil {
		t.Errorf("groups block incomplete: %v", groups)
	}

	teachers, ok := body["teachers"].(map[string]any)
	if !ok || teachers["hash"] != "teachers-hash" {
		t.Errorf("teachers block = %v", body["teachers"])
	}

	team, ok := body["team"].(map[string]any)
	if !ok {
		t.Fatalf("team block missing: %v", body)
	}
	hashes, _ := team["hash"].([]any)
	if len(hashes) != 1 || hashes[0] != "team-hash" {
		t.Errorf("team.hash = %v", team["hash"])
	}
	if _, exists := team["lastWeekIndex"]; exists {
		t.Errorf("the TS team block has no lastWeekIndex: %v", team)
	}

	if body["lastSuccess"] != true {
		t.Errorf("lastSuccess = %v", body["lastSuccess"])
	}
}

func TestInfoCarriesTheWebhookStatusWhenWired(t *testing.T) {
	srv := setupTestServer(t)

	if body := decodeBody(t, call(t, srv, "GET", "/api/info")); body["webhook"] != nil {
		t.Fatalf("webhook must be absent while it is not wired: %v", body["webhook"])
	}

	srv.SetWebhookStatus(func(_ context.Context) any {
		return map[string]any{
			"enabled":        true,
			"mode":           "webhook",
			"endpoint":       "https://mgke.example.com/telegram/webhook",
			"pendingUpdates": 7,
			"lastError":      "Wrong response from the webhook: 500 Internal Server Error",
		}
	})

	body := decodeBody(t, call(t, srv, "GET", "/api/info"))
	webhook, ok := body["webhook"].(map[string]any)
	if !ok {
		t.Fatalf("webhook block missing: %v", body)
	}
	for key, want := range map[string]any{
		"mode":           "webhook",
		"endpoint":       "https://mgke.example.com/telegram/webhook",
		"pendingUpdates": float64(7),
	} {
		if webhook[key] != want {
			t.Errorf("webhook.%s = %v, want %v", key, webhook[key], want)
		}
	}
	if webhook["lastError"] == "" || webhook["lastError"] == nil {
		t.Errorf("webhook.lastError = %v", webhook["lastError"])
	}
}

func TestGroupsSortedNumericAware(t *testing.T) {
	srv := setupTestServer(t)
	srv.cache.SetGroups(map[string]any{
		"100": map[string]any{"days": []any{}},
		"99":  map[string]any{"days": []any{}},
		"ПСМ": map[string]any{"days": []any{}},
		"10":  map[string]any{"days": []any{}},
	}, "h")

	w := call(t, srv, "GET", "/api/groups")
	body := decodeBody(t, w)
	got := body["groups"].([]any)
	want := []any{"10", "63", "64", "99", "100", "ПСМ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
}

func TestGroupByNameEnvelope(t *testing.T) {
	srv := setupTestServer(t)
	srv.cache.SetGroups(map[string]any{
		"100": map[string]any{"days": []any{map[string]any{"day": "01.09.2026", "lessons": []any{}}}},
	}, "h")

	w := call(t, srv, "GET", "/api/group/100")
	body := decodeBody(t, w)
	days, _ := body["days"].([]any)
	if len(days) != 1 {
		t.Fatalf("days = %v", body["days"])
	}
	day, _ := days[0].(map[string]any)
	if day["weekday"] != "Вторник" {
		t.Errorf("weekday = %v, want Вторник", day["weekday"])
	}
	if _, ok := day["lessons"]; !ok {
		t.Errorf("original keys lost: %v", day)
	}
	if body["update"] == nil || body["changed"] == nil {
		t.Errorf("entry envelope incomplete: %v", body)
	}
	if body["lastSuccess"] != true {
		t.Errorf("lastSuccess = %v", body["lastSuccess"])
	}

	srv.cache.SetSuccessUpdate(false)
	w = call(t, srv, "GET", "/api/group/100")
	body = decodeBody(t, w)
	if body["lastSuccess"] != false {
		t.Errorf("lastSuccess must follow the parse flag, got %v", body["lastSuccess"])
	}
}

func TestGroupByNameMissingIs404(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/group/missing")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestTeacherByNameDaysNullWhenEntryLacksDays(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/teacher/Иванов")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["days"] != nil {
		t.Errorf("days = %v, want null like the TS || null fallback", body["days"])
	}
	if body["lastSuccess"] == nil {
		t.Errorf("entry envelope incomplete: %v", body)
	}
}

func TestParserHealthEnvelope(t *testing.T) {
	srv := setupTestServer(t)
	w := call(t, srv, "GET", "/api/parser-health")
	body := decodeBody(t, w)

	if body["ok"] != true {
		t.Errorf("ok = %v", body["ok"])
	}
	if body["lastSuccessUpdate"] == nil {
		t.Errorf("lastSuccessUpdate missing: %v", body)
	}

	groups, _ := body["groups"].(map[string]any)
	if groups["hash"] == nil || groups["update"] == nil || groups["changed"] == nil {
		t.Errorf("groups block incomplete: %v", groups)
	}
	if _, exists := groups["lastWeekIndex"]; exists {
		t.Errorf("the TS parser-health groups block has no lastWeekIndex: %v", groups)
	}

	metrics, _ := body["metrics"].(map[string]any)
	if metrics["student"] != nil || metrics["teacher"] != nil {
		t.Errorf("metrics = %v, want null entries without the v2 parser", metrics)
	}

	cacheBlock, _ := body["cache"].(map[string]any)
	if cacheBlock["hits"] == nil || cacheBlock["misses"] == nil {
		t.Errorf("cache counters missing: %v", cacheBlock)
	}

	srv.cache.SetSuccessUpdate(false)
	w = call(t, srv, "GET", "/api/parser-health")
	body = decodeBody(t, w)
	if body["ok"] != false {
		t.Errorf("ok must follow the parse flag, got %v", body["ok"])
	}
	if body["lastSuccessUpdate"] != float64(0) {
		t.Errorf("lastSuccessUpdate = %v, want 0 after a failed parse", body["lastSuccessUpdate"])
	}
}
