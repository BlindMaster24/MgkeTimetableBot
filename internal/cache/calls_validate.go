package cache

import (
	"fmt"
	"strings"
	"time"
)

func (s CallsSchedule) Validate() error {
	for i, slot := range s.Weekdays {
		if err := checkCallSlot("weekdays", i, slot); err != nil {
			return err
		}
	}
	for i, slot := range s.Saturday {
		if err := checkCallSlot("saturday", i, slot); err != nil {
			return err
		}
	}
	return nil
}

func checkCallSlot(section string, index int, slot [2][2]string) error {
	for half := 0; half < 2; half++ {
		start, err := time.Parse("15:04", strings.TrimSpace(slot[half][0]))
		if err != nil {
			return fmt.Errorf("calls %s[%d] holds invalid time %q, want HH:MM", section, index, slot[half][0])
		}
		end, err := time.Parse("15:04", strings.TrimSpace(slot[half][1]))
		if err != nil {
			return fmt.Errorf("calls %s[%d] holds invalid time %q, want HH:MM", section, index, slot[half][1])
		}
		if !start.Before(end) {
			return fmt.Errorf("calls %s[%d] starts at %q but ends at %q", section, index, slot[half][0], slot[half][1])
		}
	}
	return nil
}
