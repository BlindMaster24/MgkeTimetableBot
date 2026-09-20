package telegram

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

func setupArchive(t *testing.T) *archive.Repository {
	t.Helper()

	repo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	if _, err := repo.DB().Exec(`CREATE TABLE IF NOT EXISTS timetable_archive (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		day INTEGER NOT NULL,
		"group" TEXT,
		teacher TEXT,
		data TEXT NOT NULL,
		UNIQUE(day, "group"),
		UNIQUE(day, teacher)
	)`); err != nil {
		t.Fatalf("create archive schema: %v", err)
	}
	return repo
}

func stringPtr(value string) *string { return &value }
func subPtr(value int) *int          { return &value }

func TestGroupDayLessonsBuildsCalendarEntries(t *testing.T) {
	day := model.GroupDay{
		Day: "14.09.2026",
		Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика", Type: stringPtr("Лек"), Cabinet: stringPtr("101")},
			[]*model.GroupLessonExplain{
				{Subgroup: subPtr(1), Lesson: "Физика", Cabinet: stringPtr("202")},
				{Subgroup: subPtr(2), Lesson: "Физика", Cabinet: stringPtr("203")},
			},
			nil,
		},
	}

	lessons := groupDayLessons(day)
	if len(lessons) != 2 {
		t.Fatalf("expected two calendar entries, got %d", len(lessons))
	}
	if lessons[0].Index != 0 || lessons[0].Title != "Математика" {
		t.Errorf("first lesson = %+v", lessons[0])
	}
	if lessons[1].Index != 1 || lessons[1].Title != "1,2 - Физика" {
		t.Errorf("second lesson = %+v", lessons[1])
	}
	if lessons[1].Location != "202 | 203" {
		t.Errorf("location = %q", lessons[1].Location)
	}
}

func TestTeacherDayLessonsBuildsCalendarEntries(t *testing.T) {
	day := model.TeacherDay{
		Day: "14.09.2026",
		Lessons: []model.TeacherLesson{
			{Group: "100", Lesson: "Физика", Cabinet: stringPtr("202")},
			nil,
		},
	}

	lessons := teacherDayLessons(day)
	if len(lessons) != 1 {
		t.Fatalf("expected one calendar entry, got %d", len(lessons))
	}
	if lessons[0].Title != "100-Физика" || lessons[0].Location != "202" {
		t.Errorf("lesson = %+v", lessons[0])
	}
}

func TestGoogleResyncGroupCalendarFromArchive(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	archiveRepo := setupArchive(t)
	b.archive = archiveRepo

	if err := archiveRepo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: &model.GroupDay{
			Day: "14.09.2026",
			Lessons: []model.GroupLesson{
				&model.GroupLessonExplain{Lesson: "Математика", Cabinet: stringPtr("101")},
			},
		}},
		{Type: "group", Value: "100", Day: &model.GroupDay{
			Day: "15.09.2026",
			Lessons: []model.GroupLesson{
				&model.GroupLessonExplain{Lesson: "Информатика", Cabinet: stringPtr("303")},
			},
		}},
	}); err != nil {
		t.Fatalf("append archive days: %v", err)
	}

	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-sync"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.resyncGoogleCalendar(context.Background(), calendar); err != nil {
		t.Fatalf("resync: %v", err)
	}

	if len(service.synced) != 2 {
		t.Fatalf("expected two synced days, got %v", service.synced)
	}
	if service.synced[0] != "14.09.2026" || service.synced[1] != "15.09.2026" {
		t.Errorf("synced days = %v", service.synced)
	}
	if len(service.syncedLessons) != 2 || service.syncedLessons[0].Title != "Математика" {
		t.Errorf("synced lessons = %+v", service.syncedLessons)
	}

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		t.Fatalf("bounds: %v", err)
	}
	saved, err := repo.GoogleCalendarByLocalID(calendar.ID)
	if err != nil || saved == nil {
		t.Fatalf("reload calendar: %v", err)
	}
	if saved.LastManualSyncedDay != bounds.Max {
		t.Errorf("last synced day = %d, want %d", saved.LastManualSyncedDay, bounds.Max)
	}
}

func TestGoogleResyncTeacherCalendarFromArchive(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	archiveRepo := setupArchive(t)
	b.archive = archiveRepo

	if err := archiveRepo.AppendDays([]archive.AppendDay{
		{Type: "teacher", Value: "Иванов И.И.", Day: &model.TeacherDay{
			Day: "14.09.2026",
			Lessons: []model.TeacherLesson{
				{Group: "100", Lesson: "Физика", Cabinet: stringPtr("202")},
			},
		}},
	}); err != nil {
		t.Fatalf("append archive days: %v", err)
	}

	calendar := &GoogleCalendar{Type: "teacher", Value: "Иванов И.И.", CalendarID: "cal-teacher"}
	if err := repo.SaveGoogleCalendar(calendar); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.resyncGoogleCalendar(context.Background(), calendar); err != nil {
		t.Fatalf("resync: %v", err)
	}

	if len(service.synced) != 1 || service.synced[0] != "14.09.2026" {
		t.Fatalf("synced days = %v", service.synced)
	}
	if len(service.syncedLessons) != 1 || service.syncedLessons[0].Title != "100-Физика" {
		t.Errorf("synced lessons = %+v", service.syncedLessons)
	}
}

