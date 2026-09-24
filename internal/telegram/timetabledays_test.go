package telegram

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func weekDayDate(week utils.WeekIndex, offset int) string {
	first, _ := week.WeekRange()
	return first.AddDate(0, 0, offset).Format("02.01.2006")
}

func seedGroupCache(t *testing.T, b *Bot, value string, days ...map[string]any) {
	t.Helper()

	entries := make([]any, 0, len(days))
	for _, day := range days {
		entries = append(entries, day)
	}
	b.cache.SetGroups(map[string]any{
		value: e2eJsonRoundTrip(map[string]any{"group": value, "days": entries}),
	}, "group-hash")
}

func seedTeacherCache(t *testing.T, b *Bot, value string, days ...map[string]any) {
	t.Helper()

	entries := make([]any, 0, len(days))
	for _, day := range days {
		entries = append(entries, day)
	}
	b.cache.SetTeachers(map[string]any{
		value: e2eJsonRoundTrip(map[string]any{"teacher": value, "days": entries}),
	}, "teacher-hash")
}

func lessonOn(date string, lesson string) map[string]any {
	return map[string]any{
		"day":     date,
		"lessons": []any{map[string]any{"lesson": lesson, "type": "Лек", "cabinet": "101"}},
	}
}

func sendText(t *testing.T, b *Bot, userID int64, text string) {
	t.Helper()

	u := makeUpdate(userID, text)
	u.Bot = b
	b.handleMessageText(context.Background(), u)
}

func timetableCallback(t *testing.T, b *Bot, userID int64, typeLetter string, value string, week int) {
	t.Helper()

	data := "timetable_" + typeLetter + ":" + value + ":" + strconv.Itoa(week) + ":0:1"
	b.handleCallback(context.Background(), callbackQuery(userID, 99, data))
}

func TestWeekViewRollsOverForGroupsButNotForTeachers(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(when)
	next := utils.WeekIndexFromNumber(week.Value() + 1)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	b.SetNow(func() time.Time { return when })

	seedGroupCache(t, b, "100",
		lessonOn(weekDayDate(week, 0), "ПрошлыйПонедельник"),
		lessonOn(weekDayDate(week, 1), "ПрошлыйВторник"),
		lessonOn(weekDayDate(next, 0), "БудущаяФизика"),
	)
	seedTeacherCache(t, b, "Иванов И.И.",
		lessonOn(weekDayDate(week, 0), "ПрошлыйПонедельник"),
		lessonOn(weekDayDate(week, 1), "ПрошлыйВторник"),
		lessonOn(weekDayDate(next, 0), "БудущаяФизика"),
	)

	student := chatWithGroup(t, repo, 7401, "100")
	student.HidePastDays = true
	if err := repo.Save(student); err != nil {
		t.Fatal(err)
	}

	sendText(t, b, 7401, "/week")
	text := caller.last()
	if !strings.Contains(text, "БудущаяФизика") {
		t.Fatalf("a group week that only has past days must roll over, got %q", text)
	}
	if strings.Contains(text, "ПрошлыйПонедельник") {
		t.Fatalf("the past days must not be shown after the roll over, got %q", text)
	}
	if !strings.Contains(text, buildWeekLabelFromWeek(next)) {
		t.Fatalf("the rolled over week label is missing, got %q", text)
	}

	teacherChat := chatWithGroup(t, repo, 7402, "")
	teacherChat.Mode = ModeTeacher
	teacherChat.Teacher = "Иванов И.И."
	teacherChat.HidePastDays = true
	if err := repo.Save(teacherChat); err != nil {
		t.Fatal(err)
	}

	caller.reset()
	sendText(t, b, 7402, "/week")
	got := caller.last()
	if !strings.Contains(got, b.loc("no_timetable")) {
		t.Fatalf("a teacher week must not roll over, got %q", got)
	}
	if strings.Contains(got, "БудущаяФизика") {
		t.Fatalf("the next week must stay hidden for a teacher, got %q", got)
	}
}

