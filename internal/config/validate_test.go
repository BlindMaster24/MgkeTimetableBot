package config

import (
	"strings"
	"testing"
)

func validTestConfig() *Config {
	cfg := &Config{}
	cfg.HTTP.Port = 8081
	cfg.Telegram.Token = "test-token"
	cfg.Parser.Activity = [2]int{9, 17}
	cfg.Timetable.Weekdays = [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}
	cfg.Timetable.Saturday = [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}
	cfg.Parser.Guard = GuardConfig{MinItems: 10, MaxDropPercent: 80}
	return cfg
}

func TestValidateAcceptsAWorkingConfig(t *testing.T) {
	if err := validTestConfig().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateAcceptsEmptyTimetable(t *testing.T) {
	cfg := validTestConfig()
	cfg.Timetable = TimetableConfig{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsBrokenFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"port", func(c *Config) { c.HTTP.Port = 0 }, "http.port"},
		{"token", func(c *Config) { c.Telegram.Token = "  " }, "telegram.token"},
		{"activity", func(c *Config) { c.Parser.Activity = [2]int{17, 9} }, "parser.activity"},
		{"weekday", func(c *Config) { c.Timetable.Weekdays[0][0][0] = "25:00" }, "timetable.weekdays"},
		{"saturday", func(c *Config) { c.Timetable.Saturday[0][1][1] = "09:00" }, "timetable.saturday"},
		{"guard_min", func(c *Config) { c.Parser.Guard.MinItems = -1 }, "parser.guard.min_items"},
		{"guard_drop", func(c *Config) { c.Parser.Guard.MaxDropPercent = 101 }, "parser.guard.max_drop_percent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validTestConfig()
			tc.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() = %q, want it to mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateCollectsEveryProblem(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil for an empty config, want an error")
	} else if !strings.Contains(err.Error(), "http.port") || !strings.Contains(err.Error(), "telegram.token") {
		t.Errorf("Validate() = %q, want both http.port and telegram.token", err.Error())
	}
}