func TestSyncGoogleCalendarDaysSkipsOutOfBoundsAndKeepsGoing(t *testing.T) {
	b, _, _, service := setupGoogleBot(t)
	archiveRepo := seedTwoGroupDays(t, b)

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		t.Fatalf("bounds: %v", err)
	}

	service.syncErrs = map[string]error{"15.09.2026": errors.New("calendar backend down")}

	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-partial"}
	synced, err := b.syncGoogleCalendarDays(context.Background(), archiveRepo, calendar,
		[]string{"13.09.2026", "14.09.2026", "15.09.2026"}, b.activeCallsSchedule(), bounds)

	if err == nil || !strings.Contains(err.Error(), "15.09.2026") {
		t.Fatalf("the failing day must be reported, got %v", err)
	}
	if synced != 1 {
		t.Fatalf("reported %d synced days, want 1", synced)
	}
	if len(service.synced) != 2 || service.synced[0] != "14.09.2026" || service.synced[1] != "15.09.2026" {
		t.Fatalf("synced days = %v", service.synced)
	}
	if len(service.syncedLessons) != 2 || service.syncedLessons[0].Title != "Математика" || service.syncedLessons[1].Title != "Информатика" {
		t.Fatalf("the lessons must come from the archive, got %+v", service.syncedLessons)
	}
}

func TestGoogleResyncSkippedWithoutServiceAccount(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	service.syncDisabled = true
	b.archive = setupArchive(t)

	calendar := &GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-nosync"}
	repo.SaveGoogleCalendar(calendar)

	if _, err := b.resyncGoogleCalendar(context.Background(), calendar); err != nil {
		t.Fatalf("resync: %v", err)
	}
	if len(service.synced) != 0 {
		t.Errorf("expected no sync without a service account, got %v", service.synced)
	}
}

func TestSyncGoogleCalendarsSyncsEveryStoredCalendar(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	archiveRepo := setupArchive(t)
	b.archive = archiveRepo

	if err := archiveRepo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: &model.GroupDay{Day: "14.09.2026", Lessons: []model.GroupLesson{
			&model.GroupLessonExplain{Lesson: "Математика", Cabinet: stringPtr("101")},
		}}},
		{Type: "teacher", Value: "Иванов И.И.", Day: &model.TeacherDay{Day: "14.09.2026", Lessons: []model.TeacherLesson{
			{Group: "100", Lesson: "Физика", Cabinet: stringPtr("202")},
		}}},
	}); err != nil {
		t.Fatalf("append archive days: %v", err)
	}

	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-group"}); err != nil {
		t.Fatalf("save group calendar: %v", err)
	}
	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "teacher", Value: "Иванов И.И.", CalendarID: "cal-teacher"}); err != nil {
		t.Fatalf("save teacher calendar: %v", err)
	}

	if _, err := b.SyncGoogleCalendars(context.Background()); err != nil {
		t.Fatalf("sync calendars: %v", err)
	}

	if len(service.synced) != 2 {
		t.Fatalf("expected both calendars to be synced, got %v", service.synced)
	}
	titles := map[string]bool{}
	for _, lesson := range service.syncedLessons {
		titles[lesson.Title] = true
	}
	if !titles["Математика"] || !titles["100-Физика"] {
		t.Errorf("synced lessons = %+v", service.syncedLessons)
	}
}

