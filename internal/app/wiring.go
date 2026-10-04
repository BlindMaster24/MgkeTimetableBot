package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	parserpkg "github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

var apiProbeClient = &http.Client{Timeout: apiprobe.DefaultTimeout}

func healthThresholds(cfg *config.Config) health.Thresholds {
	thresholds := health.DefaultThresholds()
	if cfg.Health == nil {
		return thresholds
	}

	thresholds.ParserStale = time.Duration(cfg.Health.ParserStaleMinutes) * time.Minute
	thresholds.ParserFailures = cfg.Health.ParserFailures
	thresholds.ParserLayout = cfg.Health.ParserLayoutFailures
	thresholds.ParserGuard = cfg.Health.ParserGuardFailures
	thresholds.CalendarStale = time.Duration(cfg.Health.CalendarStaleMinutes) * time.Minute
	thresholds.CalendarFailures = cfg.Health.CalendarFailures
	thresholds.APIErrors = cfg.Health.APIErrors
	thresholds.APIWindow = time.Duration(cfg.Health.APIWindowMinutes) * time.Minute
	thresholds.APISlow = time.Duration(cfg.Health.APISlowMS) * time.Millisecond

	return thresholds.WithDefaults()
}

func apiProbeBaseURL(cfg *config.Config) string {
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTP.Port)
}

func parserGuard(cfg *config.Config) parserpkg.Guard {
	return parserpkg.Guard{
		Disabled:       cfg.Parser.Guard.Disabled,
		MinItems:       cfg.Parser.Guard.MinItems,
		MaxDropPercent: cfg.Parser.Guard.MaxDropPercent,
	}
}

func parserLayoutIssues(report parserpkg.Report) []health.LayoutIssue {
	issues := make([]health.LayoutIssue, 0, len(report.Failing()))
	for _, probe := range report.Failing() {
		issues = append(issues, health.LayoutIssue{
			Source:   report.Source,
			Selector: probe.Selector,
			Expected: probe.Expected,
			Found:    probe.Found,
		})
	}
	return issues
}

func guardIssues(report parserpkg.Report) []health.GuardIssue {
	var issues []health.GuardIssue

	if report.Keep != nil {
		issues = append(issues, health.GuardIssue{
			Source: report.Source,
			Reason: report.Keep.Reason,
			Detail: report.Keep.Summary(),
		})
	}
	for _, name := range report.Fallbacks {
		issues = append(issues, health.GuardIssue{
			Source: report.Source,
			Reason: "fallback",
			Detail: name,
		})
	}

	return issues
}

func googleDayChanges(changes []cache.DayChange) []telegrambot.GoogleDayChange {
	result := make([]telegrambot.GoogleDayChange, 0, len(changes))
	for _, change := range changes {
		kind := "group"
		if change.Kind == cache.KindTeachers {
			kind = "teacher"
		}
		result = append(result, telegrambot.GoogleDayChange{Type: kind, Value: change.Value, Date: change.Date})
	}
	return result
}
