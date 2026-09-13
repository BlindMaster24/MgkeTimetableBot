package apiprobe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

const (
	DefaultTimeout = 15 * time.Second
	HealthPath     = "/api/health"
)

type Target struct {
	Method string
	Path   string
}

type Result struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	Err      string
}

func (r Result) Label() string {
	if r.Method == "" {
		return r.Path
	}
	return r.Method + " " + r.Path
}

func (r Result) Healthy() bool {
	if r.Err != "" {
		return false
	}
	if r.Status < 500 {
		return r.Status < 400
	}
	return r.Path == HealthPath && r.Status == http.StatusServiceUnavailable
}

func Targets(c *cache.RaspCache) []Target {
	targets := []Target{
		{http.MethodGet, "/api/info"},
		{http.MethodGet, "/api/groups"},
		{http.MethodGet, "/api/teachers"},
		{http.MethodGet, "/api/parser-health"},
		{http.MethodGet, HealthPath},
	}

	if c != nil {
		if name := sampleKey(c.GetGroups()); name != "" {
			targets = append(targets, Target{http.MethodGet, "/api/group/" + url.PathEscape(name)})
		}
		if name := sampleKey(c.GetTeachers()); name != "" {
			targets = append(targets, Target{http.MethodGet, "/api/teacher/" + url.PathEscape(name)})
		}
	}

	return targets
}

func Run(ctx context.Context, client *http.Client, baseURL string, targets []Target) []Result {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}

	base := strings.TrimRight(baseURL, "/")
	results := make([]Result, 0, len(targets))
	for _, target := range targets {
		results = append(results, probe(ctx, client, base, target))
	}
	return results
}

func probe(ctx context.Context, client *http.Client, base string, target Target) Result {
	result := Result{Method: target.Method, Path: target.Path}

	request, err := http.NewRequestWithContext(ctx, target.Method, base+target.Path, nil)
	if err != nil {
		result.Err = err.Error()
		return result
	}

	started := time.Now()
	response, err := client.Do(request)
	result.Duration = time.Since(started)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	defer response.Body.Close()

	_, _ = io.Copy(io.Discard, response.Body)
	result.Status = response.StatusCode
	result.Duration = time.Since(started)
	return result
}

func Healthy(results []Result) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !result.Healthy() {
			return false
		}
	}
	return true
}

func HealthyCount(results []Result) int {
	count := 0
	for _, result := range results {
		if result.Healthy() {
			count++
		}
	}
	return count
}

func Failed(results []Result) []Result {
	var failed []Result
	for _, result := range results {
		if !result.Healthy() {
			failed = append(failed, result)
		}
	}
	return failed
}

func Slowest(results []Result) (Result, bool) {
	if len(results) == 0 {
		return Result{}, false
	}

	slowest := results[0]
	for _, result := range results[1:] {
		if result.Duration > slowest.Duration {
			slowest = result
		}
	}
	return slowest, true
}

func Describe(result Result) string {
	if result.Err != "" {
		return fmt.Sprintf("%s: %s", result.Label(), result.Err)
	}
	return fmt.Sprintf("%s (%d)", result.Label(), result.Status)
}

func sampleKey[T any](values map[string]T) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return keys[0]
}
