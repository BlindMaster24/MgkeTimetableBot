package preflight

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func checkImageLayout() Check {
	checks, err := imagepkg.CheckGoldenLayout()
	if err != nil {
		return Check{Name: "image:layout", Level: LevelFail, Detail: "render: " + err.Error()}
	}

	for _, golden := range checks {
		if golden.Differing < 0 {
			return Check{
				Name:   "image:layout",
				Level:  LevelFail,
				Detail: golden.Name + ": the rendered sample changed size, so the layout moved",
				Hints:  []string{"error: run go test ./internal/image -update after reviewing the render"},
			}
		}
		if golden.Differing > 0 {
			return Check{
				Name:   "image:layout",
				Level:  LevelFail,
				Detail: fmt.Sprintf("%s: %d pixels differ from golden/%s.png", golden.Name, golden.Differing, golden.Name),
				Hints:  []string{"error: run go test ./internal/image -update after reviewing the render"},
			}
		}
	}

	return Check{Name: "image:layout", Level: LevelOK, Detail: fmt.Sprintf("%d reference renders match the committed layout", len(checks))}
}

func checkImage(groups model.Groups, opts Options) Check {
	if opts.SkipImage {
		return Check{Name: "image", Level: LevelSkip, Detail: "image rendering skipped"}
	}
	if len(groups) == 0 {
		return Check{Name: "image", Level: LevelSkip, Detail: "no freshly parsed groups to render"}
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		days := entryDays(groups[name])
		if len(days) == 0 {
			continue
		}
		return renderCheck(name, days, opts)
	}
	return Check{Name: "image", Level: LevelSkip, Detail: "no group carried a day to render"}
}

func checkImageFromCache(cfg *config.Config, opts Options) Check {
	if opts.SkipImage {
		return Check{Name: "image", Level: LevelSkip, Detail: "image rendering skipped"}
	}

	cached, err := cache.New(cfg.ResolvedCacheDir())
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "cache: " + err.Error()}
	}

	groups := cached.GetGroups()
	if len(groups) == 0 {
		return Check{Name: "image", Level: LevelSkip, Detail: "no cached groups to render"}
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		days := entryDays(groups[name])
		if len(days) == 0 {
			continue
		}
		check := renderCheck(name, days, opts)
		if check.Level == LevelOK {
			check.Detail = "cached " + check.Detail
		}
		return check
	}
	return Check{Name: "image", Level: LevelSkip, Detail: "no cached group carried a day to render"}
}

func renderCheck(name string, days []map[string]any, opts Options) Check {
	font := imagepkg.FontPath()
	if font == "" {
		return Check{Name: "image", Level: LevelFail, Detail: "no loadable font found on this system", Hints: []string{"error: install font-dejavu in the runtime image"}}
	}

	dir := opts.ImageDir
	remove := false
	if dir == "" {
		temp, err := os.MkdirTemp("", "preflight-image")
		if err != nil {
			return Check{Name: "image", Level: LevelFail, Detail: "temp dir: " + err.Error()}
		}
		dir = temp
		remove = true
	}
	if remove {
		defer os.RemoveAll(dir)
	}

	path, err := imagepkg.RenderGroupDays(name, days, dir, imagepkg.Texts{}, "")
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "render: " + err.Error()}
	}

	file, err := os.Open(path)
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "open rendered image: " + err.Error()}
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "stat rendered image: " + err.Error()}
	}

	config, err := png.DecodeConfig(file)
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "the rendered file is not a readable PNG: " + err.Error()}
	}
	if config.Width <= 0 || config.Height <= 0 || info.Size() == 0 {
		return Check{Name: "image", Level: LevelFail, Detail: "the rendered PNG is empty"}
	}

	return Check{
		Name:   "image",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%q -> %dx%d px, %d KB, font %s", name, config.Width, config.Height, info.Size()/1024, filepath.Base(font)),
	}
}

func groupDays(groups model.Groups) []dayEntry {
	entries := make([]dayEntry, 0, len(groups))
	for name, group := range groups {
		if group == nil {
			continue
		}
		for _, day := range group.Days {
			entries = append(entries, dayEntry{
				owner:    name,
				label:    day.Day,
				lessons:  len(day.Lessons),
				hasOwner: true,
			})
		}
	}
	return entries
}

