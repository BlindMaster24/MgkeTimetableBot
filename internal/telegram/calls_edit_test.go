package telegram

import "testing"

func TestParseCallsInputKeepsAWorkingSchedule(t *testing.T) {
	parsed := parseCallsInput("Будни\n1. 08:00-08:45 / 08:55-09:20\n2. 09:30-10:15 / 10:25-11:10\nСуббота\n1. 09:00-09:45 / 09:55-10:40", testLoc)
	if parsed == nil {
		t.Fatal("a valid bell schedule was dropped")
	}
	if len(parsed.Weekdays) != 2 || len(parsed.Saturday) != 1 {
		t.Fatalf("parsed %d weekday rows and %d saturday rows", len(parsed.Weekdays), len(parsed.Saturday))
	}
	if parsed.Weekdays[0] != [2][2]string{{"08:00", "08:45"}, {"08:55", "09:20"}} {
		t.Fatalf("the first weekday slot = %v", parsed.Weekdays[0])
	}
	if parsed.Saturday[0] != [2][2]string{{"09:00", "09:45"}, {"09:55", "10:40"}} {
		t.Fatalf("the saturday slot = %v", parsed.Saturday[0])
	}
}

func TestParseCallsInputRejectsAScheduleTheCacheRefuses(t *testing.T) {
	for _, input := range []string{
		"1. 10:00-09:00 / 10:10-11:00",
		"1. 25:00-26:00 / 27:00-28:00",
		"1. 10:00-10:99 / 11:00-11:45",
	} {
		if parsed := parseCallsInput(input, testLoc); parsed != nil {
			t.Errorf("parseCallsInput(%q) accepted a slot the schedule cannot use: %+v", input, parsed)
		}
	}
}
