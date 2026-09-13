package parser

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func testLoopConfig() LoopConfig {
	return LoopConfig{
		Enabled:          true,
		Activity:         [2]int{9, 17},
		Default:          time.Hour,
		ActivityInterval: 30 * time.Second,
		ErrorDelay:       time.Minute,
		CallsEnabled:     true,
		CallsInterval:    time.Hour,
		TeamInterval:     24 * time.Hour,
	}
}

func TestLoopConfigFromDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Parser.Enabled = true

	loop := LoopConfigFrom(cfg)

	if !loop.Enabled {
		t.Error("expected the loop to follow parser.enabled")
	}
	if loop.Default != defaultInterval || loop.ActivityInterval != defaultActivity {
		t.Errorf("defaults not applied: %+v", loop)
	}
	if loop.ErrorDelay != defaultErrorDelay || loop.CallsInterval != defaultCallsInterval {
		t.Errorf("error/calls defaults not applied: %+v", loop)
	}
	if loop.TeamInterval != defaultTeamsInterval {
		t.Errorf("team interval = %v", loop.TeamInterval)
	}
	if loop.Activity != defaultActivityWindow {
		t.Errorf("activity window = %v", loop.Activity)
	}
	if !loop.CallsEnabled {
		t.Error("calls should default to enabled when the section is absent")
	}
}

func TestLoopConfigFromValues(t *testing.T) {
	cfg := &config.Config{}
	cfg.Parser.Enabled = true
	cfg.Parser.Activity = [2]int{8, 20}
	cfg.Parser.UpdateInterval.Default = 600
	cfg.Parser.UpdateInterval.Activity = 20
	cfg.Parser.UpdateInterval.Error = 30
	cfg.Parser.UpdateInterval.Calls = 120
	cfg.Parser.UpdateInterval.Teams = 43200
	cfg.Parser.Calls = &config.CallsConfig{Enabled: false}

	loop := LoopConfigFrom(cfg)

	if loop.Default != 10*time.Minute || loop.ActivityInterval != 20*time.Second {
		t.Errorf("intervals = %+v", loop)
	}
	if loop.ErrorDelay != 30*time.Second || loop.CallsInterval != 2*time.Minute {
		t.Errorf("error/calls intervals = %+v", loop)
	}
	if loop.TeamInterval != 12*time.Hour {
		t.Errorf("team interval = %v", loop.TeamInterval)
	}
	if loop.CallsEnabled {
		t.Error("calls.enabled=false should switch the calls parser off")
	}
	if loop.Activity != [2]int{8, 20} {
		t.Errorf("activity = %v", loop.Activity)
	}
}

func TestNextDelay(t *testing.T) {
	loop := testLoopConfig()

	t.Run("failed parse retries soon", func(t *testing.T) {
		now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
		if got := NextDelay(loop, now, true); got != loop.ErrorDelay {
			t.Errorf("delay = %v, want %v", got, loop.ErrorDelay)
		}
	})

	t.Run("activity hours poll often", func(t *testing.T) {
		now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
		if got := NextDelay(loop, now, false); got != loop.ActivityInterval {
			t.Errorf("delay = %v, want %v", got, loop.ActivityInterval)
		}
	})

	t.Run("sunday is never an activity day", func(t *testing.T) {
		sunday := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		if sunday.Weekday() != time.Sunday {
			t.Fatalf("fixture is not a Sunday: %v", sunday.Weekday())
		}
		if got := NextDelay(loop, sunday, false); got != loop.Default {
			t.Errorf("delay = %v, want %v", got, loop.Default)
		}
	})

	t.Run("night waits for the activity window", func(t *testing.T) {
		now := time.Date(2026, 9, 14, 8, 45, 0, 0, time.UTC)
		got := NextDelay(loop, now, false)
		want := 15 * time.Minute
		if got != want {
			t.Errorf("delay = %v, want %v", got, want)
		}
	})

	t.Run("deep night uses the regular interval", func(t *testing.T) {
		now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
		if got := NextDelay(loop, now, false); got != loop.Default {
			t.Errorf("delay = %v, want %v", got, loop.Default)
		}
	})
}
