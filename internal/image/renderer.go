package image

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fogleman/gg"
)

type Renderer struct {
	outputDir string
	cacheDir  string
}

func NewRenderer(outputDir string) *Renderer {
	os.MkdirAll(outputDir, 0755)
	return &Renderer{outputDir: outputDir, cacheDir: outputDir}
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
	Date     string
	Weekday  string
	Lessons  []LessonRow
}

type LessonRow struct {
	Number  int
	Cells   []string
}

var groupColumns = []Column{
	{Title: "№", Width: 30, Align: "center"},
	{Title: "Предмет", Width: 250, Align: "left"},
	{Title: "Вид", Width: 60, Align: "center"},
	{Title: "Аудитория", Width: 80, Align: "center"},
	{Title: "Преподаватель", Width: 200, Align: "left"},
}

var teacherColumns = []Column{
	{Title: "№", Width: 30, Align: "center"},
	{Title: "Предмет", Width: 250, Align: "left"},
	{Title: "Вид", Width: 60, Align: "center"},
	{Title: "Аудитория", Width: 80, Align: "center"},
	{Title: "Группа", Width: 80, Align: "center"},
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
		padding      = 20
		headerH      = 36
		rowH         = 28
		titleH       = 60
		footerH      = 40
		cellPadX     = 5
		dpr          = 1
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

	tableH := headerH + maxLessons*rowH + 10
	totalW := colsWidth*cellsPerRow + padding*2
	totalH := titleH + rows*tableH + footerH + padding*2

	dc := gg.NewContext(totalW*dpr, totalH*dpr)
	dc.SetColor(color.White)
	dc.Clear()

	fontPath := findFont()
	if fontPath == "" {
		return "", fmt.Errorf("no suitable font found on this system")
	}

	dc.LoadFontFace(fontPath, float64(14*dpr))
	dc.SetColor(color.Black)

	title := fmt.Sprintf("Группа - %s", ti.Group)
	if !isGroup {
		title = fmt.Sprintf("Преподаватель - %s", ti.Teacher)
	}
	dc.DrawStringAnchored(title, float64(totalW*dpr)/2, float64(padding*dpr+30*dpr), 0.5, 0.5)

	for row := 0; row < rows; row++ {
		for col := 0; col < cellsPerRow; col++ {
			idx := row*cellsPerRow + col
			if idx >= len(ti.Days) {
				break
			}
			day := ti.Days[idx]

			tx := float64((padding + col*colsWidth) * dpr)
			ty := float64((titleH + row*tableH + padding) * dpr)

			dc.SetColor(color.RGBA{R: 41, G: 128, B: 185, A: 255})
			dc.DrawRectangle(tx, ty, float64(colsWidth*dpr), float64(headerH*dpr))
			dc.Fill()

			dc.SetColor(color.White)
			dc.LoadFontFace(fontPath, float64(13*dpr))
			headerText := fmt.Sprintf("%s, %s", day.Weekday, day.Date)
			dc.DrawStringAnchored(headerText, tx+float64(colsWidth*dpr)/2, ty+float64(headerH*dpr)/2, 0.5, 0.5)

			dc.SetColor(color.RGBA{R: 240, G: 240, B: 240, A: 255})
			dc.DrawRectangle(tx, ty+float64(headerH*dpr), float64(colsWidth*dpr), float64(headerH*dpr))
			dc.Fill()

			dc.SetColor(color.Black)
			dc.LoadFontFace(fontPath, float64(11*dpr))
			cx := tx
			for _, c := range cols {
				titleText := c.Title
				dc.DrawStringAnchored(titleText, cx+float64(c.Width*dpr)/2, ty+float64(headerH*dpr)+float64(headerH*dpr)/2, 0.5, 0.5)
				cx += float64(c.Width * dpr)
			}

			ly := ty + float64(2*headerH*dpr) + 2
			dc.LoadFontFace(fontPath, float64(12*dpr))
			for i := 0; i < maxLessons; i++ {
				if i < len(day.Lessons) {
				lesson := day.Lessons[i]
					cx := tx
					for ci, c := range cols {
						if ci < len(lesson.Cells) {
							dc.SetColor(color.Black)
							if c.Align == "center" {
								dc.DrawStringAnchored(lesson.Cells[ci], cx+float64(c.Width*dpr)/2, ly, 0.5, 0.5)
							} else {
								dc.DrawString(lesson.Cells[ci], cx+float64(cellPadX*dpr), ly)
							}
						}
						cx += float64(c.Width * dpr)
					}
				}
				ly += float64(rowH * dpr)
			}

			dc.SetColor(color.RGBA{R: 200, G: 200, B: 200, A: 255})
			dc.DrawRectangle(tx, ty, float64(colsWidth*dpr), float64(tableH*dpr))
			dc.Stroke()
		}
	}

	dc.SetColor(color.RGBA{R: 150, G: 150, B: 150, A: 255})
	dc.LoadFontFace(fontPath, float64(8*dpr))
	fy := float64((totalH - footerH/2) * dpr)
	dc.DrawString("TG: https://t.me/mgkect_info_bot", float64(padding*dpr), fy)
	dc.DrawString(fmt.Sprintf("Сгенерировано: %s", time.Now().Format("02.01.2006 15:04")), float64(totalW*dpr-padding*dpr), fy)

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

func findFont() string {
	candidates := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
		"C:/Windows/Fonts/arial.ttf",
		"C:/Windows/Fonts/calibri.ttf",
		"C:/Windows/Fonts/segoeui.ttf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
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
