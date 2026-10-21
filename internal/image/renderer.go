package image

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
)

//go:embed mgke.png
var logoPNG []byte

type Renderer struct {
	outputDir string
	cacheDir  string
	fontPath  string
	now       func() time.Time
}

func NewRenderer(outputDir string) *Renderer {
	os.MkdirAll(outputDir, 0755)
	return &Renderer{outputDir: outputDir, cacheDir: outputDir}
}

func (r *Renderer) SetFontPath(path string) {
	r.fontPath = path
}

func (r *Renderer) SetNow(now func() time.Time) {
	r.now = now
}

func (r *Renderer) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

type Column struct {
	Title string
	Width int
	Align string
}

type TableData struct {
	Title   string
	Columns []Column
	Rows    [][]string
}

type TimetableImage struct {
	Group   string
	Teacher string
	Days    []DayData
}

type DayData struct {
	Date    string
	Weekday string
	Lessons []LessonRow
}

type LessonRow struct {
	Number int
	Cells  []string
}

var groupColumns = []Column{
	{Title: "№", Width: 30, Align: "center"},
	{Title: "Предмет", Width: 250, Align: "left"},
	{Title: "Вид", Width: 60, Align: "center"},
	{Title: "Аудитория", Width: 100, Align: "center"},
	{Title: "Преподаватель", Width: 190, Align: "left"},
}

var teacherColumns = []Column{
	{Title: "№", Width: 30, Align: "center"},
	{Title: "Предмет", Width: 250, Align: "left"},
	{Title: "Вид", Width: 60, Align: "center"},
	{Title: "Аудитория", Width: 100, Align: "center"},
	{Title: "Группа", Width: 68, Align: "center"},
}

func (r *Renderer) RenderGroupImage(group string, days []DayData) (string, error) {
	return r.renderTimetable(TimetableImage{Group: group, Days: days}, true)
}

func (r *Renderer) RenderTeacherImage(teacher string, days []DayData) (string, error) {
	return r.renderTimetable(TimetableImage{Teacher: teacher, Days: days}, false)
}