func TestWeekViewKeepsTheCurrentWeekWhenFutureDaysRemain(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(when)
	next := utils.WeekIndexFromNumber(week.Value() + 1)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	b.SetNow(func() time.Time { return when })

	seedGroupCache(t, b, "100",
		lessonOn(weekDayDate(week, 0), "ПрошлыйПонедельник"),
		lessonOn(weekDayDate(week, 3), "ТекущийЧетверг"),
		lessonOn(weekDayDate(next, 0), "БудущаяФизика"),
	)
	seedTeacherCache(t, b, "Иванов И.И.", lessonOn(weekDayDate(week, 3), "ТекущийЧетверг"))

	chat := chatWithGroup(t, repo, 7403, "100")
	chat.HidePastDays = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	sendText(t, b, 7403, "/week")
	text := caller.last()
	if !strings.Contains(text, "ТекущийЧетверг") {
		t.Fatalf("the remaining days of the current week must be shown, got %q", text)
	}
	if strings.Contains(text, "БудущаяФизика") {
		t.Fatalf("the week must not roll over while future days remain, got %q", text)
	}
}

func TestArchiveFailureShowsNoTimetableInsteadOfTheCache(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(when)

	newBot := func(t *testing.T, withArchive bool) (*Bot, *recordingCaller, int64, *archive.Repository) {
		t.Helper()

		caller := &recordingCaller{}
		b, repo := setupE2EBotWithCaller(t, caller)
		b.SetNow(func() time.Time { return when })
		seedGroupCache(t, b, "100", lessonOn(weekDayDate(week, 3), "КэшТекущейНедели"))
		seedTeacherCache(t, b, "Иванов И.И.", lessonOn(weekDayDate(week, 3), "КэшТекущейНедели"))

		userID := int64(7500)
		chatWithGroup(t, repo, userID, "100")
		teacherChat := chatWithGroup(t, repo, 7501, "")
		teacherChat.Mode = ModeTeacher
		teacherChat.Teacher = "Иванов И.И."
		if err := repo.Save(teacherChat); err != nil {
			t.Fatal(err)
		}

		var archiveRepo *archive.Repository
		if withArchive {
			archiveRepo = setupArchive(t)
			b.archive = archiveRepo
		}
		return b, caller, userID, archiveRepo
	}

	t.Run("the archive answered nothing", func(t *testing.T) {
		b, caller, userID, _ := newBot(t, true)

		timetableCallback(t, b, userID, "g", "100", week.Value())
		if got := caller.last(); strings.Contains(got, "КэшТекущейНедели") {
			t.Fatalf("an empty archive week must not be filled from the cache, got %q", got)
		}
	})

	t.Run("the archive query failed", func(t *testing.T) {
		b, caller, userID, archiveRepo := newBot(t, true)
		if _, err := archiveRepo.DB().Exec(`DROP TABLE timetable_archive`); err != nil {
			t.Fatal(err)
		}

		timetableCallback(t, b, userID, "g", "100", week.Value())
		if got := caller.last(); strings.Contains(got, "КэшТекущейНедели") {
			t.Fatalf("a failing archive must not fall back to the cache, got %q", got)
		}

		caller.reset()
		timetableCallback(t, b, 7501, "t", "Иванов И.И.", week.Value())
		if got := caller.last(); strings.Contains(got, "КэшТекущейНедели") {
			t.Fatalf("a failing archive must not fall back to the cache for a teacher, got %q", got)
		}
	})

	t.Run("there is no archive at all", func(t *testing.T) {
		b, caller, userID, _ := newBot(t, false)

		timetableCallback(t, b, userID, "g", "100", week.Value())
		if got := caller.last(); !strings.Contains(got, "КэшТекущейНедели") {
			t.Fatalf("without an archive the cached days must be used, got %q", got)
		}
	})
}

