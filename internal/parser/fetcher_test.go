package parser

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const testGroupHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание</h1>
<h2>Группа - 63ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 25.08.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td class="sub">101</td></tr>
</table>
<h2>Группа - 64ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 25.08.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Физика<br>(Пр)<br>Петров</td><td class="sub">102</td></tr>
</table>
</div></div>
</body></html>`

const testTeacherHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание</h1>
<h2>Преподаватель - Иванов И.И.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, 25.08.2025</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>63ТП<br>Математика<br>(Лек)</td><td class="sub">101</td></tr>
</table>
</div></div>
</body></html>`

func TestFetchAndParseGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(testGroupHTML))
	}))
	defer srv.Close()

	log := logger.New("error", nil)
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = FetchAndParse(log, c, srv.URL, srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	groups := c.GetGroups()
	if len(groups) == 0 {
		t.Fatal("expected parsed groups in cache")
	}

	if _, ok := groups["63ТП"]; !ok {
		t.Error("expected group 63ТП in cache")
	}
	if _, ok := groups["64ТП"]; !ok {
		t.Error("expected group 64ТП in cache")
	}
}

func TestFetchAndParseTeachers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(testTeacherHTML))
	}))
	defer srv.Close()

	log := logger.New("error", nil)
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = FetchAndParse(log, c, srv.URL, srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	teachers := c.GetTeachers()
	if len(teachers) == 0 {
		t.Fatal("expected parsed teachers in cache")
	}

	if _, ok := teachers["Иванов И.И."]; !ok {
		t.Error("expected teacher Иванов И.И. in cache")
	}
}

func TestFetchHTMLStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	_, err := fetchHTML(&http.Client{}, srv.URL)
	if err == nil {
		t.Error("expected error for 403 status")
	}
}

func TestFetchHTMLConnectionRefused(t *testing.T) {
	_, err := fetchHTML(&http.Client{}, "http://127.0.0.1:1")
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestFetchAndParseEmptyResponse(t *testing.T) {
	html := `<html><body></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	log := logger.New("error", nil)
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = FetchAndParse(log, c, srv.URL, srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	groups := c.GetGroups()
	teachers := c.GetTeachers()
	if len(groups) != 0 || len(teachers) != 0 {
		t.Errorf("expected empty results, got groups=%d teachers=%d", len(groups), len(teachers))
	}
}

func TestUserAgent(t *testing.T) {
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	fetchHTML(&http.Client{}, srv.URL)

	if capturedUA == "" {
		t.Error("expected User-Agent header")
	}
}
