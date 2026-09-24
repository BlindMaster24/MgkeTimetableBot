package telegram

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func runICS(t *testing.T, b *Bot, userID int64) {
	t.Helper()

	u := makeUpdate(userID, "/ics")
	u.Bot = b
	if err := (&icsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
}

func TestICSCommandExportsTheWeekInTheOldFormat(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = true

	week := b.relevantWeekIndex()
	first, _ := week.WeekRange()
	seedArchivedLessons(t, archiveRepo, "100", first, []any{
		map[string]any{"lesson": "Математика", "type": "Лек", "teacher": "Иванов И.И.", "cabinet": "101"},
		[]any{
			map[string]any{"lesson": "Английский", "subgroup": 1, "cabinet": "201"},
			map[string]any{"lesson": "Английский", "subgroup": 2, "cabinet": "202"},
		},
	})

	runICS(t, b, userID)

	document := caller.documentPayload()
	if document == "" {
		t.Fatal("no document was sent")
	}

	filename := fmt.Sprintf("schedule-group-100-week-%02d.ics", week.AcademicWeekNumber())

	for _, want := range []string{
		filename,
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//MGKE Timetable Bot//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"DTSTART:" + first.Format("20060102") + "T080000",
		"DTEND:" + first.Format("20060102") + "T094000",
		"DTSTART:" + first.Format("20060102") + "T095000",
		"DTEND:" + first.Format("20060102") + "T113000",
		"SUMMARY:Математика (Лек)",
		"DESCRIPTION:Преподаватель: Иванов И.И.\\nКабинет: 101",
		"LOCATION:101",
		"SUMMARY:Английский\\, подгр. 1",
		"SUMMARY:Английский\\, подгр. 2",
		"LOCATION:202",
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the ics document is missing %q:\n%s", want, document)
		}
	}

	if count := strings.Count(document, "BEGIN:VEVENT"); count != 3 {
		t.Errorf("expected one event per filled lesson and subgroup, got %d:\n%s", count, document)
	}
	if caller.last() != "" {
		t.Errorf("the old bot sent the file without a caption, got %q", caller.last())
	}
}

func TestICSCommandTeacherWeek(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeTeacher, "", "Иванов И.И.")
	b.cfg.Calendar.ICS.Enabled = true

	first, _ := b.relevantWeekIndex().WeekRange()
	seedArchivedDay(t, archiveRepo, "teacher", "Иванов И.И.", first, map[string]any{
		"lesson": "Физика", "group": "100", "cabinet": "303",
	})

	runICS(t, b, userID)

	document := caller.documentPayload()
	if !strings.Contains(document, "schedule-teacher-Иванов И.И.-week-") {
		t.Errorf("unexpected filename:\n%s", document)
	}
	if !strings.Contains(document, "SUMMARY:Физика") {
		t.Errorf("expected the lesson in the summary:\n%s", document)
	}
	if !strings.Contains(document, "DESCRIPTION:Группа: 100\\nКабинет: 303") {
		t.Errorf("teacher events must carry the group:\n%s", document)
	}
}

func TestICSCommandStaysSilentOnDisabledExport(t *testing.T) {
	b, caller, archiveRepo, userID := setupArchiveBot(t, ModeStudent, "100", "")

	first, _ := b.relevantWeekIndex().WeekRange()
	seedArchivedDay(t, archiveRepo, "group", "100", first, groupLesson("Математика"))

	runICS(t, b, userID)

	if caller.last() != "ICS отключен в конфиге." {
		t.Fatalf("unexpected answer %q", caller.last())
	}
	if document := caller.documentPayload(); document != "" {
		t.Errorf("no document expected, got %q", document)
	}
}

func TestICSCommandWithoutArchivedWeek(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Calendar.ICS.Enabled = true

	runICS(t, b, userID)

	if caller.last() != "Нет расписания за текущую неделю." {
		t.Fatalf("unexpected answer %q", caller.last())
	}
}

func TestICSCommandRejectsGuestMode(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeGuest, "", "")
	b.cfg.Calendar.ICS.Enabled = true

	runICS(t, b, userID)

	if caller.last() != "Режим чата не поддерживает экспорт расписания." {
		t.Fatalf("unexpected answer %q", caller.last())
	}
}
