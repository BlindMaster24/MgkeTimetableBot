package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func main() {
	cfg, err := config.Load(os.Args[1])
	if err != nil {
		panic(err)
	}

	dir := os.TempDir() + "/mgke-probe-cache"
	os.RemoveAll(dir)
	c, err := cache.New(dir)
	if err != nil {
		panic(err)
	}
	log := logger.New("error", nil)
	fetcher := parser.NewFetcher(log, c, parser.Options{Guard: parser.Guard{Disabled: cfg.Parser.Guard.Disabled, MinItems: cfg.Parser.Guard.MinItems, MaxDropPercent: cfg.Parser.Guard.MaxDropPercent}})
	if err := fetcher.Timetable(context.Background(), cfg.Parser.Endpoints.TimetableGroup, cfg.Parser.Endpoints.TimetableTeacher); err != nil {
		panic(err)
	}

	var cached struct {
		Timetable map[string]map[string]any `json:"timetable"`
	}
	raw, err := os.ReadFile(os.Args[2])
	if err == nil {
		_ = json.Unmarshal(raw, &cached)
	}

	freshAll := c.GetGroups()
	for _, group := range []string{"106", "167"} {
		fresh, _ := freshAll[group].(map[string]any)
		if fresh == nil {
			fmt.Println(group, "missing in the fresh parse")
			continue
		}
		oldDays, _ := cached.Timetable[group]["days"].([]any)
		newDays, _ := fresh["days"].([]any)
		fmt.Println("=== group", group, "cached days:", len(oldDays), "fresh days:", len(newDays))
		for i := range newDays {
			if i >= len(oldDays) {
				continue
			}
			o, _ := oldDays[i].(map[string]any)
			n, _ := newDays[i].(map[string]any)
			oj, _ := json.Marshal(o)
			nj, _ := json.Marshal(n)
			if string(oj) != string(nj) {
				fmt.Println("day", o["day"], "differs")
				fmt.Println("  cached:", cut(string(oj), 420))
				fmt.Println("  fresh :", cut(string(nj), 420))
			}
		}
	}
}

func cut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
