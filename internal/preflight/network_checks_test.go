package preflight

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestCheckGroupsAcceptsAHealthyPage(t *testing.T) {
	server := pageServer(t)
	defer server.Close()

	groups, check := checkGroups(server.Client(), server.URL+"/groups")
	t.Logf("groups=%d level=%s detail=%s hints=%v", len(groups), check.Level, check.Detail, check.Hints)
	if len(groups) == 0 {
		t.Fatalf("the healthy page produced no groups: %s %v", check.Detail, check.Hints)
	}
	if check.Level != LevelOK {
		t.Fatalf("level = %s, want ok (hints %v)", check.Level, check.Hints)
	}
}

func TestCheckGroupsReportsAMissingPage(t *testing.T) {
	server := pageServer(t)
	defer server.Close()

	if _, check := checkGroups(server.Client(), server.URL+"/missing"); check.Level != LevelFail {
		t.Fatalf("level = %s, want fail for a 404", check.Level)
	}
	if _, check := checkGroups(server.Client(), " "); check.Level != LevelSkip {
		t.Fatalf("level = %s, want skip without an endpoint", check.Level)
	}
}

func TestCheckTeachersAcceptsAHealthyPage(t *testing.T) {
	server := pageServer(t)
	defer server.Close()

	check := checkTeachers(server.Client(), server.URL+"/teachers")
	t.Logf("level=%s detail=%s hints=%v", check.Level, check.Detail, check.Hints)
	if check.Level != LevelOK {
		t.Fatalf("level = %s, want ok (hints %v)", check.Level, check.Hints)
	}
}

func TestCheckTimetableFlagsAnEmptyWeek(t *testing.T) {
	cfg := &config.Config{}
	check := checkTimetable(cfg)
	if check.Level != LevelWarn {
		t.Fatalf("level = %s, want a warning for an empty week", check.Level)
	}

	cfg.Timetable.Weekdays = [][2][2]string{
		{{"10:00", "11:30"}, {"11:40", "13:10"}},
		{{"13:20", "14:50"}, {"15:00", "16:30"}},
		{{"16:40", "18:10"}, {"18:20", "19:50"}},
		{{"20:00", "21:30"}, {"21:40", "23:10"}},
	}
	cfg.Timetable.Saturday = [][2][2]string{
		{{"10:00", "11:30"}, {"11:40", "13:10"}},
		{{"13:20", "14:50"}, {"15:00", "16:30"}},
	}
	if healthy := checkTimetable(cfg); healthy.Level != LevelOK {
		t.Fatalf("level = %s, want ok for a full week: %v", healthy.Level, healthy.Hints)
	}
}
