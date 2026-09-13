package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const (
	userAgent = "MGKE timetable bot (https://github.com/BlindMaster24/MgkeTimetableBot)"

	shrinkFloor     = 10
	shrinkThreshold = 5
)

type Options struct {
	Proxy    string
	OnReport func(Report)
}

type Fetcher struct {
	log      *logger.Logger
	cache    *cache.RaspCache
	client   *http.Client
	onReport func(Report)

	mu      sync.Mutex
	reports map[string]Report
}

func NewFetcher(log *logger.Logger, c *cache.RaspCache, opts Options) *Fetcher {
	client := &http.Client{Timeout: 30 * time.Second}

	if proxy := strings.TrimSpace(opts.Proxy); proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(parsed)}
		} else {
			log.Error().Err(err).Str("proxy", proxy).Msg("invalid proxy url, using a direct connection")
		}
	}

	return &Fetcher{
		log:      log,
		cache:    c,
		client:   client,
		onReport: opts.OnReport,
		reports:  make(map[string]Report),
	}
}

func (f *Fetcher) Cache() *cache.RaspCache { return f.cache }

func (f *Fetcher) Reports() map[string]Report {
	f.mu.Lock()
	defer f.mu.Unlock()

	reports := make(map[string]Report, len(f.reports))
	for source, report := range f.reports {
		reports[source] = report
	}
	return reports
}

func (f *Fetcher) emit(report Report) {
	f.mu.Lock()
	f.reports[report.Source] = report
	f.mu.Unlock()

	event := f.log.Info()
	if !report.OK() {
		event = f.log.Warn()
	}
	event.Str("source", report.Source).Str("report", report.Summary()).Msg("parse report")

	if f.onReport != nil {
		f.onReport(report)
	}
}

func (f *Fetcher) Timetable(groupURL, teacherURL string) error {
	var errs []error

	if err := f.fetchGroups(groupURL); err != nil {
		errs = append(errs, err)
	}
	if err := f.fetchTeachers(teacherURL); err != nil {
		errs = append(errs, err)
	}
	if err := f.cache.Save(); err != nil {
		errs = append(errs, fmt.Errorf("save cache: %w", err))
	}

	return errors.Join(errs...)
}

func (f *Fetcher) fetchGroups(groupURL string) error {
	doc, err := f.document(groupURL)
	if err != nil {
		f.emit(failedReport(SourceGroups, groupURL, err))
		f.log.Error().Err(err).Str("url", groupURL).Msg("group parse failed")
		return err
	}

	groupParser := NewGroupParser(doc)
	groups, err := groupParser.Run()
	report := groupParser.Report()
	report.URL = groupURL

	if err != nil {
		report.Warn("parser error: %v", err)
		f.emit(report)
		f.log.Error().Err(err).Str("url", groupURL).Msg("group parse failed")
		return err
	}

	if len(groups) == 0 {
		report.KeepOld("the page produced no groups")
		f.emit(report)
		f.log.Error().Str("url", groupURL).Msg("group parse returned no groups, cache left untouched")
		return nil
	}

	previous := len(f.cache.GetGroups())
	if suspiciousShrink(previous, len(groups)) {
		report.KeepOld(fmt.Sprintf("group count dropped from %d to %d", previous, len(groups)))
		f.emit(report)
		f.log.Error().Int("previous", previous).Int("parsed", len(groups)).Msg("group parse looks broken, cache left untouched")
		return nil
	}

	f.cache.SetGroups(jsonRoundTrip(groups), groupParser.ContentHash())
	f.emit(report)
	f.log.Info().Int("groups", len(groups)).Str("hash", groupParser.ContentHash()).Msg("groups parsed")

	return nil
}

