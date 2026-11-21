package image

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func weekdayTexts() Texts {
	return Texts{Weekdays: [7]string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}}
}

func TestRenderTeacherDaysWritesAnImage(t *testing.T) {
	dir := t.TempDir()
	dayMaps := []map[string]any{{
		"day": "07.09.2026",
		"lessons": []any{
			map[string]any{"lesson": "Математика", "type": "лек", "cabinet": "101", "group": "100"},
		},
	}}

	path, err := RenderTeacherDays("Иванов И.И.", dayMaps, dir, weekdayTexts(), "02.01.2006")
	if err != nil {
		t.Fatalf("RenderTeacherDays: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("the teacher image is missing or empty: %v", err)
	}
	if filepath.Dir(path) != dir || !bytes.HasSuffix([]byte(path), []byte(".png")) {
		t.Fatalf("image path = %s, want a png inside %s", path, dir)
	}
}

func TestRenderTeacherDaysRefusesAnEmptyWeek(t *testing.T) {
	if _, err := RenderTeacherDays("Иванов И.И.", nil, t.TempDir(), weekdayTexts(), ""); err == nil {
		t.Fatal("an empty week must be an error")
	}
}

func TestFormatTeacherLessonFollowsTheColumns(t *testing.T) {
	lesson := map[string]any{"lesson": "Физика", "type": "пр", "cabinet": "3-209", "group": "106"}
	got := FormatTeacherLesson(lesson, 3)
	want := []string{"3", "Физика", "пр", "3-209", "106"}
	if len(got) != len(want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("columns = %v, want %v", got, want)
		}
	}

	sparse := FormatTeacherLesson(map[string]any{"lesson": "Химия"}, 1)
	if sparse[3] != "-" || sparse[4] != "-" {
		t.Fatalf("missing cabinet/group = %v, want dashes", sparse)
	}
	if FormatTeacherLesson(nil, 1) != nil {
		t.Fatal("a nil lesson has no columns")
	}
	if FormatTeacherLesson("Физика", 1) != nil {
		t.Fatal("a lesson that is not a map has no columns")
	}
}

func TestSubgroupPrefixOnlyForPositiveNumbers(t *testing.T) {
	tests := []struct {
		lesson map[string]any
		want   string
	}{
		{map[string]any{"subgroup": 2.0}, "2. "},
		{map[string]any{"subgroup": 0.0}, ""},
		{map[string]any{"subgroup": "2"}, ""},
		{map[string]any{}, ""},
	}
	for _, tc := range tests {
		if got := getSubgroupPrefix(tc.lesson); got != tc.want {
			t.Errorf("getSubgroupPrefix(%v) = %q, want %q", tc.lesson, got, tc.want)
		}
	}
}

func TestWeekdayNameReadsTheDate(t *testing.T) {
	texts := weekdayTexts()
	if got := weekdayName("07.09.2026", texts); got != texts.Weekdays[1] {
		t.Fatalf("weekdayName(07.09.2026) = %q, want %q", got, texts.Weekdays[1])
	}
	if got := weekdayName("31.02.2026", texts); got != "" {
		t.Fatalf("an impossible date must have no weekday, got %q", got)
	}
}

func TestFontPathIsStable(t *testing.T) {
	first := FontPath()
	if first != FontPath() {
		t.Fatal("FontPath must not change between calls")
	}
	if first != "" {
		if _, err := os.Stat(first); err != nil {
			t.Fatalf("FontPath returned a path that does not exist: %v", err)
		}
	}
}
