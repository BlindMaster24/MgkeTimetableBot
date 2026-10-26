package image

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"time"

	"golang.org/x/image/font/gofont/goregular"
)

//go:embed golden/group.png
var goldenGroupPNG []byte

//go:embed golden/teacher.png
var goldenTeacherPNG []byte

const goldenTimestamp = "2026-10-21T12:34:56"

func GoldenImages() map[string][]byte {
	return map[string][]byte{
		"group":   goldenGroupPNG,
		"teacher": goldenTeacherPNG,
	}
}

func GoldenGroupDays() []DayData {
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

func GoldenTeacherDays() []DayData {
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

func RenderGoldenSamples() (map[string][]byte, error) {
	fontFile, err := os.CreateTemp("", "mgke-golden-*.ttf")
	if err != nil {
		return nil, fmt.Errorf("create the golden font file: %w", err)
	}
	defer os.Remove(fontFile.Name())

	if _, err := fontFile.Write(goregular.TTF); err != nil {
		fontFile.Close()
		return nil, fmt.Errorf("write the golden font file: %w", err)
	}
	if err := fontFile.Close(); err != nil {
		return nil, fmt.Errorf("close the golden font file: %w", err)
	}

	at, err := time.Parse("2006-01-02T15:04:05", goldenTimestamp)
	if err != nil {
		return nil, fmt.Errorf("golden timestamp: %w", err)
	}

	dir, err := os.MkdirTemp("", "mgke-golden-")
	if err != nil {
		return nil, fmt.Errorf("create the golden render directory: %w", err)
	}
	defer os.RemoveAll(dir)

	renderer := NewRenderer(dir)
	renderer.SetFontPath(fontFile.Name())
	renderer.SetNow(func() time.Time { return at })

	groupPath, err := renderer.RenderGroupImage("63ТП", GoldenGroupDays())
	if err != nil {
		return nil, fmt.Errorf("render the group sample: %w", err)
	}
	teacherPath, err := renderer.RenderTeacherImage("Иванов А.А.", GoldenTeacherDays())
	if err != nil {
		return nil, fmt.Errorf("render the teacher sample: %w", err)
	}

	group, err := os.ReadFile(groupPath)
	if err != nil {
		return nil, fmt.Errorf("read the group sample: %w", err)
	}
	teacher, err := os.ReadFile(teacherPath)
	if err != nil {
		return nil, fmt.Errorf("read the teacher sample: %w", err)
	}

	return map[string][]byte{"group": group, "teacher": teacher}, nil
}

type GoldenLayout struct {
	Name       string
	Width      int
	Height     int
	WantWidth  int
	WantHeight int
	Differing  int
}

func CheckGoldenLayout() ([]GoldenLayout, error) {
	rendered, err := RenderGoldenSamples()
	if err != nil {
		return nil, err
	}

	golden := GoldenImages()
	names := make([]string, 0, len(golden))
	for name := range golden {
		names = append(names, name)
	}
	sort.Strings(names)

	checks := make([]GoldenLayout, 0, len(names))
	for _, name := range names {
		got := rendered[name]
		if got == nil {
			return nil, fmt.Errorf("the renderer produced no %s sample", name)
		}

		differing, err := ComparePNG(golden[name], got, 8)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}

		check := GoldenLayout{Name: name, Differing: differing}
		wantImage, err := png.Decode(bytes.NewReader(golden[name]))
		if err != nil {
			return nil, fmt.Errorf("%s: decode the golden image: %w", name, err)
		}
		check.WantWidth, check.WantHeight = wantImage.Bounds().Dx(), wantImage.Bounds().Dy()

		gotImage, err := png.Decode(bytes.NewReader(got))
		if err != nil {
			return nil, fmt.Errorf("%s: decode the rendered image: %w", name, err)
		}
		check.Width, check.Height = gotImage.Bounds().Dx(), gotImage.Bounds().Dy()

		checks = append(checks, check)
	}
	return checks, nil
}

func ComparePNG(want, got []byte, tolerance int) (int, error) {
	wantImage, err := png.Decode(bytes.NewReader(want))
	if err != nil {
		return 0, fmt.Errorf("decode the golden image: %w", err)
	}
	gotImage, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		return 0, fmt.Errorf("decode the rendered image: %w", err)
	}
	if wantImage.Bounds() != gotImage.Bounds() {
		return -1, nil
	}
	return differingPixels(wantImage, gotImage, tolerance), nil
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