func TestSyncGoogleCalendarsSkipsReconciledCalendar(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	archiveRepo := seedTwoGroupDays(t, b)

	bounds, err := archiveRepo.DayIndexBounds()
	if err != nil {
		t.Fatalf("bounds: %v", err)
	}

	if err := repo.SaveGoogleCalendar(&GoogleCalendar{
		Type: "group", Value: "100", CalendarID: "cal-reconciled", LastManualSyncedDay: bounds.Max,
	}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.SyncGoogleCalendars(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(service.synced) != 0 {
		t.Errorf("reconciled calendar should not be resynced, got %v", service.synced)
	}
}

func TestSyncGoogleCalendarsCatchesUpOnlyNewDays(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	seedTwoGroupDays(t, b)

	first := archive.DateToDayIndex("14.09.2026")
	if err := repo.SaveGoogleCalendar(&GoogleCalendar{
		Type: "group", Value: "100", CalendarID: "cal-catchup", LastManualSyncedDay: first,
	}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.SyncGoogleCalendars(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(service.synced) != 1 || service.synced[0] != "15.09.2026" {
		t.Fatalf("expected only the new day to be synced, got %v", service.synced)
	}
}

func TestSyncGoogleCalendarChangesSyncsOnlyChangedDay(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	seedTwoGroupDays(t, b)

	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-changes"}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	synced, err := b.SyncGoogleCalendarChanges(context.Background(), []GoogleDayChange{
		{Type: "group", Value: "100", Date: "15.09.2026"},
	})
	if err != nil {
		t.Fatalf("sync changes: %v", err)
	}
	if synced != 1 {
		t.Errorf("reported %d synced days, want 1", synced)
	}

	if len(service.synced) != 1 || service.synced[0] != "15.09.2026" {
		t.Fatalf("synced days = %v, want only 15.09.2026", service.synced)
	}
	if service.syncedCalendar[0] != "cal-changes" {
		t.Errorf("synced calendar = %q", service.syncedCalendar[0])
	}
	if len(service.syncedLessons) != 1 || service.syncedLessons[0].Title != "Информатика" {
		t.Errorf("synced lessons = %+v", service.syncedLessons)
	}
}

func TestSyncGoogleCalendarChangesClearsEmptiedDay(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	archiveRepo := setupArchive(t)
	b.archive = archiveRepo

	if err := archiveRepo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: &model.GroupDay{Day: "16.09.2026", Lessons: nil}},
	}); err != nil {
		t.Fatalf("append archive days: %v", err)
	}

	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-empty"}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.SyncGoogleCalendarChanges(context.Background(), []GoogleDayChange{
		{Type: "group", Value: "100", Date: "16.09.2026"},
	}); err != nil {
		t.Fatalf("sync changes: %v", err)
	}

	if len(service.synced) != 1 || service.synced[0] != "16.09.2026" {
		t.Fatalf("expected the emptied day to be synced, got %v", service.synced)
	}
	if len(service.syncedLessons) != 0 {
		t.Errorf("expected no lessons, got %+v", service.syncedLessons)
	}
}

func TestSyncGoogleCalendarChangesIgnoresUnrelatedCalendars(t *testing.T) {
	b, repo, _, service := setupGoogleBot(t)
	seedTwoGroupDays(t, b)

	if err := repo.SaveGoogleCalendar(&GoogleCalendar{Type: "group", Value: "100", CalendarID: "cal-100"}); err != nil {
		t.Fatalf("save calendar: %v", err)
	}

	if _, err := b.SyncGoogleCalendarChanges(context.Background(), []GoogleDayChange{
		{Type: "group", Value: "200", Date: "15.09.2026"},
		{Type: "teacher", Value: "Иванов И.И.", Date: "15.09.2026"},
	}); err != nil {
		t.Fatalf("sync changes: %v", err)
	}
	if len(service.synced) != 0 {
		t.Errorf("unrelated changes should not sync anything, got %v", service.synced)
	}
}

func TestGroupGoogleChangesDedupes(t *testing.T) {
	grouped := groupGoogleChanges([]GoogleDayChange{
		{Type: "group", Value: "100", Date: "15.09.2026"},
		{Type: "group", Value: "100", Date: "15.09.2026"},
		{Type: "group", Value: "100", Date: "16.09.2026"},
		{Type: "teacher", Value: "Иванов И.И.", Date: "15.09.2026"},
		{Type: "group", Value: "", Date: "17.09.2026"},
	})

	if got := len(grouped[googleTarget{Type: "group", Value: "100"}]); got != 2 {
		t.Errorf("group dates = %d, want 2", got)
	}
	if got := len(grouped[googleTarget{Type: "teacher", Value: "Иванов И.И."}]); got != 1 {
		t.Errorf("teacher dates = %d, want 1", got)
	}
}

func seedTwoGroupDays(t *testing.T, b *Bot) *archive.Repository {
	t.Helper()

	repo := setupArchive(t)
	b.archive = repo

	if err := repo.AppendDays([]archive.AppendDay{
		{Type: "group", Value: "100", Day: &model.GroupDay{
			Day: "14.09.2026",
			Lessons: []model.GroupLesson{
				&model.GroupLessonExplain{Lesson: "Математика", Cabinet: stringPtr("101")},
			},
		}},
		{Type: "group", Value: "100", Day: &model.GroupDay{
			Day: "15.09.2026",
			Lessons: []model.GroupLesson{
				&model.GroupLessonExplain{Lesson: "Информатика", Cabinet: stringPtr("303")},
			},
		}},
	}); err != nil {
		t.Fatalf("append archive days: %v", err)
	}
	return repo
}

func TestActiveCallsSchedulePrefersCache(t *testing.T) {
	b, _, _, _ := setupGoogleBot(t)

	active := b.activeCallsSchedule()
	if len(active.Weekdays) == 0 || len(active.Saturday) == 0 {
		t.Fatalf("calls schedule is empty: %+v", active)
	}
	if active.Weekdays[0][0][0] != "08:00" {
		t.Errorf("first call slot = %v, want the cache schedule", active.Weekdays[0])
	}
}
