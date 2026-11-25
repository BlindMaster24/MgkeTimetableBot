package telegram

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func TestRelevantWeekIgnoresALaggingTeacherTable(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 777)

	cur := utils.WeekIndexFromDate(utils.SiteNow()).Value()
	b.cache.Groups = &cache.RaspEntry[cache.GroupTimetable]{LastWeekIndex: cur}
	b.cache.Teachers = &cache.RaspEntry[cache.TeacherTimetable]{LastWeekIndex: cur - 1}

	if got := b.relevantWeekIndex().Value(); got != cur {
		t.Fatalf("a teacher table one week behind dragged the week to %d, want %d", got, cur)
	}
}

func TestRelevantWeekFallsBackWhenNothingIsPublished(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 777)

	cur := utils.WeekIndexFromDate(utils.SiteNow()).Value()
	b.cache.Groups = &cache.RaspEntry[cache.GroupTimetable]{LastWeekIndex: cur - 2}
	b.cache.Teachers = &cache.RaspEntry[cache.TeacherTimetable]{LastWeekIndex: cur - 1}

	if got := b.relevantWeekIndex().Value(); got != cur-1 {
		t.Fatalf("the newest published week must win, got %d, want %d", got, cur-1)
	}
}
