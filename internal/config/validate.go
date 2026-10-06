package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func (c *Config) Validate() error {
	var errs []error
	if c == nil {
		return errors.New("config is nil")
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		errs = append(errs, fmt.Errorf("http.port must be between 1 and 65535, got %d", c.HTTP.Port))
	}
	if strings.TrimSpace(c.Telegram.Token) == "" {
		errs = append(errs, errors.New("telegram.token is required"))
	}
	if strings.TrimSpace(c.ResolvedChatDBPath()) == "" {
		errs = append(errs, errors.New("chat_db_path must not be empty"))
	}
	if strings.TrimSpace(c.ResolvedCacheDir()) == "" {
		errs = append(errs, errors.New("cache_dir must not be empty"))
	}
	activity := c.Parser.Activity
	if activity[0] < 0 || activity[0] > 23 || activity[1] < 0 || activity[1] > 23 || activity[0] > activity[1] {
		errs = append(errs, fmt.Errorf("parser.activity must hold two hours 0-23 with start <= end, got %v", activity))
	}
	for i, slot := range c.Timetable.Weekdays {
		if err := checkSlot("timetable.weekdays", i, slot); err != nil {
			errs = append(errs, err)
		}
	}
	for i, slot := range c.Timetable.Saturday {
		if err := checkSlot("timetable.saturday", i, slot); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Parser.Guard.MinItems < 0 {
		errs = append(errs, fmt.Errorf("parser.guard.min_items must not be negative, got %d", c.Parser.Guard.MinItems))
	}
	if c.Parser.Guard.MaxDropPercent < 0 || c.Parser.Guard.MaxDropPercent > 100 {
		errs = append(errs, fmt.Errorf("parser.guard.max_drop_percent must be between 0 and 100, got %d", c.Parser.Guard.MaxDropPercent))
	}
	if c.Archive.BackupKeep < 0 {
		errs = append(errs, fmt.Errorf("archive.backup_keep must not be negative, got %d", c.Archive.BackupKeep))
	}
	if c.Archive.RetentionDays < 0 {
		errs = append(errs, fmt.Errorf("archive.retention_days must not be negative, got %d", c.Archive.RetentionDays))
	}
	if c.MaintenanceEnabled() {
		if _, err := maintenanceScheduleParser.Parse(c.ResolvedMaintenanceSchedule()); err != nil {
			errs = append(errs, fmt.Errorf("archive.maintenance_schedule must be a cron expression with seconds, got %q: %w", c.Archive.MaintenanceSchedule, err))
		}
	}
	if c.Metrics.Enabled {
		path := c.ResolvedMetricsPath()
		if !strings.HasPrefix(path, "/") || len(path) < 2 {
			errs = append(errs, fmt.Errorf("metrics.path must be an absolute path like /metrics, got %q", c.Metrics.Path))
		}
		if strings.HasPrefix(path, "/api") {
			errs = append(errs, fmt.Errorf("metrics.path must stay outside /api, otherwise the REST API key gate applies to it, got %q", path))
		}
		if token := strings.TrimSpace(c.Metrics.Token); token != "" && len(token) < MinimumMetricsToken {
			errs = append(errs, fmt.Errorf("metrics.token must be at least %d characters long when it is set, got %d", MinimumMetricsToken, len(token)))
		}
	}
	return errors.Join(errs...)
}

var maintenanceScheduleParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func checkSlot(section string, index int, slot [2][2]string) error {
	for half := 0; half < 2; half++ {
		start, err := time.Parse("15:04", strings.TrimSpace(slot[half][0]))
		if err != nil {
			return fmt.Errorf("%s[%d] holds invalid time %q, want HH:MM", section, index, slot[half][0])
		}
		end, err := time.Parse("15:04", strings.TrimSpace(slot[half][1]))
		if err != nil {
			return fmt.Errorf("%s[%d] holds invalid time %q, want HH:MM", section, index, slot[half][1])
		}
		if !start.Before(end) {
			return fmt.Errorf("%s[%d] starts at %q but ends at %q", section, index, slot[half][0], slot[half][1])
		}
	}
	return nil
}
