package image

import (
	"os"
	"testing"
	"time"
)

func TestRenderer_RenderGroupImage(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)

	days := []DayData{
		{
			Date:    "01.09.2025",
			Weekday: "Понедельник",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Математика", "лекция", "101", "Иванов А.А."}},
				{Number: 2, Cells: []string{"2", "Физика", "пр-ка", "202", "Петров Б.Б."}},
				{Number: 3, Cells: []string{"3", "Информатика", "пр-ка", "303", "Сидоров В.В."}},
			},
		},
	}

	path, err := r.RenderGroupImage("63ТП", days)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty PNG file")
	}
	t.Logf("rendered %s (%d bytes)", path, info.Size())
}

func TestRenderer_RenderTeacherImage(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)

	days := []DayData{
		{
			Date:    "01.09.2025",
			Weekday: "Понедельник",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Математика", "лекция", "101", "63ТП"}},
				{Number: 2, Cells: []string{"2", "Физика", "пр-ка", "202", "64ИС"}},
			},
		},
		{
			Date:    "02.09.2025",
			Weekday: "Вторник",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Информатика", "лекция", "303", "63ТП"}},
			},
		},
	}

	path, err := r.RenderTeacherImage("Иванов А.А.", days)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty PNG file")
	}
	t.Logf("rendered %s (%d bytes)", path, info.Size())
}

func TestRenderer_EmptyDays(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)

	_, err := r.RenderGroupImage("63", nil)
	if err == nil {
		t.Error("expected error for empty days")
	}
}

func TestRenderer_Cleanup(t *testing.T) {
	dir := t.TempDir()
	r := NewRenderer(dir)

	days := []DayData{
		{Date: "01.09.2025", Weekday: "Пн", Lessons: []LessonRow{
			{Number: 1, Cells: []string{"1", "Math", "", "", ""}},
		}},
	}

	path, err := r.RenderGroupImage("63", days)
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	r.Cleanup(time.Hour)
	entries, _ = os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("file should not be cleaned up yet, got %d files", len(entries))
	}

	t.Log("cleanup test: file has age 0, may or may not be cleaned depending on timing")

	_ = path
}

func TestFormatGroupLesson(t *testing.T) {
	lesson := map[string]any{
		"lesson":  "Математика",
		"type":    "лекция",
		"cabinet": "101",
		"teacher": "Иванов А.А.",
	}
	cells := FormatGroupLesson(lesson, 3)
	if len(cells) != 5 {
		t.Fatalf("expected 5 cells, got %d", len(cells))
	}
	if cells[0] != "3" {
		t.Errorf("expected number 3, got %q", cells[0])
	}
	if cells[1] != "Математика" {
		t.Errorf("expected 'Математика', got %q", cells[1])
	}
}

func TestFormatGroupLesson_Nil(t *testing.T) {
	cells := FormatGroupLesson(nil, 1)
	if cells != nil {
		t.Errorf("expected nil, got %v", cells)
	}
}

func TestFormatTeacherLesson(t *testing.T) {
	lesson := map[string]any{
		"lesson":  "Физика",
		"type":    "пр-ка",
		"cabinet": "202",
		"group":   "63ТП",
	}
	cells := FormatTeacherLesson(lesson, 1)
	if len(cells) != 5 {
		t.Fatalf("expected 5 cells, got %d", len(cells))
	}
	if cells[4] != "63ТП" {
		t.Errorf("expected group 63ТП, got %q", cells[4])
	}
}

func TestWeekdayName(t *testing.T) {
	tests := []struct {
		date     string
		expected string
	}{
		{"01.09.2025", "Понедельник"},
		{"02.09.2025", "Вторник"},
		{"03.09.2025", "Среда"},
		{"04.09.2025", "Четверг"},
		{"05.09.2025", "Пятница"},
		{"06.09.2025", "Суббота"},
		{"07.09.2025", "Воскресенье"},
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			result := weekdayName(tt.date)
			if result != tt.expected {
				t.Errorf("weekdayName(%s) = %q, want %q", tt.date, result, tt.expected)
			}
		})
	}
}
