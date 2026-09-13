package parser

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func newTestCache(t *testing.T) *cache.RaspCache {
	t.Helper()

	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return c
}

func seedGroups(t *testing.T, c *cache.RaspCache, count int) {
	t.Helper()

	groups := make(map[string]any, count)
	for i := 0; i < count; i++ {
		groups[string(rune('A'+i))] = map[string]any{
			"group": string(rune('A' + i)),
			"days":  []any{},
		}
	}
	c.SetGroups(groups, "seed")
}

func TestFetcherKeepsCacheWhenPageIsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><div class="entry"><div class="content"></div></div></body></html>`))
	}))
	defer srv.Close()

	c := newTestCache(t)
	seedGroups(t, c, 3)

	var seen []Report
	fetcher := NewFetcher(logger.New("error", nil), c, Options{
		OnReport: func(report Report) { seen = append(seen, report) },
	})

	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	if len(c.GetGroups()) != 3 {
		t.Errorf("cache was overwritten by an empty parse: %d groups", len(c.GetGroups()))
	}

	report, ok := fetcher.Reports()[SourceGroups]
	if !ok {
		t.Fatal("expected a groups report")
	}
	if !report.KeptOld {
		t.Error("expected the report to say the previous data was kept")
	}
	if report.OK() {
		t.Errorf("a kept-old report must not look clean: %+v", report)
	}
	if len(seen) == 0 {
		t.Error("expected the report callback to fire")
	}
}

func TestFetcherKeepsCacheWhenParseShrinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><div class="entry"><div class="content">
<h2>Группа - 100</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td></tr>
</table>
</div></div></body></html>`))
	}))
	defer srv.Close()

	c := newTestCache(t)
	seedGroups(t, c, 40)

	fetcher := NewFetcher(logger.New("error", nil), c, Options{})
	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	if len(c.GetGroups()) != 40 {
		t.Errorf("a shrunken parse must not replace the cache: %d groups", len(c.GetGroups()))
	}

	report := fetcher.Reports()[SourceGroups]
	if !report.KeptOld {
		t.Error("expected the report to flag the shrink")
	}
}

func TestFetcherAppliesHealthyParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><div class="entry"><div class="content">
<h2>Группа - 100</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td></tr>
</table>
<h2>Группа - 101</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Физика<br>(Пр)<br>Петров П.П.<br>2-101</td></tr>
</table>
</div></div></body></html>`))
	}))
	defer srv.Close()

	c := newTestCache(t)
	fetcher := NewFetcher(logger.New("error", nil), c, Options{})

	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	if len(c.GetGroups()) != 2 {
		t.Errorf("expected two groups in the cache, got %d", len(c.GetGroups()))
	}

	report := fetcher.Reports()[SourceGroups]
	if report.KeptOld {
		t.Errorf("a healthy parse must not be flagged: %s", report.Summary())
	}
	if report.Items != 2 {
		t.Errorf("report items = %d", report.Items)
	}
	if !report.OK() {
		t.Errorf("expected a clean report, got %s", report.Summary())
	}
}

func TestFetcherKeepsCallsWhenPageHasNoSlots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><div class="entry"><div class="content"><p>нет звонков</p></div></div></body></html>`))
	}))
	defer srv.Close()

	c := newTestCache(t)
	c.SetCallsNotify(cache.Schedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
	}, cache.Schedule{}, "site", "")

	fetcher := NewFetcher(logger.New("error", nil), c, Options{})
	if err := fetcher.Calls(srv.URL); err != nil {
		t.Fatalf("calls: %v", err)
	}

	if len(c.GetCalls().Site.Schedule.Weekdays) != 1 {
		t.Errorf("the previous bell schedule was lost: %+v", c.GetCalls().Site.Schedule)
	}

	report := fetcher.Reports()[SourceCalls]
	if !report.KeptOld {
		t.Error("expected the calls report to flag that the previous data was kept")
	}
}

func TestFetcherReportsEverySource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><div class="entry"><div class="content">
<h2>Группа - 100</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td></tr>
</table>
</div></div></body></html>`))
	}))
	defer srv.Close()

	c := newTestCache(t)
	fetcher := NewFetcher(logger.New("error", nil), c, Options{})

	_ = fetcher.Timetable(srv.URL, srv.URL)
	_ = fetcher.Calls(srv.URL)
	_ = fetcher.Team([]string{srv.URL})

	reports := fetcher.Reports()
	for _, source := range []string{SourceGroups, SourceTeachers, SourceCalls, SourceTeam} {
		if _, ok := reports[source]; !ok {
			t.Errorf("no report for %s", source)
		}
	}
}
