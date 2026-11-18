package cache

import (
	"encoding/json"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/timetable"
)

func TestDatedDaysKeepsOnlyParsableDays(t *testing.T) {
	if days := datedDays(nil); days != nil {
		t.Fatalf("nil days must stay nil, got %v", days)
	}

	// A single day is the boundary the mutation testing cares about: the
	// function must still normalise it.
	one := datedDays([]timetable.Day{{Date: " 07.09.2026 "}})
	if len(one) != 1 || one[0].Date != "07.09.2026" {
		t.Fatalf("a single dated day must be normalised, got %v", one)
	}

	both := datedDays([]timetable.Day{
		{Date: "понедельник"},
		{Date: "07.09.2026"},
	})
	if len(both) != 1 || both[0].Date != "07.09.2026" {
		t.Fatalf("the undated day must be dropped, got %v", both)
	}
}

func TestGroupTimetableKeepsTheStoredGroupName(t *testing.T) {
	raw := []byte(`{"100": {"group": "101", "days": [{"day": "07.09.2026", "lessons": []}]}}`)

	var groups GroupTimetable
	if err := json.Unmarshal(raw, &groups); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if groups["100"].Group != "101" {
		t.Fatalf("the stored group name must survive, got %q", groups["100"].Group)
	}

	blank := GroupTimetable{}
	if err := json.Unmarshal([]byte(`{"100": {"days": [{"day": "07.09.2026", "lessons": []}]}}`), &blank); err != nil {
		t.Fatalf("unmarshal without a name: %v", err)
	}
	if blank["100"].Group != "100" {
		t.Fatalf("a schedule without a name must take the map key, got %q", blank["100"].Group)
	}
}

func TestTeacherTimetableKeepsTheStoredTeacherName(t *testing.T) {
	raw := []byte(`{"Иванов И.И.": {"teacher": "Петров П.П.", "days": [{"day": "07.09.2026", "lessons": []}]}}`)

	var teachers TeacherTimetable
	if err := json.Unmarshal(raw, &teachers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if teachers["Иванов И.И."].Teacher != "Петров П.П." {
		t.Fatalf("the stored teacher name must survive, got %q", teachers["Иванов И.И."].Teacher)
	}

	blank := TeacherTimetable{}
	if err := json.Unmarshal([]byte(`{"Иванов И.И.": {"days": [{"day": "07.09.2026", "lessons": []}]}}`), &blank); err != nil {
		t.Fatalf("unmarshal without a name: %v", err)
	}
	if blank["Иванов И.И."].Teacher != "Иванов И.И." {
		t.Fatalf("a schedule without a name must take the map key, got %q", blank["Иванов И.И."].Teacher)
	}
}

func TestIMPLaUSIBLEGroupPayloadFallsBackToTheLegacyPath(t *testing.T) {
	// The typed path validates every day; a broken lesson forces the legacy
	// converter, which still has to answer a usable schedule.
	raw := []byte(`{"100": {"group": "100", "days": [{"day": "07.09.2026", "lessons": [{"num": "x"}]}]}}`)

	var groups GroupTimetable
	if err := json.Unmarshal(raw, &groups); err != nil {
		t.Fatalf("the legacy path must accept the payload: %v", err)
	}
	if groups["100"].Group != "100" {
		t.Fatalf("group = %q, want 100", groups["100"].Group)
	}
}

func TestOnlyDatedEntriesNormalisesAndKeepsUntouchedEntries(t *testing.T) {
	entries := map[string]any{
		"plain":  "value",
		"nodays": map[string]any{"group": "x"},
		"dated": map[string]any{"days": []any{
			map[string]any{"day": " 07.09.2026 ", "lessons": []any{}},
			"not a map",
			map[string]any{"day": "понедельник"},
		}},
	}

	sanitized := onlyDatedEntries(entries)
	if sanitized["plain"] != "value" {
		t.Fatalf("a non-entry value must pass through, got %v", sanitized["plain"])
	}
	if got := sanitized["nodays"].(map[string]any)["group"]; got != "x" {
		t.Fatalf("an entry without days must pass through, got %v", got)
	}

	days := sanitized["dated"].(map[string]any)["days"].([]any)
	if len(days) != 1 {
		t.Fatalf("only the dated day must survive, got %v", days)
	}
	if got := days[0].(map[string]any)["day"]; got != "07.09.2026" {
		t.Fatalf("day = %v, want the trimmed date", got)
	}
}