func (r *Renderer) renderTimetable(ti TimetableImage, isGroup bool) (string, error) {
	if len(ti.Days) == 0 {
		return "", fmt.Errorf("no days to render")
	}

	cols := groupColumns
	if !isGroup {
		cols = teacherColumns
	}

	const (
		padding     = 10
		titleBandH  = 60
		titleFont   = 30
		dayTitleH   = 30
		colHeaderH  = 26
		minRowH     = 28
		lineH       = 18
		cellPadX    = 6
		cellPadY    = 5
		footerBandH = 40
		footerFont  = 8
		logoSize    = 70
	)

	colsWidth := 0
	for _, c := range cols {
		colsWidth += c.Width
	}

	cellsPerRow := 2
	rows := (len(ti.Days) + cellsPerRow - 1) / cellsPerRow

	maxLessons := 0
	for _, d := range ti.Days {
		if len(d.Lessons) > maxLessons {
			maxLessons = len(d.Lessons)
		}
	}
	if maxLessons < 3 {
		maxLessons = 3
	}

	rowHeights := make([][]float64, len(ti.Days))
	maxTableH := 0.0
	for di, day := range ti.Days {
		heights := make([]float64, maxLessons)
		for i := range heights {
			heights[i] = minRowH
			if i >= len(day.Lessons) {
				continue
			}
			lines := 1
			for _, cell := range day.Lessons[i].Cells {
				if n := len(textLines(cell)); n > lines {
					lines = n
				}
			}
			if want := float64(lines*lineH + 2*cellPadY); want > heights[i] {
				heights[i] = want
			}
		}
		rowHeights[di] = heights

		tableH := float64(dayTitleH+colHeaderH) + 4
		for _, h := range heights {
			tableH += h
		}
		if tableH > maxTableH {
			maxTableH = tableH
		}
	}

	totalW := colsWidth*cellsPerRow + padding*2
	totalH := titleBandH + int(maxTableH)*rows + footerBandH
	if float64(int(maxTableH)) < maxTableH {
		totalH++
	}

	dc := gg.NewContext(totalW, totalH)
	dc.SetColor(color.White)
	dc.Clear()

	fontPath := r.fontPath
	if fontPath == "" {
		fontPath = findFont()
	}
	if fontPath == "" {
		return "", fmt.Errorf("no suitable font found on this system")
	}

	if logo, err := png.Decode(bytes.NewReader(logoPNG)); err == nil {
		bounds := logo.Bounds()
		if bounds.Dx() > 0 && bounds.Dy() > 0 {
			scale := float64(logoSize) / float64(bounds.Dx())
			dc.Push()
			dc.Translate(padding, padding)
			dc.Scale(scale, scale)
			dc.DrawImageAnchored(logo, 0, 0, 0, 0)
			dc.Pop()
		}
	}

	dc.SetColor(color.Black)
	dc.LoadFontFace(fontPath, titleFont)
	title := fmt.Sprintf("Группа - %s", ti.Group)
	if !isGroup {
		title = fmt.Sprintf("Преподаватель - %s", ti.Teacher)
	}
	dc.DrawStringAnchored(title, float64(totalW)/2, 45, 0.5, 0.5)

	grid := color.RGBA{R: 205, G: 205, B: 205, A: 255}
	headFill := color.RGBA{R: 242, G: 242, B: 242, A: 255}
	subHeadFill := color.RGBA{R: 248, G: 248, B: 248, A: 255}

	for row := 0; row < rows; row++ {
		for col := 0; col < cellsPerRow; col++ {
			idx := row*cellsPerRow + col
			if idx >= len(ti.Days) {
				break
			}
			day := ti.Days[idx]
			heights := rowHeights[idx]

			tx := float64(padding + col*colsWidth)
			ty := float64(titleBandH) + float64(row)*maxTableH
			tableW := float64(colsWidth)

			dc.SetColor(headFill)
			dc.DrawRectangle(tx, ty, tableW, dayTitleH)
			dc.Fill()

			dc.SetColor(color.Black)
			dc.LoadFontFace(fontPath, 18)
			dc.DrawStringAnchored(fmt.Sprintf("%s, %s", day.Weekday, day.Date), tx+tableW/2, ty+float64(dayTitleH)/2, 0.5, 0.5)

			dc.SetColor(subHeadFill)
			dc.DrawRectangle(tx, ty+dayTitleH, tableW, colHeaderH)
			dc.Fill()

			dc.SetColor(color.Black)
			dc.LoadFontFace(fontPath, 14)
			headerY := ty + float64(dayTitleH) + float64(colHeaderH)/2
			cx := tx
			for _, c := range cols {
				dc.DrawStringAnchored(c.Title, cx+float64(c.Width)/2, headerY, 0.5, 0.5)
				cx += float64(c.Width)
			}

			bodyTop := ty + float64(dayTitleH+colHeaderH)
			ry := bodyTop
			for i := 0; i < maxLessons; i++ {
				rowH := heights[i]
				if i < len(day.Lessons) {
					lesson := day.Lessons[i]
					cx := tx
					for ci, c := range cols {
						if ci < len(lesson.Cells) {
							lines := textLines(lesson.Cells[ci])
							dc.SetColor(color.Black)
							dc.LoadFontFace(fontPath, 14)
							startY := ry + float64(cellPadY) + float64(lineH)/2
							for li, line := range lines {
								if c.Align == "center" {
									dc.DrawStringAnchored(line, cx+float64(c.Width)/2, startY+float64(li*lineH), 0.5, 0.5)
								} else {
									dc.DrawStringAnchored(line, cx+float64(cellPadX), startY+float64(li*lineH), 0, 0.5)
								}
							}
						}
						cx += float64(c.Width)
					}
				}
				ry += rowH
			}

			bodyBottom := bodyTop
			for _, h := range heights {
				bodyBottom += h
			}

			dc.SetColor(grid)
			dc.SetLineWidth(1)
			cx = tx
			for _, c := range cols {
				cx += float64(c.Width)
				dc.DrawLine(cx, ty, cx, bodyBottom)
			}
			dc.DrawLine(tx, ty+dayTitleH, tx+tableW, ty+dayTitleH)
			dc.DrawLine(tx, bodyTop, tx+tableW, bodyTop)
			ryLine := bodyTop
			for _, h := range heights {
				ryLine += h
				dc.DrawLine(tx, ryLine, tx+tableW, ryLine)
			}
			dc.Stroke()

			dc.SetColor(grid)
			dc.DrawRectangle(tx, ty, tableW, bodyBottom-ty)
			dc.Stroke()
		}
	}

	dc.SetColor(color.RGBA{R: 150, G: 150, B: 150, A: 255})
	dc.LoadFontFace(fontPath, footerFont)
	baseY := float64(totalH - 8)
	dc.DrawStringAnchored("Viber: https://mgke.keller.by/viber", padding, baseY-18, 0, 0.5)
	dc.DrawStringAnchored("TG: https://t.me/mgke_slave_bot", padding, baseY-9, 0, 0.5)
	dc.DrawStringAnchored("VK: https://vk.com/mgke_slave", padding, baseY, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Сгенерированно в: %s", r.clock().Format("02.01.2006 15:04:05,000")), float64(totalW-padding), baseY-9, 1, 0.5)
	dc.DrawStringAnchored("Таблица с расписанием была сгенерированна в ботах Алексея Костюка из 63 группы", float64(totalW-padding), baseY, 1, 0.5)

	fname := fmt.Sprintf("timetable_%d.png", time.Now().UnixNano())
	outPath := filepath.Join(r.outputDir, fname)
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, dc.Image()); err != nil {
		return "", err
	}
	return outPath, nil
}

