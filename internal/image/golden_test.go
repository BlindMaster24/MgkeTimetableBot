package image

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"
)

var updateGoldenImages = flag.Bool("update", false, "rewrite the golden timetable images")

const goldenTime = "2026-10-21T12:34:56"

func goldenRenderer(t *testing.T) *Renderer {
	t.Helper()

	fontPath := filepath.Join(t.TempDir(), "goregular.ttf")
	if err := os.WriteFile(fontPath, goregular.TTF, 0o644); err != nil {
		t.Fatalf("write the golden font: %v", err)
	}

	r := NewRenderer(t.TempDir())
	r.SetFontPath(fontPath)
	r.SetNow(func() time.Time {
		at, err := time.Parse("2006-01-02T15:04:05", goldenTime)
		if err != nil {
			t.Fatalf("golden time: %v", err)
		}
		return at
	})
	return r
}

func goldenGroupDays() []DayData {
	return []DayData{
		{
			Date:    "21.10.2026",
			Weekday: "Среда (сегодня)",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Математика", "Лек", "101", "Иванов А.А."}},
				{Number: 2, Cells: []string{"2", "1. Физика\n2. Информатика", "Пр\nЛР", "202\n303", "Петров Б.Б.\nСидоров В.В."}},
				{Number: 3, Cells: []string{"3", "История", "Лек", "105", "Козлов Г.Г."}},
				{Number: 4, Cells: []string{"4", "Английский язык", "Пр", "210", "Смирнова Е.Е."}},
			},
		},
		{
			Date:    "22.10.2026",
			Weekday: "Четверг",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Химия", "ЛР", "310", "Орлов Д.Д."}},
				{Number: 2, Cells: []string{"2", "Биология", "Лек", "312", "Зайцева М.М."}},
			},
		},
		{
			Date:    "23.10.2026",
			Weekday: "Пятница",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Литература", "Лек", "208", "Волков И.И."}},
				{Number: 2, Cells: []string{"2", "География", "Лек", "112", "Морозова О.О."}},
				{Number: 3, Cells: []string{"3", "Информатика", "ЛР", "303", "Сидоров В.В."}},
			},
		},
	}
}

func goldenTeacherDays() []DayData {
	return []DayData{
		{
			Date:    "21.10.2026",
			Weekday: "Среда",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Математика", "Лек", "101", "63ТП"}},
				{Number: 2, Cells: []string{"2", "Физика", "Пр", "202", "64ИС"}},
			},
		},
		{
			Date:    "22.10.2026",
			Weekday: "Четверг",
			Lessons: []LessonRow{
				{Number: 1, Cells: []string{"1", "Информатика", "ЛР", "303", "63ТП"}},
			},
		},
	}
}

func TestGroupTimetableMatchesTheGoldenImage(t *testing.T) {
	r := goldenRenderer(t)
	path, err := r.RenderGroupImage("63ТП", goldenGroupDays())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	compareWithGolden(t, path, filepath.Join("testdata", "group.golden.png"))
}

func TestTeacherTimetableMatchesTheGoldenImage(t *testing.T) {
	r := goldenRenderer(t)
	path, err := r.RenderTeacherImage("Иванов А.А.", goldenTeacherDays())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	compareWithGolden(t, path, filepath.Join("testdata", "teacher.golden.png"))
}

func compareWithGolden(t *testing.T, renderedPath, goldenPath string) {
	t.Helper()

	got, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read the rendered image: %v", err)
	}

	if *updateGoldenImages {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", goldenPath, err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%s is missing: run go test ./internal/image -update", goldenPath)
	}

	gotImage, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode the rendered image: %v", err)
	}
	wantImage, err := png.Decode(bytes.NewReader(want))
	if err != nil {
		t.Fatalf("decode %s: %v", goldenPath, err)
	}

	if gotImage.Bounds() != wantImage.Bounds() {
		received := filepath.Join(filepath.Dir(goldenPath), filepath.Base(goldenPath)+".got.png")
		os.WriteFile(received, got, 0o644)
		t.Fatalf("size %v, want %v (received image kept in %s)", gotImage.Bounds(), wantImage.Bounds(), received)
	}

	if differing := differingPixels(wantImage, gotImage, 8); differing > 0 {
		received := filepath.Join(filepath.Dir(goldenPath), filepath.Base(goldenPath)+".got.png")
		os.WriteFile(received, got, 0o644)
		t.Fatalf("%d pixels differ from the golden layout (received image kept in %s): review the render and run go test ./internal/image -update", differing, received)
	}
}

func differingPixels(want, got image.Image, tolerance int) int {
	bounds := want.Bounds()
	differing := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			wr, wg, wb, _ := want.At(x, y).RGBA()
			gr, gg, gb, _ := got.At(x, y).RGBA()
			if channelDiff(int(wr>>8), int(gr>>8)) > tolerance ||
				channelDiff(int(wg>>8), int(gg>>8)) > tolerance ||
				channelDiff(int(wb>>8), int(gb>>8)) > tolerance {
				differing++
			}
		}
	}
	return differing
}

func channelDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
