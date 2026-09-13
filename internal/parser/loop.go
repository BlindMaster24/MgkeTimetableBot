package parser

import (
	"context"
	"math"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

const (
	defaultInterval      = time.Hour
	defaultActivity      = 30 * time.Second
	defaultErrorDelay    = time.Minute
	defaultTeamsInterval = 24 * time.Hour
	defaultCallsInterval = time.Hour
)

var defaultActivityWindow = [2]int{9, 17}

type LoopConfig struct {
	Enabled          bool
	Activity         [2]int
	Default          time.Duration
	ActivityInterval time.Duration
	ErrorDelay       time.Duration
	CallsEnabled     bool
	CallsInterval    time.Duration
	TeamInterval     time.Duration
}

func LoopConfigFrom(cfg *config.Config) LoopConfig {
	loop := LoopConfig{
		Enabled:          cfg.Parser.Enabled,
		Activity:         cfg.Parser.Activity,
		Default:          time.Duration(cfg.Parser.UpdateInterval.Default) * time.Second,
		ActivityInterval: time.Duration(cfg.Parser.UpdateInterval.Activity) * time.Second,
		ErrorDelay:       time.Duration(cfg.Parser.UpdateInterval.Error) * time.Second,
		CallsInterval:    time.Duration(cfg.Parser.UpdateInterval.Calls) * time.Second,
		TeamInterval:     time.Duration(cfg.Parser.UpdateInterval.Teams) * time.Second,
	}
	if cfg.Parser.Calls == nil || cfg.Parser.Calls.Enabled {
		loop.CallsEnabled = true
	}

	if loop.Default <= 0 {
		loop.Default = defaultInterval
	}
	if loop.ActivityInterval <= 0 {
		loop.ActivityInterval = defaultActivity
	}
	if loop.ErrorDelay <= 0 {
		loop.ErrorDelay = defaultErrorDelay
	}
	if loop.CallsInterval <= 0 {
		loop.CallsInterval = defaultCallsInterval
	}
	if loop.TeamInterval <= 0 {
		loop.TeamInterval = defaultTeamsInterval
	}
	if loop.Activity[0] <= 0 || loop.Activity[1] <= 0 || loop.Activity[0] >= loop.Activity[1] {
		loop.Activity = defaultActivityWindow
	}

	return loop
}

func NextDelay(cfg LoopConfig, now time.Time, failed bool) time.Duration {
	if failed {
		return cfg.ErrorDelay
	}

	hour := now.Hour()
	if now.Weekday() != time.Sunday && cfg.Activity[0] <= hour && hour <= cfg.Activity[1] {
		return cfg.ActivityInterval
	}

	startHour := cfg.Activity[0] - int(math.Ceil(cfg.Default.Hours()))
	if hour >= startHour && hour <= cfg.Activity[0] {
		aligned := time.Date(now.Year(), now.Month(), now.Day(), cfg.Activity[0], 0, 0, 0, now.Location())
		if now.Before(aligned) && !now.Add(cfg.Default).Before(aligned) {
			return aligned.Sub(now)
		}
	}

	return cfg.Default
}

func RunLoop(ctx context.Context, cfg LoopConfig, log *logger.Logger, parse func() error) {
	if !cfg.Enabled {
		return
	}

	delay := NextDelay(cfg, time.Now(), false)
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		err := parse()
		delay = NextDelay(cfg, time.Now(), err != nil)
		log.Info().
			Dur("delay", delay.Round(time.Second)).
			Bool("error", err != nil).
			Msg("next parse scheduled")
	}
}
