package cache

import (
	"strings"
	"testing"
)

func TestCallsScheduleValidateAcceptsWorkingHours(t *testing.T) {
	schedule := CallsSchedule{
		Weekdays: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
		Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
	}
	if err := schedule.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if err := (CallsSchedule{}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for an empty schedule", err)
	}
}

func TestCallsScheduleValidateRejectsBrokenSlots(t *testing.T) {
	cases := []struct {
		name     string
		schedule CallsSchedule
		want     string
	}{
		{"weekday", CallsSchedule{Weekdays: [][2][2]string{{{"25:00", "09:45"}, {"09:55", "10:40"}}}}, "calls weekdays[0]"},
		{"saturday", CallsSchedule{Saturday: [][2][2]string{{{"09:00", "09:45"}, {"10:40", "09:55"}}}}, "calls saturday[0]"},
		{"reversed", CallsSchedule{Weekdays: [][2][2]string{{{"10:40", "09:00"}, {"09:55", "10:40"}}}}, "starts at"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schedule.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() = %q, want it to mention %q", err.Error(), tc.want)
			}
		})
	}
}
