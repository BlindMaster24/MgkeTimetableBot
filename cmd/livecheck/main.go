// Command livecheck audits a live cache dump for empty days, bad dates and empty lessons.
package main

import (
	"context"
	"fmt"
	"io"
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
	audit := &reportAudit{out: os.Stdout}
	fetcher := parser.NewFetcher(log, rasp, parser.Options{OnReport: audit.onReport})

	started := time.Now()
	ctx := context.Background()
	if err := fetcher.Timetable(ctx, cfg.Parser.Endpoints.TimetableGroup, cfg.Parser.Endpoints.TimetableTeacher); err != nil {
		fmt.Println("timetable error:", err)
		audit.fail()
	}
	if err := fetcher.Calls(ctx, cfg.Parser.Endpoints.BellSchedule); err != nil {
		fmt.Println("calls error:", err)
		audit.fail()
	}
	if err := fetcher.Team(ctx, cfg.Parser.Endpoints.Team); err != nil {
		fmt.Println("team error:", err)
		audit.fail()
	}
	fmt.Printf("fetched in %s\n", time.Since(started))

	for kind, entries := range map[string]map[string]any{"group": rasp.GetGroups(), "teacher": rasp.GetTeachers()} {
		result := auditEntries(kind, entries)
		result.report(kind)
		audit.add(result.Issues())
	}
	fmt.Printf("groups=%d teachers=%d teamNames=%d\n", len(rasp.GetGroups()), len(rasp.GetTeachers()), len(rasp.GetTeamNames()))

	calls := rasp.GetCalls()
	fmt.Printf("calls: weekdays=%d saturday=%d source=%q\n", len(calls.Active.Schedule.Weekdays), len(calls.Active.Schedule.Saturday), calls.Active.Source)
	for _, problem := range callsProblems(calls.Active.Schedule) {
		fmt.Println("  issue:", problem)
		audit.fail()
	}

	if total := audit.Issues(); total > 0 {
		fmt.Printf("FAILED with %d issues\n", total)
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

// reportAudit turns parser reports into findings and a running issue count.
type reportAudit struct {
	out    io.Writer
	issues int
}

// onReport prints one parser report and counts its problems.
func (a *reportAudit) onReport(report parser.Report) {
	fmt.Fprintf(a.out, "report %s: items=%d probes=%d warnings=%d fallbacks=%d keptOld=%v\n",
		report.Source, report.Items, len(report.Probes), len(report.Warnings), len(report.Fallbacks), report.KeptOld)
	for _, probe := range report.Probes {
		if !probe.OK() {
			fmt.Fprintf(a.out, "  probe failed: %s %s (expected %q, found %d)\n", probe.Source, probe.Selector, probe.Expected, probe.Found)
			a.issues++
		}
	}
	for _, warning := range report.Warnings {
		fmt.Fprintln(a.out, "  warning:", warning)
		a.issues++
	}
	for _, fallback := range report.Fallbacks {
		fmt.Fprintln(a.out, "  fallback:", fallback)
	}
	if report.KeptOld {
		fmt.Fprintln(a.out, "  kept previous cache entry")
		a.issues++
	}
}

// Issues is the number of findings that make the dump unhealthy.
func (a *reportAudit) Issues() int {
	return a.issues
}

func (a *reportAudit) fail() {
	a.issues++
}

func (a *reportAudit) add(count int) {
	a.issues += count
}

// callsProblems lists the reasons an active bell schedule cannot be used.
func callsProblems(schedule cache.CallsSchedule) []string {
	var problems []string
	if len(schedule.Weekdays) == 0 || len(schedule.Saturday) == 0 {
		problems = append(problems, "active calls schedule is empty")
	}
	for i, slot := range schedule.Weekdays {
		if slot[0][0] == "" || slot[1][1] == "" {
			problems = append(problems, fmt.Sprintf("weekday slot %d has empty bounds", i))
		}
	}
	return problems
}