func TestArchiveFailureKeepsTheArchiveCommandsEmptyAnswer(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	b.SetNow(func() time.Time { return when })
	chatWithGroup(t, repo, 7510, "100")

	archiveRepo := setupArchive(t)
	b.archive = archiveRepo
	if _, err := archiveRepo.DB().Exec(`DROP TABLE timetable_archive`); err != nil {
		t.Fatal(err)
	}

	sendText(t, b, 7510, "/archive week 3")
	if got := caller.last(); got != "Нет данных за указанную неделю" {
		t.Fatalf("a failing archive must keep the empty answer, got %q", got)
	}
}

func TestImageCommandRendersTheCachedWeek(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(when)

	t.Chdir(t.TempDir())

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	b.SetNow(func() time.Time { return when })
	seedGroupCache(t, b, "100", lessonOn(weekDayDate(week, 3), "КэшТекущейНедели"))
	seedTeacherCache(t, b, "Иванов И.И.", lessonOn(weekDayDate(week, 3), "КэшТекущейНедели"))

	chatWithGroup(t, repo, 7601, "100")
	sendText(t, b, 7601, "/image")
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("the student image must be rendered, got %q", caller.last())
	}

	teacherChat := chatWithGroup(t, repo, 7602, "")
	teacherChat.Mode = ModeTeacher
	teacherChat.Teacher = "Иванов И.И."
	if err := repo.Save(teacherChat); err != nil {
		t.Fatal(err)
	}
	caller.reset()
	sendText(t, b, 7602, "/image")
	if !caller.calledMethod("sendPhoto") {
		t.Fatalf("the teacher image must be rendered, got %q", caller.last())
	}

	guest := chatWithGroup(t, repo, 7603, "100")
	guest.Mode = ""
	if err := repo.Save(guest); err != nil {
		t.Fatal(err)
	}
	caller.reset()
	sendText(t, b, 7603, "/image")
	if caller.calledMethod("sendPhoto") {
		t.Fatal("an unconfigured chat must not get an image")
	}
	if got := caller.last(); got != b.loc("setup_needed") {
		t.Fatalf("unconfigured chat answer = %q", got)
	}

	student := chatWithGroup(t, repo, 7604, "")
	student.Mode = ModeStudent
	if err := repo.Save(student); err != nil {
		t.Fatal(err)
	}
	caller.reset()
	sendText(t, b, 7604, "/image")
	if got := caller.last(); got != b.loc("need_group") {
		t.Fatalf("a student without a group must be told to pick one, got %q", got)
	}

	chatWithGroup(t, repo, 7605, "999")
	caller.reset()
	sendText(t, b, 7605, "/image")
	if got := caller.last(); got != b.loc("group_not_exists") {
		t.Fatalf("an unknown group must be reported, got %q", got)
	}
}

func TestTimetableCallbackNamesTheMissingTarget(t *testing.T) {
	when := time.Date(2026, time.September, 16, 10, 0, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(when)

	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller)
	b.SetNow(func() time.Time { return when })

	seedGroupCache(t, b, "100", lessonOn(weekDayDate(week, 0), "Физика"))
	seedTeacherCache(t, b, "Иванов И.И.", lessonOn(weekDayDate(week, 0), "Физика"))

	chatWithGroup(t, repo, 7701, "100")
	caller.reset()
	timetableCallback(t, b, 7701, "t", "Петров П.П.", week.Value())
	if got := caller.last(); got != b.loc("teacher_not_exists") {
		t.Fatalf("a missing teacher must be named as such, got %q", got)
	}

	chatWithGroup(t, repo, 7702, "100")
	caller.reset()
	timetableCallback(t, b, 7702, "g", "999", week.Value())
	if got := caller.last(); got != b.loc("group_not_exists") {
		t.Fatalf("a missing group must be named as such, got %q", got)
	}
}
