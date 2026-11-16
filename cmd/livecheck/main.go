// Command livecheck audits a live cache dump for empty days, bad dates and empty lessons.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func main() {
	log := logger.New("info", nil)
	cfg, err := config.LoadWithEnv(os.Args[1], os.LookupEnv)
	if err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "livecheck")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	rasp, err := cache.New(dir)
	if err != nil {
		panic(err)
	}
	issues := 0
	fetcher := parser.NewFetcher(log, rasp, parser.Options{OnReport: func(report parser.Report) {
		fmt.Printf("report %s: items=%d probes=%d warnings=%d fallbacks=%d keptOld=%v\n",
			report.Source, report.Items, len(report.Probes), len(report.Warnings), len(report.Fallbacks), report.KeptOld)
		for _, probe := range report.Probes {
			if !probe.OK() {
				fmt.Printf("  probe failed: %s %s (expected %q, found %d)\n", probe.Source, probe.Selector, probe.Expected, probe.Found)
				issues++
			}
		}
		for _, warning := range report.Warnings {
			fmt.Println("  warning:", warning)
			issues++
		}
		for _, fallback := range report.Fallbacks {
			fmt.Println("  fallback:", fallback)
		}
		if report.KeptOld {
			fmt.Println("  kept previous cache entry")
			issues++
		}
	}})

	started := time.Now()
	ctx := context.Background()
	if err := fetcher.Timetable(ctx, cfg.Parser.Endpoints.TimetableGroup, cfg.Parser.Endpoints.TimetableTeacher); err != nil {
		fmt.Println("timetable error:", err)
		issues++
	}
	if err := fetcher.Calls(ctx, cfg.Parser.Endpoints.BellSchedule); err != nil {
		fmt.Println("calls error:", err)
		issues++
	}
	if err := fetcher.Team(ctx, cfg.Parser.Endpoints.Team); err != nil {
		fmt.Println("team error:", err)
		issues++
	}
	fmt.Printf("fetched in %s\n", time.Since(started))

	for kind, entries := range map[string]map[string]any{"group": rasp.GetGroups(), "teacher": rasp.GetTeachers()} {
		result := auditEntries(kind, entries)
		result.report(kind)
		issues += result.Issues()
	}
	fmt.Printf("groups=%d teachers=%d teamNames=%d\n", len(rasp.GetGroups()), len(rasp.GetTeachers()), len(rasp.GetTeamNames()))

	calls := rasp.GetCalls()
	fmt.Printf("calls: weekdays=%d saturday=%d source=%q\n", len(calls.Active.Schedule.Weekdays), len(calls.Active.Schedule.Saturday), calls.Active.Source)
	if len(calls.Active.Schedule.Weekdays) == 0 || len(calls.Active.Schedule.Saturday) == 0 {
		fmt.Println("  issue: active calls schedule is empty")
		issues++
	}
	for i, slot := range calls.Active.Schedule.Weekdays {
		if slot[0][0] == "" || slot[1][1] == "" {
			fmt.Println("  issue: weekday slot", i, "has empty bounds")
			issues++
		}
	}

	if issues > 0 {
		fmt.Printf("FAILED with %d issues\n", issues)
		os.Exit(1)
	}
	fmt.Println("all invariants hold")
}

type auditResult struct {
	Entries        int
	EmptyDays      int
	BadDates       int
	LessonlessDays int
	BadLessons     int
	DayCounts      map[int]int
	EmptyWeekdays  map[string]int
	Notes          []string
}

// Issues counts the findings that make the dump unhealthy.
func (r auditResult) Issues() int {
	return r.EmptyDays + r.BadDates + r.BadLessons
}

func auditEntries(kind string, entries map[string]any) auditResult {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	result := auditResult{
		Entries:       len(entries),
		DayCounts:     map[int]int{},
		EmptyWeekdays: map[string]int{},
	}
	note := func(format string, args ...any) {
		result.Notes = append(result.Notes, fmt.Sprintf(format, args...))
	}

	for _, name := range names {
		entry, _ := entries[name].(map[string]any)
		days, _ := entry["days"].([]any)
		if len(days) == 0 {
			result.EmptyDays++
			if result.EmptyDays <= 5 {
				note("  %s %q: no days", kind, name)
			}
			continue
		}
		result.DayCounts[len(days)]++
		for _, raw := range days {
			day, _ := raw.(map[string]any)
			if day == nil {
				continue
			}
			dateStr, _ := day["day"].(string)
			parsed, err := time.Parse("02.01.2006", dateStr)
			if err != nil {
				result.BadDates++
				if result.BadDates <= 5 {
					note("  %s %q: unparsable day %q", kind, name, dateStr)
				}
				continue
			}
			lessons, _ := day["lessons"].([]any)
			if len(lessons) == 0 {
				result.EmptyWeekdays[parsed.Weekday().String()]++
				result.LessonlessDays++
				continue
			}
			for _, rawLesson := range lessons {
				switch entry := rawLesson.(type) {
				case nil:
					continue
				case map[string]any:
					bad := entry["lesson"] == nil || entry["lesson"] == ""
					if lessonStr, ok := entry["lesson"].(string); ok && strings.ContainsAny(lessonStr, "()") {
						bad = true
					}
					if groupStr, ok := entry["group"].(string); ok && strings.Contains(groupStr, "-") {
						bad = true
					}
					if bad {
						result.BadLessons++
						if result.BadLessons <= 5 {
							note("  %s %q %s: bad lesson map: %v", kind, name, dateStr, entry)
						}
					}
				case []any:
					for _, sub := range entry {
						m, _ := sub.(map[string]any)
						if m == nil || m["lesson"] == nil || m["lesson"] == "" || m["subgroup"] == nil {
							result.BadLessons++
							if result.BadLessons <= 5 {
								note("  %s %q %s: bad subgroup: %v", kind, name, dateStr, sub)
							}
						}
					}
				default:
					result.BadLessons++
					note("  %s %q %s: unexpected lesson shape %T", kind, name, dateStr, rawLesson)
				}
			}
		}
	}
	return result
}

func (r auditResult) report(kind string) {
	for _, note := range r.Notes {
		fmt.Println(note)
	}
	fmt.Printf("%s audit: entries=%d emptyDays=%d badDates=%d lessonlessDays=%d badLessons=%d dayCounts=%v emptyWeekdays=%v\n",
		kind, r.Entries, r.EmptyDays, r.BadDates, r.LessonlessDays, r.BadLessons, r.DayCounts, r.EmptyWeekdays)
}