func teacherDays(teachers model.Teachers) []dayEntry {
	entries := make([]dayEntry, 0, len(teachers))
	for name, teacher := range teachers {
		if teacher == nil {
			continue
		}
		for _, day := range teacher.Days {
			entries = append(entries, dayEntry{
				owner:    name,
				label:    day.Day,
				lessons:  len(day.Lessons),
				hasOwner: true,
			})
		}
	}
	return entries
}

type dayEntry struct {
	owner    string
	label    string
	lessons  int
	hasOwner bool
}

func dayQuality(entries []dayEntry) (dated int, bad []string, unordered []string, withLessons int) {
	owners := make(map[string]bool)

	for _, entry := range entries {
		if _, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label)); err != nil {
			bad = append(bad, fmt.Sprintf("%s: %q", entry.owner, entry.label))
			continue
		}
		dated++
		if entry.lessons > 0 {
			owners[entry.owner] = true
		}
	}
	withLessons = len(owners)

	for _, owner := range ownersOf(entries) {
		previous := time.Time{}
		for _, entry := range entries {
			if entry.owner != owner {
				continue
			}
			parsed, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label))
			if err != nil {
				continue
			}
			if !previous.IsZero() && parsed.Before(previous) {
				unordered = append(unordered, fmt.Sprintf("%s: %s before %s", owner, parsed.Format("02.01.2006"), previous.Format("02.01.2006")))
			}
			previous = parsed
		}
	}
	return dated, bad, unordered, withLessons
}

func ownersOf(entries []dayEntry) []string {
	seen := make(map[string]bool)
	var owners []string
	for _, entry := range entries {
		if entry.hasOwner && !seen[entry.owner] {
			seen[entry.owner] = true
			owners = append(owners, entry.owner)
		}
	}
	return owners
}

func dateRangeHint(entries []dayEntry) string {
	var first, last time.Time
	for _, entry := range entries {
		parsed, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label))
		if err != nil {
			continue
		}
		if first.IsZero() || parsed.Before(first) {
			first = parsed
		}
		if parsed.After(last) {
			last = parsed
		}
	}
	if first.IsZero() {
		return ""
	}

	today := time.Now()
	todayIndex := utils.DayIndexFromDate(today)
	if utils.DayIndexFromDate(last) < todayIndex {
		return fmt.Sprintf("the whole timetable ends at %s, the site has not published the current week", last.Format("02.01.2006"))
	}
	if utils.DayIndexFromDate(first) > todayIndex+30 {
		return fmt.Sprintf("the timetable starts at %s, far in the future", first.Format("02.01.2006"))
	}
	return ""
}

func probeHints(report parser.Report) []string {
	var hints []string
	for _, probe := range report.Failing() {
		if probe.Inverted {
			hints = append(hints, fmt.Sprintf("error: %q found %d unexpected item(s) (expected %s)", probe.Selector, probe.Missing, probe.Expected))
			continue
		}
		hints = append(hints, fmt.Sprintf("error: no data for %q (expected %s)", probe.Selector, probe.Expected))
	}
	for _, probe := range report.Advisories() {
		hints = append(hints, fmt.Sprintf("warning: no data for %q (expected %s)", probe.Selector, probe.Expected))
	}
	for _, fallback := range report.Fallbacks {
		hints = append(hints, "warning: fallback used: "+fallback)
	}
	for _, warning := range report.Warnings {
		hints = append(hints, "warning: "+warning)
	}
	return hints
}

func entryDays(data any) []map[string]any {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil
	}

	days, _ := entry["days"].([]any)
	result := make([]map[string]any, 0, len(days))
	for _, day := range days {
		if m, ok := day.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func endpointsFor(cfg *config.Config, override Endpoints) Endpoints {
	endpoints := Endpoints{
		Groups:   cfg.Parser.Endpoints.TimetableGroup,
		Teachers: cfg.Parser.Endpoints.TimetableTeacher,
		Calls:    cfg.Parser.Endpoints.BellSchedule,
	}
	if override.Groups != "" {
		endpoints.Groups = override.Groups
	}
	if override.Teachers != "" {
		endpoints.Teachers = override.Teachers
	}
	if override.Calls != "" {
		endpoints.Calls = override.Calls
	}
	return endpoints
}

func limit(values []string, max int) []string {
	if len(values) <= max {
		return values
	}
	return values[:max]
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, localePath)); err == nil {
				return dir
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
