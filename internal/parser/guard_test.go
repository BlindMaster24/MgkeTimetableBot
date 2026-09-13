package parser

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func groupsHTML(count int) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="entry"><div class="content">`)
	for i := 0; i < count; i++ {
		name := strconv.Itoa(100 + i)
		b.WriteString(`<h2>Группа - ` + name + `</h2>
<table>
<tr><th>№</th><th>Понедельник, 07.09.2026</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов И.И.<br>3-205</td></tr>
</table>`)
	}
	b.WriteString(`</div></div></body></html>`)
	return b.String()
}

func serveGroups(t *testing.T, count int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(groupsHTML(count)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGuardTripsOnlyAboveTheConfiguredLimits(t *testing.T) {
	guard := Guard{}.WithDefaults()

	if !guard.Trips(40, 7) {
		t.Error("a drop from 40 to 7 is more than 80%")
	}
	if guard.Trips(40, 8) {
		t.Error("a drop from 40 to 8 is exactly the 80% limit and must pass")
	}
	if guard.Trips(9, 1) {
		t.Error("caches below min_items must not trigger the guard")
	}

	disabled := Guard{Disabled: true, MinItems: 10, MaxDropPercent: 80}
	if disabled.Trips(40, 1) {
		t.Error("a disabled guard must never trip")
	}

	invalid := Guard{MinItems: -3, MaxDropPercent: 400}.WithDefaults()
	if invalid != guard {
		t.Errorf("invalid thresholds must fall back to the defaults: %+v", invalid)
	}
}

func TestFetcherGuardHonoursTheConfiguredDropLimit(t *testing.T) {
	c := newTestCache(t)
	seedGroups(t, c, 40)

	srv := serveGroups(t, 8)

	strict := NewFetcher(logger.New("error", nil), c, Options{Guard: Guard{MaxDropPercent: 50}})
	if err := strict.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	if len(c.GetGroups()) != 40 {
		t.Errorf("a 50%% limit must keep the cache: %d groups", len(c.GetGroups()))
	}
	if report := strict.Reports()[SourceGroups]; report.Keep == nil || report.Keep.LimitPercent != 50 {
		t.Errorf("the report must carry the configured limit: %+v", report.Keep)
	}

	relaxedCache := newTestCache(t)
	seedGroups(t, relaxedCache, 40)
	relaxed := NewFetcher(logger.New("error", nil), relaxedCache, Options{Guard: Guard{MaxDropPercent: 90}})
	if err := relaxed.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	if len(relaxedCache.GetGroups()) <= 40 {
		t.Errorf("a 90%% limit must accept the parse: %d groups", len(relaxedCache.GetGroups()))
	}
}

func TestFetcherGuardIgnoresSmallCaches(t *testing.T) {
	c := newTestCache(t)
	seedGroups(t, c, 40)

	srv := serveGroups(t, 2)

	fetcher := NewFetcher(logger.New("error", nil), c, Options{Guard: Guard{MinItems: 50}})
	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	if len(c.GetGroups()) <= 40 {
		t.Errorf("a cache below min_items must be updated: %d groups", len(c.GetGroups()))
	}
	if report := fetcher.Reports()[SourceGroups]; report.Keep != nil {
		t.Errorf("no keep must be reported: %+v", report.Keep)
	}
}

func TestFetcherGuardCanBeDisabled(t *testing.T) {
	c := newTestCache(t)
	seedGroups(t, c, 40)

	srv := serveGroups(t, 1)

	fetcher := NewFetcher(logger.New("error", nil), c, Options{Guard: Guard{Disabled: true}})
	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	if len(c.GetGroups()) <= 40 {
		t.Errorf("a disabled guard must apply the parse: %d groups", len(c.GetGroups()))
	}
}

func TestFetcherReportsStructuredKeep(t *testing.T) {
	c := newTestCache(t)
	seedGroups(t, c, 40)

	srv := serveGroups(t, 3)

	fetcher := NewFetcher(logger.New("error", nil), c, Options{})
	if err := fetcher.Timetable(srv.URL, srv.URL); err != nil {
		t.Fatalf("timetable: %v", err)
	}

	keep := fetcher.Reports()[SourceGroups].Keep
	if keep == nil {
		t.Fatal("expected the report to carry the keep reason")
	}
	if keep.Reason != KeepReasonShrink || keep.Previous != 40 || keep.Current != 3 || keep.LimitPercent != 80 {
		t.Errorf("keep = %+v", keep)
	}
	if keep.DropPercent() != 92 {
		t.Errorf("drop = %d%%, want 92%%", keep.DropPercent())
	}
	if summary := keep.Summary(); !strings.Contains(summary, "40 -> 3") {
		t.Errorf("summary = %q", summary)
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
