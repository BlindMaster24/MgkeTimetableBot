package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func checkStorage(cfg *config.Config) Check {
	paths := []struct {
		name string
		path string
	}{
		{"db_path", cfg.ResolvedDBPath()},
		{"chat_db_path", cfg.ResolvedChatDBPath()},
		{"cache_dir", cfg.ResolvedCacheDir()},
	}

	var facts []string
	var hints []string
	level := LevelOK

	for _, entry := range paths {
		dir := filepath.Dir(entry.path)
		if dir == "" || dir == "." {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			facts = append(facts, entry.name+" unwritable")
			hints = append(hints, fmt.Sprintf("error: %s: %v", dir, err))
			level = LevelFail
			continue
		}

		probe := filepath.Join(dir, storageProbeName)
		if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
			facts = append(facts, entry.name+" unwritable")
			hints = append(hints, fmt.Sprintf("error: %s: %v", dir, err))
			level = LevelFail
			continue
		}
		os.Remove(probe)
		facts = append(facts, entry.path+" writable")
	}

	if legacy, adopted := config.AdoptLegacyDBPath(cfg.ResolvedDBPath()); adopted {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, fmt.Sprintf("warning: the archive still lives in %s; stop the bot and rename it to %s", legacy, cfg.ResolvedDBPath()))
	}

	return Check{Name: "storage", Level: level, Detail: strings.Join(facts, ", "), Hints: hints}
}

func checkTimetable(cfg *config.Config) Check {
	problems := append(slotProblems("weekdays", cfg.Timetable.Weekdays), slotProblems("saturday", cfg.Timetable.Saturday)...)

	if len(cfg.Timetable.Weekdays) == 0 {
		problems = append(problems, "timetable.weekdays is empty, autoskip and calls would not work")
	}

	if len(problems) > 0 {
		hints := make([]string, 0, len(problems))
		for _, problem := range problems {
			hints = append(hints, "warning: "+problem)
		}
		return Check{
			Name:   "timetable",
			Level:  LevelWarn,
			Detail: fmt.Sprintf("%d weekday slots, %d saturday slots, %d problems", len(cfg.Timetable.Weekdays), len(cfg.Timetable.Saturday), len(problems)),
			Hints:  hints,
		}
	}

	return Check{
		Name:   "timetable",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%d weekday slots, %d saturday slots, all ordered", len(cfg.Timetable.Weekdays), len(cfg.Timetable.Saturday)),
	}
}

func slotProblems(name string, slots [][2][2]string) []string {
	var problems []string
	previousEnd := ""
	for i, slot := range slots {
		firstStart, firstEnd := slot[0][0], slot[0][1]
		secondStart, secondEnd := slot[1][0], slot[1][1]
		label := fmt.Sprintf("timetable.%s slot %d", name, i+1)

		singleBlock := firstStart == secondStart && firstEnd == secondEnd
		if !timeOrdered(firstStart, firstEnd) {
			problems = append(problems, fmt.Sprintf("%s runs from %s to %s", label, firstStart, firstEnd))
		}
		if !singleBlock {
			if !timeOrdered(secondStart, secondEnd) {
				problems = append(problems, fmt.Sprintf("%s second half runs from %s to %s", label, secondStart, secondEnd))
			}
			if timeOrdered(firstStart, firstEnd) && timeOrdered(secondStart, secondEnd) && !timeOrdered(firstEnd, secondStart) {
				problems = append(problems, fmt.Sprintf("%s second half starts at %s before the first ends at %s", label, secondStart, firstEnd))
			}
		}
		if previousEnd != "" && !timeOrdered(previousEnd, firstStart) {
			problems = append(problems, fmt.Sprintf("%s starts at %s before the previous slot ends at %s", label, firstStart, previousEnd))
		}
		previousEnd = secondEnd
	}
	return problems
}

func timeOrdered(from, to string) bool {
	fromMinutes, okFrom := clockMinutes(from)
	toMinutes, okTo := clockMinutes(to)
	if !okFrom || !okTo {
		return false
	}
	return fromMinutes < toMinutes
}

func clockMinutes(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	var hours, minutes int
	if _, err := fmt.Sscanf(parts[0], "%d", &hours); err != nil {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minutes); err != nil {
		return 0, false
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, false
	}
	return hours*60 + minutes, true
}
