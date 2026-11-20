package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestStartParserLoadsTheInitialDump(t *testing.T) {
	page := entriesPage("Группа", 3)
	teacherPage := entriesPage("Преподаватель", 2)
	site := stubSite(t, page, teacherPage)

	r := newParseRuntime(t)
	r.cfg.Parser.Enabled = true
	r.cfg.Parser.Endpoints.TimetableGroup = site + "/groups"
	r.cfg.Parser.Endpoints.TimetableTeacher = site + "/teachers"
	r.cfg.Parser.Endpoints.BellSchedule = site + "/calls"
	r.cfg.Parser.Endpoints.Team = []string{site + "/team"}
	r.cfg.Parser.Guard = config.GuardConfig{MinItems: 1, MaxDropPercent: 100}

	archiveRepo, err := archive.New(t.TempDir() + "/archive.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archiveRepo.Close() })
	r.archive = archiveRepo

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	r.startParser(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(r.cache.GetGroups()) == 3 && len(r.cache.GetTeachers()) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := len(r.cache.GetGroups()); got != 3 {
		t.Fatalf("groups = %d, want the initial parse to seed 3", got)
	}
	if got := len(r.cache.GetTeachers()); got != 2 {
		t.Fatalf("teachers = %d, want 2", got)
	}
	if !r.cache.Stats().SuccessUpdate {
		t.Fatal("a healthy initial parse must mark the cache updated")
	}
}

// stubSite serves the entry pages the fetcher expects and a friendly Bot API.
func stubSite(t *testing.T, groupPage, teacherPage string) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			_, _ = w.Write([]byte(groupPage))
		case strings.HasSuffix(r.URL.Path, "/teachers"):
			_, _ = w.Write([]byte(teacherPage))
		default:
			// Calls and team pages: an empty but valid document.
			_, _ = w.Write([]byte(`<html><body><table><tr><td>1</td><td>08:00</td><td>08:45</td></tr></table></body></html>`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
