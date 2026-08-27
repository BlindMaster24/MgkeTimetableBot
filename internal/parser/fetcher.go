package parser

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const userAgent = "MGKE timetable bot (https://github.com/BlindMaster24/MgkeTimetableBot)"

func FetchAndParse(log *logger.Logger, c *cache.RaspCache, groupURL, teacherURL string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	groupData, groupHash, err := fetchAndParseGroups(client, groupURL)
	if err != nil {
		log.Error().Err(err).Str("url", groupURL).Msg("group parse failed")
	} else {
		c.SetGroups(groupData, groupHash)
		log.Info().Int("groups", len(groupData)).Str("hash", groupHash).Msg("groups parsed")
	}

	teacherData, teacherHash, err := fetchAndParseTeachers(client, teacherURL)
	if err != nil {
		log.Error().Err(err).Str("url", teacherURL).Msg("teacher parse failed")
	} else {
		c.SetTeachers(teacherData, teacherHash)
		log.Info().Int("teachers", len(teacherData)).Str("hash", teacherHash).Msg("teachers parsed")
	}

	if err := c.Save(); err != nil {
		return fmt.Errorf("save cache: %w", err)
	}

	return nil
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
