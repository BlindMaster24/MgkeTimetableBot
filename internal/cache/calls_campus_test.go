package cache

import "testing"

func TestActiveCallsForCampusFollowsTheSiteVariants(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	kazinca := [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}}
	knorina := [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}

	c.SetCallsNotify(t.Context(), Schedule{Weekdays: kazinca}, Schedule{}, "site", "")
	c.SetCallsSiteVariants(t.Context(), []CallsVariant{
		{Name: "Казинца", Schedule: CallsSchedule{Weekdays: kazinca, Saturday: kazinca}},
		{Name: "Кнорина", Schedule: CallsSchedule{Weekdays: knorina, Saturday: knorina}},
	})

	fallback := CallsSchedule{Weekdays: kazinca, Saturday: kazinca}

	if got := c.ActiveCallsForCampus("Кнорина", fallback); got.Weekdays[0] != knorina[0] {
		t.Fatalf("the Кнорина campus was not selected: %v", got.Weekdays)
	}
	if got := c.ActiveCallsForCampus("", fallback); got.Weekdays[0] != kazinca[0] {
		t.Fatalf("an empty campus must keep the active schedule: %v", got.Weekdays)
	}
	if got := c.ActiveCallsForCampus("Неизвестный", fallback); got.Weekdays[0] != kazinca[0] {
		t.Fatalf("an unknown campus must keep the active schedule: %v", got.Weekdays)
	}
	if got := c.ActiveCallsForCampus("Кнорина", fallback); len(got.Saturday) == 0 {
		t.Fatal("a campus without saturday slots must reuse the weekday ones")
	}
}

func TestActiveCallsForCampusIgnoresTheCampusForAManualSource(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	manual := [][2][2]string{{{"10:00", "10:45"}, {"10:55", "11:40"}}}
	site := [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}}
	c.SetCallsNotify(t.Context(), Schedule{Weekdays: site}, Schedule{Weekdays: manual}, "manual", "")
	c.SetCallsSiteVariants(t.Context(), []CallsVariant{
		{Name: "Кнорина", Schedule: CallsSchedule{Weekdays: site, Saturday: site}},
	})

	got := c.ActiveCallsForCampus("Кнорина", CallsSchedule{})
	if got.Weekdays[0] != manual[0] {
		t.Fatalf("a manual schedule must win over a campus: %v", got.Weekdays)
	}
}

func TestActiveCallsForCampusFallsBackWhenNothingIsStored(t *testing.T) {
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	fallback := [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}
	got := c.ActiveCallsForCampus("", CallsSchedule{Weekdays: fallback})
	if got.Weekdays[0] != fallback[0] {
		t.Fatalf("the fallback schedule was not used: %v", got.Weekdays)
	}
	if got.Saturday[0] != fallback[0] {
		t.Fatalf("saturday must fall back to the weekday slots: %v", got.Saturday)
	}
}
