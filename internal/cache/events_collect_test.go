package cache

import (
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func dayEventFixture(date string, lessons ...any) map[string]any {
	if lessons == nil {
		lessons = []any{}
	}
	return map[string]any{"day": date, "lessons": lessons}
}

func entryFixture(days ...any) map[string]any {
	return map[string]any{"days": days}
}

func lessonFixture(subject string) map[string]any {
	return map[string]any{"lesson": subject, "cabinet": "101"}
}

func TestCollectEntryDayEventsKeepsOnlyTomorrowAndToday(t *testing.T) {
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	todayIdx := utils.DayIndexFromDate(today)
	tomorrow := today.AddDate(0, 0, 1).Format("02.01.2006")
	later := today.AddDate(0, 0, 3).Format("02.01.2006")
	todayDate := today.Format("02.01.2006")

	old := entryFixture(dayEventFixture(todayDate, lessonFixture("Математика")))
	newEntry := entryFixture(
		dayEventFixture(todayDate, lessonFixture("Физика")),
		dayEventFixture(tomorrow, lessonFixture("Химия")),
		dayEventFixture(later, lessonFixture("Биология")),
	)

	out, changes := collectEntryDayEvents(KindGroups, "100", old, newEntry, todayIdx)

	types := map[string]string{}
	for _, event := range out {
		if event.ev.Day == nil {
			continue
		}
		types[event.ev.Day.Day["day"].(string)] = event.evType
	}
	if types[todayDate] != DayEventUpdate {
		t.Fatalf("today's change must be an update, got %v", types)
	}
	if _, ok := types[tomorrow]; ok {
		t.Fatalf("a day the entry never had is not a day event, got %v", types)
	}
	if _, ok := types[later]; ok {
		t.Fatalf("a day three days ahead must not raise an event, got %v", types)
	}

	// The calendar sync must hear about the changed day and about the days the
	// entry did not have before, and about nothing else.
	dates := map[string]bool{}
	for _, change := range changes {
		dates[change.Date] = true
	}
	for _, want := range []string{todayDate, tomorrow, later} {
		if !dates[want] {
			t.Fatalf("the change log must name %s: %+v", want, changes)
		}
	}
	if len(changes) != 3 {
		t.Fatalf("unchanged days must stay out of the change log, got %+v", changes)
	}

	var diffSeen bool
	for _, event := range out {
		if event.ev.Day != nil && event.ev.Day.Diff != nil {
			diffSeen = true
		}
	}
	if !diffSeen {
		t.Fatal("today's change must carry a diff")
	}
}

func TestCollectEntryDayEventsTreatsTheNoticedTomorrowAsAnUpdate(t *testing.T) {
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	todayIdx := utils.DayIndexFromDate(today)
	tomorrow := today.AddDate(0, 0, 1).Format("02.01.2006")

	old := map[string]any{
		"days":            []any{dayEventFixture(tomorrow, lessonFixture("Математика"))},
		"lastNoticedDay":  float64(todayIdx + 1),
		"lastNoticedWeek": float64(0),
	}
	newEntry := entryFixture(dayEventFixture(tomorrow, lessonFixture("Физика")))

	out, _ := collectEntryDayEvents(KindGroups, "100", old, newEntry, todayIdx)
	if len(out) != 1 || out[0].evType != DayEventUpdate {
		t.Fatalf("a day the chat has already seen must count as an update: %+v", out)
	}

	old["lastNoticedDay"] = float64(0)
	out, _ = collectEntryDayEvents(KindGroups, "100", old, newEntry, todayIdx)
	if len(out) != 1 || out[0].evType != DayEventAdd {
		t.Fatalf("an unseen day must count as an add: %+v", out)
	}
}

func TestCollectEntryDayEventsIgnoresTheUnchangedAndTheMalformed(t *testing.T) {
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	todayIdx := utils.DayIndexFromDate(today)
	todayDate := today.Format("02.01.2006")

	if out, changes := collectEntryDayEvents(KindGroups, "100", entryFixture(dayEventFixture(todayDate, lessonFixture("Математика"))), entryFixture(dayEventFixture(todayDate, lessonFixture("Математика"))), todayIdx); out != nil || changes != nil {
		t.Fatalf("an unchanged entry must stay silent, got %+v %+v", out, changes)
	}

	old := entryFixture(dayEventFixture(todayDate, lessonFixture("Математика")))
	broken := entryFixture(
		"not a day",
		map[string]any{"lessons": []any{lessonFixture("Физика")}},
		dayEventFixture("", lessonFixture("Химия")),
	)
	out, changes := collectEntryDayEvents(KindGroups, "100", old, broken, todayIdx)
	if len(out) != 0 {
		t.Fatalf("malformed days must not raise events, got %+v", out)
	}
	for _, change := range changes {
		if change.Date == "" {
			t.Fatalf("a day without a date must not reach the change log: %+v", changes)
		}
	}
}

func TestCollectWeekEntriesListsTheGroupsWithLessons(t *testing.T) {
	date := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	week := utils.WeekIndexFromDate(date).Value()
	other := date.AddDate(0, 0, 7)

	tt := map[string]any{
		"100": entryFixture(dayEventFixture(date.Format("02.01.2006"), lessonFixture("Математика"))),
		"200": entryFixture(dayEventFixture(other.Format("02.01.2006"), lessonFixture("Физика"))),
		"300": entryFixture(dayEventFixture(date.Format("02.01.2006"), []any{}...)),
		"400": entryFixture(),
	}

	entries := collectWeekEntries(tt, week)
	if len(entries) != 1 || entries[0] != "100" {
		t.Fatalf("collectWeekEntries = %v, want only the group with lessons that week", entries)
	}
}