func (r *Renderer) Cleanup(maxAge time.Duration) {
	entries, err := os.ReadDir(r.outputDir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(r.outputDir, e.Name()))
		}
	}
}

var (
	fontPathOnce   sync.Once
	fontPathCached string
)

var fontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"/System/Library/Fonts/Supplemental/Verdana.ttf",
	"/Library/Fonts/Arial.ttf",
	"C:/Windows/Fonts/arial.ttf",
	"C:/Windows/Fonts/calibri.ttf",
	"C:/Windows/Fonts/segoeui.ttf",
}

var fontDirs = []string{
	"/System/Library/Fonts/Supplemental",
	"/Library/Fonts",
	"/System/Library/Fonts",
	"/usr/share/fonts/truetype",
	"/usr/local/share/fonts",
	"/usr/share/fonts",
	"C:/Windows/Fonts",
}

func FontPath() string {
	return findFont()
}

func findFont() string {
	fontPathOnce.Do(func() {
		fontPathCached = lookupFont()
	})
	return fontPathCached
}

func lookupFont() string {
	for _, path := range fontCandidates {
		if fontLoads(path) {
			return path
		}
	}
	for _, dir := range fontDirs {
		if path := firstFontIn(dir); path != "" {
			return path
		}
	}
	return ""
}

func fontLoads(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return gg.NewContext(1, 1).LoadFontFace(path, 12) == nil
}

func firstFontIn(dir string) string {
	for _, pattern := range []string{"*.ttf", "*.otf", "*/*.ttf", "*/*.otf", "*/*/*.ttf", "*/*/*.otf"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, path := range matches {
			name := strings.ToLower(filepath.Base(path))
			if strings.Contains(name, "emoji") || strings.Contains(name, "symbol") {
				continue
			}
			if fontLoads(path) {
				return path
			}
		}
	}
	return ""
}

func textLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func FormatGroupLesson(lesson any, index int) []string {
	if lesson == nil {
		return nil
	}
	switch v := lesson.(type) {
	case map[string]any:
		return []string{
			fmt.Sprintf("%d", index),
			getStr(v, "lesson"),
			getStr(v, "type"),
			getStrDefault(v, "cabinet", "-"),
			getStr(v, "teacher"),
		}
	case []any:
		var lessons []string
		for _, sub := range v {
			if subMap, ok := sub.(map[string]any); ok {
				parts := []string{
					fmt.Sprintf("%d", index),
					getSubgroupPrefix(subMap) + getStr(subMap, "lesson"),
					getStr(subMap, "type"),
					getStrDefault(subMap, "cabinet", "-"),
					getStr(subMap, "teacher"),
				}
				lessons = append(lessons, strings.Join(parts, "|||"))
			}
		}
		if len(lessons) == 1 {
			return strings.Split(lessons[0], "|||")
		}
		var result []string
		for i := 0; i < 5; i++ {
			var col []string
			for _, l := range lessons {
				parts := strings.Split(l, "|||")
				if i < len(parts) {
					col = append(col, parts[i])
				}
			}
			result = append(result, strings.Join(col, "\n"))
		}
		return result
	}
	return nil
}

func FormatTeacherLesson(lesson any, index int) []string {
	if lesson == nil {
		return nil
	}
	switch v := lesson.(type) {
	case map[string]any:
		return []string{
			fmt.Sprintf("%d", index),
			getStr(v, "lesson"),
			getStr(v, "type"),
			getStrDefault(v, "cabinet", "-"),
			getStrDefault(v, "group", "-"),
		}
	}
	return nil
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStrDefault(m map[string]any, key, def string) string {
	if v := getStr(m, key); v != "" {
		return v
	}
	return def
}

func getSubgroupPrefix(m map[string]any) string {
	if sub, ok := m["subgroup"].(float64); ok && sub > 0 {
		return fmt.Sprintf("%d. ", int(sub))
	}
	return ""
}

func weekdayName(date string) string {
	t, err := time.Parse("02.01.2006", date)
	if err != nil {
		return ""
	}
	names := []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
	return names[t.Weekday()]
}
