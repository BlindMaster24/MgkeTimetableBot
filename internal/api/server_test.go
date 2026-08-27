package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/gin-gonic/gin"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	c, err := cache.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.SetGroups(map[string]any{
		"63": map[string]any{"group": "63", "days": []any{}},
		"64": map[string]any{"group": "64", "days": []any{}},
	}, "hash1")
	c.SetTeachers(map[string]any{
		"Иванов": map[string]any{"teacher": "Иванов"},
	}, "hash2")
	gin.SetMode(gin.TestMode)
	return NewServer(c, 0)
}

func TestHandleInfo(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/info", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["name"] != "MgkeTimetableBot API" {
		t.Errorf("expected name MgkeTimetableBot API, got %s", body["name"])
	}
	if body["version"] != "2.0" {
		t.Errorf("expected version 2.0, got %s", body["version"])
	}
}

func TestHandleGroups(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/groups", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string][]string
	json.Unmarshal(w.Body.Bytes(), &body)
	groups := body["groups"]
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestHandleTeachers(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teachers", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string][]string
	json.Unmarshal(w.Body.Bytes(), &body)
	teachers := body["teachers"]
	if len(teachers) != 1 {
		t.Errorf("expected 1 teacher, got %d", len(teachers))
	}
}

func TestHandleGroupByNameFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/group/63", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleGroupByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/group/999", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTeacherByNameFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teacher/Иванов", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleTeacherByNameNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/teacher/Несуществующий", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleParserHealth(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/parser-health", nil)
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body cache.Stats
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.GroupsCount != 2 {
		t.Errorf("expected 2 groups in stats, got %d", body.GroupsCount)
	}
	if body.TeachersCount != 1 {
		t.Errorf("expected 1 teacher in stats, got %d", body.TeachersCount)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{8080, "8080"},
		{300, "300"},
		{9999, "9999"},
	}
	for _, c := range cases {
		got := itoa(c.n)
		if got != c.want {
			t.Errorf("itoa(%d) = %s, want %s", c.n, got, c.want)
		}
	}
}