func (f *Fetcher) fetchTeachers(teacherURL string) error {
	doc, err := f.document(teacherURL)
	if err != nil {
		f.emit(failedReport(SourceTeachers, teacherURL, err))
		f.log.Error().Err(err).Str("url", teacherURL).Msg("teacher parse failed")
		return err
	}

	teacherParser := NewTeacherParser(doc)
	teachers, err := teacherParser.Run()
	report := teacherParser.Report()
	report.URL = teacherURL

	if err != nil {
		report.Warn("parser error: %v", err)
		f.emit(report)
		f.log.Error().Err(err).Str("url", teacherURL).Msg("teacher parse failed")
		return err
	}

	if len(teachers) == 0 {
		report.KeepOld("the page produced no teachers")
		f.emit(report)
		f.log.Error().Str("url", teacherURL).Msg("teacher parse returned no teachers, cache left untouched")
		return nil
	}

	previous := len(f.cache.GetTeachers())
	if suspiciousShrink(previous, len(teachers)) {
		report.KeepOld(fmt.Sprintf("teacher count dropped from %d to %d", previous, len(teachers)))
		f.emit(report)
		f.log.Error().Int("previous", previous).Int("parsed", len(teachers)).Msg("teacher parse looks broken, cache left untouched")
		return nil
	}

	f.cache.SetTeachers(jsonRoundTrip(teachers), teacherParser.ContentHash())
	f.emit(report)
	f.log.Info().Int("teachers", len(teachers)).Str("hash", teacherParser.ContentHash()).Msg("teachers parsed")

	return nil
}

func (f *Fetcher) Calls(bellScheduleURL string) error {
	if bellScheduleURL == "" {
		return nil
	}

	doc, err := f.document(bellScheduleURL)
	if err != nil {
		f.emit(failedReport(SourceCalls, bellScheduleURL, err))
		f.log.Warn().Err(err).Str("url", bellScheduleURL).Msg("calls parse failed")
		return nil
	}

	schedule, report := ParseCallsScheduleReport(doc)
	report.URL = bellScheduleURL

	if schedule == nil || len(schedule.Weekdays) == 0 {
		report.KeepOld("the page produced no bell schedule slots")
		f.emit(report)
		f.log.Warn().Str("url", bellScheduleURL).Msg("calls parse returned empty, cache left untouched")
		return nil
	}

	f.cache.SetCallsNotify(*schedule, cache.Schedule{}, "site", "")
	f.emit(report)
	f.log.Info().Int("weekdays", len(schedule.Weekdays)).Msg("calls parsed from site")

	if err := f.cache.Save(); err != nil {
		return fmt.Errorf("save cache: %w", err)
	}
	return nil
}

func (f *Fetcher) Team(urls []string) error {
	if len(urls) == 0 {
		return nil
	}

	team := f.cache.GetTeamNames()
	if team == nil {
		team = make(map[string]string)
	}

	hashes := make([]string, 0, len(urls))
	var reports []Report
	var errs []error

	for _, rawURL := range urls {
		doc, err := f.document(rawURL)
		if err != nil {
			reports = append(reports, failedReport(SourceTeam, rawURL, err))
			f.log.Error().Err(err).Str("url", rawURL).Msg("team parse failed")
			errs = append(errs, err)
			continue
		}

		updated, report := ParseTeamReport(doc, team)
		report.URL = rawURL
		reports = append(reports, report)
		hashes = append(hashes, hashDocument(doc))
		team = updated
	}

	merged := mergeReports(reports...)
	merged.Items = len(team)

	if len(team) == 0 {
		merged.KeepOld("the pages produced no staff names")
		f.emit(merged)
		f.log.Warn().Int("pages", len(urls)).Msg("team parse returned no names, cache left untouched")
		return errors.Join(errs...)
	}

	f.cache.SetTeam(team, hashes)
	f.emit(merged)
	f.log.Info().Int("names", len(team)).Int("pages", len(urls)).Msg("team parsed")

	if err := f.cache.Save(); err != nil {
		errs = append(errs, fmt.Errorf("save cache: %w", err))
	}
	return errors.Join(errs...)
}

func (f *Fetcher) document(rawURL string) (*goquery.Document, error) {
	resp, err := fetchHTML(f.client, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html %s: %w", rawURL, err)
	}
	return doc, nil
}

func FetchAndParse(log *logger.Logger, c *cache.RaspCache, groupURL, teacherURL, bellScheduleURL string) error {
	fetcher := NewFetcher(log, c, Options{})
	return errors.Join(fetcher.Timetable(groupURL, teacherURL), fetcher.Calls(bellScheduleURL))
}

func failedReport(source, url string, err error) Report {
	report := Report{Source: source, URL: url, At: time.Now()}
	report.Warn("fetch failed: %v", err)
	return report
}

func suspiciousShrink(previous, current int) bool {
	if previous < shrinkFloor {
		return false
	}
	return current*shrinkThreshold < previous
}

func fetchHTML(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("fetch %s: status %d, body: %s", url, resp.StatusCode, truncate(string(body), 200))
	}

	return resp, nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func jsonRoundTrip(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}
