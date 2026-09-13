package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const userAgent = "MGKE timetable bot (https://github.com/BlindMaster24/MgkeTimetableBot)"

type Options struct {
	Proxy string
}

type Fetcher struct {
	log    *logger.Logger
	cache  *cache.RaspCache
	client *http.Client
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

	return &Fetcher{log: log, cache: c, client: client}
}

func (f *Fetcher) Cache() *cache.RaspCache { return f.cache }

func (f *Fetcher) Timetable(groupURL, teacherURL string) error {
	var errs []error

	if groupData, groupHash, err := fetchAndParseGroups(f.client, groupURL); err != nil {
		f.log.Error().Err(err).Str("url", groupURL).Msg("group parse failed")
		errs = append(errs, err)
	} else {
		groupData = jsonRoundTrip(groupData)
		f.cache.SetGroups(groupData, groupHash)
		f.log.Info().Int("groups", len(groupData)).Str("hash", groupHash).Msg("groups parsed")
	}

	if teacherData, teacherHash, err := fetchAndParseTeachers(f.client, teacherURL); err != nil {
		f.log.Error().Err(err).Str("url", teacherURL).Msg("teacher parse failed")
		errs = append(errs, err)
	} else {
		teacherData = jsonRoundTrip(teacherData)
		f.cache.SetTeachers(teacherData, teacherHash)
		f.log.Info().Int("teachers", len(teacherData)).Str("hash", teacherHash).Msg("teachers parsed")
	}

	if err := f.cache.Save(); err != nil {
		errs = append(errs, fmt.Errorf("save cache: %w", err))
	}
	return errors.Join(errs...)
}

func (f *Fetcher) Calls(bellScheduleURL string) error {
	if bellScheduleURL == "" {
		return nil
	}

	schedule := fetchAndParseCalls(f.client, bellScheduleURL)
	if schedule == nil {
		f.log.Warn().Str("url", bellScheduleURL).Msg("calls parse returned empty")
		return nil
	}

	f.cache.SetCallsNotify(*schedule, cache.Schedule{}, "site", "")
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
	var errs []error

	for _, rawURL := range urls {
		resp, err := fetchHTML(f.client, rawURL)
		if err != nil {
			f.log.Error().Err(err).Str("url", rawURL).Msg("team parse failed")
			errs = append(errs, err)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			f.log.Error().Err(err).Str("url", rawURL).Msg("team parse failed")
			errs = append(errs, err)
			continue
		}

		team = ParseTeam(doc, team)
		hashes = append(hashes, hashDocument(doc))
	}

	f.cache.SetTeam(team, hashes)
	f.log.Info().Int("names", len(team)).Int("pages", len(urls)).Msg("team parsed")

	if err := f.cache.Save(); err != nil {
		errs = append(errs, fmt.Errorf("save cache: %w", err))
	}
	return errors.Join(errs...)
}

func FetchAndParse(log *logger.Logger, c *cache.RaspCache, groupURL, teacherURL, bellScheduleURL string) error {
	fetcher := NewFetcher(log, c, Options{})
	return errors.Join(fetcher.Timetable(groupURL, teacherURL), fetcher.Calls(bellScheduleURL))
}

func fetchAndParseGroups(client *http.Client, url string) (map[string]any, string, error) {
	resp, err := fetchHTML(client, url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("parse html: %w", err)
	}

	p := NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		return nil, "", err
	}

	result := make(map[string]any)
	for key, group := range groups {
		result[key] = map[string]any{
			"group": group.Group,
			"days":  group.Days,
		}
	}

	return result, p.ContentHash(), nil
}

func fetchAndParseTeachers(client *http.Client, url string) (map[string]any, string, error) {
	resp, err := fetchHTML(client, url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("parse html: %w", err)
	}

	p := NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		return nil, "", err
	}

	result := make(map[string]any)
	for key, teacher := range teachers {
		result[key] = map[string]any{
			"teacher": teacher.Teacher,
			"days":    teacher.Days,
		}
	}

	return result, p.ContentHash(), nil
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

func jsonRoundTrip(v map[string]any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return v
	}
	return result
}

func fetchAndParseCalls(client *http.Client, url string) *cache.Schedule {
	resp, err := fetchHTML(client, url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	return ParseCallsSchedule(doc)
}
