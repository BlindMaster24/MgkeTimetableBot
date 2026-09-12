package cache

import "testing"

func testSiteSchedule() Schedule {
	return Schedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
	}
}

func testManualSchedule() (Schedule, Schedule) {
	manualWeekdays := [][2][2]string{{{"07:30", "08:15"}, {"08:25", "09:10"}}}
	return Schedule{
		Weekdays: manualWeekdays,
		Saturday: manualWeekdays,
	}, Schedule{}
}

func TestCallsAutoPrefersSiteWhenFresh(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	site := testSiteSchedule()
	c.SetCalls(site, Schedule{}, "site")

	calls := c.GetCalls()
	if calls.Active.Source != "site" {
		t.Fatalf("auto source should be site, got %q", calls.Active.Source)
	}
	if len(calls.Active.Schedule.Weekdays) != 1 {
		t.Fatalf("expected site schedule active, got %d slots", len(calls.Active.Schedule.Weekdays))
	}
}

func TestCallsOverrideSiteAndReset(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	site := testSiteSchedule()
	c.SetCalls(site, Schedule{}, "site")

	c.SetCallsOverride("manual")
	calls := c.GetCalls()
	if calls.OverrideSource != "manual" {
		t.Fatalf("override should be manual, got %q", calls.OverrideSource)
	}
	if calls.Active.Source == "site" {
		t.Errorf("override must move active away from site, got %q", calls.Active.Source)
	}

	c.ResetCallsOverride()
	calls = c.GetCalls()
	if calls.OverrideSource != "" {
		t.Fatalf("override should clear, got %q", calls.OverrideSource)
	}
	if calls.Active.Source != "site" {
		t.Fatalf("auto should select site again, got %q", calls.Active.Source)
	}
}

func TestCallsOverrideConfigKeepsSourceOnly(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetCalls(testSiteSchedule(), Schedule{}, "site")
	c.SetCallsOverride("config")

	calls := c.GetCalls()
	if calls.Active.Source != "config" {
		t.Fatalf("active source: got %q", calls.Active.Source)
	}
	if len(calls.Site.Schedule.Weekdays) != 1 {
		t.Error("site data must be preserved for the config override")
	}
}

func TestCallsParseKeepsOverride(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetCalls(testSiteSchedule(), Schedule{}, "site")
	c.SetCallsOverride("site")

	updated := Schedule{
		Weekdays: [][2][2]string{
			{{"10:00", "10:45"}, {"10:55", "11:40"}},
			{{"12:00", "12:45"}, {"12:55", "13:40"}},
		},
		Saturday: testSiteSchedule().Saturday,
	}
	c.SetCalls(updated, Schedule{}, "site")

	calls := c.GetCalls()
	if calls.OverrideSource != "site" {
		t.Fatalf("override must survive a parse, got %q", calls.OverrideSource)
	}
	if calls.Active.Source != "site" {
		t.Fatalf("active source: got %q", calls.Active.Source)
	}
	if len(calls.Active.Schedule.Weekdays) != 2 {
		t.Fatalf("active schedule should follow the site, got %d slots", len(calls.Active.Schedule.Weekdays))
	}
}

func TestCallsManualOverrideSelectsManualData(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetCalls(testSiteSchedule(), Schedule{}, "site")
	c.DrainEvents()

	manual, _ := testManualSchedule()
	c.SetCallsManualNotify(manual.Weekdays, manual.Saturday, "репетиция", false)

	calls := c.GetCalls()
	if calls.OverrideSource != "manual" {
		t.Fatalf("manual edit should set the override, got %q", calls.OverrideSource)
	}
	if calls.Active.Source != "manual" {
		t.Fatalf("active source: got %q", calls.Active.Source)
	}
	if calls.Active.Schedule.Weekdays[0][0][0] != "07:30" {
		t.Errorf("manual schedule not active: %v", calls.Active.Schedule.Weekdays)
	}
	if calls.ManualReason != "репетиция" {
		t.Errorf("manual reason: got %q", calls.ManualReason)
	}
	if len(c.DrainEvents()) != 0 {
		t.Error("notify=false must not emit a calls event")
	}
}

func TestCallsManualNotifyEmitsEvent(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c.SetCalls(testSiteSchedule(), Schedule{}, "site")
	c.DrainEvents()

	manual, _ := testManualSchedule()
	c.SetCallsManualNotify(manual.Weekdays, manual.Saturday, "приказ", true)

	events := c.DrainEvents()
	if len(events) != 1 || events[0].Calls == nil {
		t.Fatalf("expected one calls event, got %d", len(events))
	}
	if !events[0].Calls.WeekdaysChanged {
		t.Error("expected weekdaysChanged in the calls event")
	}
	if events[0].Calls.Reason != "приказ" {
		t.Errorf("reason: got %q", events[0].Calls.Reason)
	}
}

func TestCallsPreferSiteFalseFavoursManual(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.SetCallsPreferSite(false)

	manual, _ := testManualSchedule()
	c.SetCalls(testSiteSchedule(), manual, "site")

	calls := c.GetCalls()
	if calls.Active.Source != "manual" {
		t.Fatalf("prefer_site=false should select manual, got %q", calls.Active.Source)
	}
}
