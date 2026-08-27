package image

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/fogleman/gg"
)

type Renderer struct {
	outputDir string
}

func NewRenderer(outputDir string) *Renderer {
	os.MkdirAll(outputDir, 0755)
	return &Renderer{outputDir: outputDir}
}

type DayTable struct {
	Date    string
	Weekday string
	Lessons []Row
}

type Row struct {
	Index   int
	Content string
}

func BuildDayTable(day model.GroupDay) DayTable {
	return DayTable{
		Date:    day.Day,
		Weekday: weekdayName(day.Day),
		Lessons: buildLessonRows(day.Lessons),
	}
}

func BuildTeacherDayTable(day model.TeacherDay) DayTable {
	return DayTable{
		Date:    day.Day,
		Weekday: weekdayName(day.Day),
		Lessons: buildTeacherLessonRows(day.Lessons),
	}
}

func buildLessonRows(lessons []model.GroupLesson) []Row {
	var rows []Row
	for i, l := range lessons {
		idx := i + 1
		text := formatGroupLesson(l)
		if text == "" {
			text = "-"
		}
		rows = append(rows, Row{Index: idx, Content: text})
	}
	return rows
}

func buildTeacherLessonRows(lessons []model.TeacherLesson) []Row {
	var rows []Row
	for i, l := range lessons {
		idx := i + 1
		text := formatTeacherLesson(l)
		if text == "" {
			text = "-"
		}
		rows = append(rows, Row{Index: idx, Content: text})
	}
	return rows
}

func formatGroupLesson(l model.GroupLesson) string {
	if l == nil {
		return "-"
	}
	if s := model.AsSingle(l); s != nil {
		return formatSingleGroupLesson(s)
	}
	if arr := model.AsArray(l); arr != nil {
		parts := make([]string, 0, len(arr))
		for _, e := range arr {
			parts = append(parts, formatSingleGroupLesson(e))
		}
		return strings.Join(parts, " | ")
	}
	return "-"
}

func formatSingleGroupLesson(e *model.GroupLessonExplain) string {
	if e == nil {
		return "-"
	}
	var parts []string
	if e.Subgroup != nil && *e.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", *e.Subgroup))
	}
	parts = append(parts, e.Lesson)
	if e.Type != nil && *e.Type != "" {
		parts = append(parts, fmt.Sprintf("(%s)", *e.Type))
	}
	if e.Teacher != nil && *e.Teacher != "" {
		parts = append(parts, *e.Teacher)
	}
	if e.Cabinet != nil && *e.Cabinet != "" {
		parts = append(parts, *e.Cabinet)
	}
	if e.Comment != nil && *e.Comment != "" {
		parts = append(parts, fmt.Sprintf("[%s]", *e.Comment))
	}
	return strings.Join(parts, " ")
}

func formatTeacherLesson(l model.TeacherLesson) string {
	if l == nil {
		return "-"
	}
	var parts []string
	if l.Subgroup != nil && *l.Subgroup > 0 {
		parts = append(parts, fmt.Sprintf("%d.", *l.Subgroup))
	}
	parts = append(parts, l.Group, l.Lesson)
	if l.Type != nil && *l.Type != "" {
		parts = append(parts, fmt.Sprintf("(%s)", *l.Type))
	}
	if l.Cabinet != nil && *l.Cabinet != "" {
		parts = append(parts, *l.Cabinet)
	}
	if l.Comment != nil && *l.Comment != "" {
		parts = append(parts, fmt.Sprintf("[%s]", *l.Comment))
	}
	return strings.Join(parts, " ")
}

func (r *Renderer) RenderDayTables(label string, tables []DayTable) (string, error) {
	if len(tables) == 0 {
		return "", fmt.Errorf("no tables to render")
	}

	const (
		padding    = 30
		lineHeight = 28
		colWidth   = 300
		headerH    = 40
	)

	totalWidth := padding*2 + len(tables)*colWidth + (len(tables)-1)*10
	maxRows := 0
	for _, t := range tables {
		if len(t.Lessons) > maxRows {
			maxRows = len(t.Lessons)
		}
	}
	totalHeight := padding + headerH + maxRows*lineHeight + padding

	dc := gg.NewContext(totalWidth, totalHeight)
	dc.SetColor(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	dc.Clear()

	fontPaths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	}
	loaded := false
	for _, p := range fontPaths {
		if err := dc.LoadFontFace(p, 14); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		return "", fmt.Errorf("no suitable font found")
	}

	dc.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: 255})
	dc.DrawStringAnchored(label, float64(totalWidth)/2, float64(padding)/2, 0.5, 0.5)

	y := float64(padding + 20)
	for i, t := range tables {
		x := float64(padding + i*(colWidth+10))

		dc.SetColor(color.RGBA{R: 41, G: 128, B: 185, A: 255})
		dc.DrawRectangle(x, y, float64(colWidth), float64(headerH))
		dc.Fill()

		dc.SetColor(color.RGBA{R: 255, G: 255, B: 255, A: 255})
		header := fmt.Sprintf("%s %s", t.Weekday, t.Date)
		dc.DrawStringAnchored(header, x+float64(colWidth)/2, y+float64(headerH)/2, 0.5, 0.5)

		ly := y + float64(headerH) + 5
		for _, row := range t.Lessons {
			dc.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: 255})
			line := fmt.Sprintf("%d. %s", row.Index, row.Content)
			dc.DrawString(line, x+5, ly)
			ly += float64(lineHeight)
		}
	}

	outputPath := filepath.Join(r.outputDir, fmt.Sprintf("timetable_%d.png", time.Now().UnixNano()))

	img := dc.Image()
	return savePNG(img, outputPath)
}

func savePNG(img image.Image, path string) (string, error) {
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return path, nil
}

func weekdayName(date string) string {
	t, err := time.Parse("02.01.2006", date)
	if err != nil {
		return ""
	}
	names := []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
	return names[t.Weekday()]
}
