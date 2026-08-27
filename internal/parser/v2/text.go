package v2

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

var dateRe = regexp.MustCompile(`(\d{1,2}\.\d{1,2}\.\d{2,4})`)
var dayNames = []string{
	"понедельник", "вторник", "среда", "четверг",
	"пятница", "суббота", "воскресенье",
}

func cleanText(text string) string {
	if text == "" {
		return ""
	}
	normalized := strings.TrimSpace(text)
	normalized = strings.Join(strings.Fields(normalized), " ")
	if len(normalized) == 0 {
		return ""
	}
	return normalized
}

func normalizeDate(value string) string {
	parts := strings.Split(value, ".")
	if len(parts) < 3 {
		return ""
	}
	day, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	month, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	yearStr := strings.TrimSpace(parts[2])
	if len(yearStr) == 2 {
		yearStr = "20" + yearStr
	}
	year, _ := strconv.Atoi(yearStr)
	if day == 0 || month == 0 || year == 0 {
		return ""
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Format("02.01.2006")
}

func parseDayLabel(value string) (day string, weekday string) {
	text := cleanText(value)
	if text == "" {
		return "", ""
	}
	dateMatch := dateRe.FindString(text)
	if dateMatch == "" {
		return "", ""
	}
	normalized := normalizeDate(dateMatch)
	if normalized == "" {
		return "", ""
	}
	lower := strings.ToLower(text)
	for _, name := range dayNames {
		if strings.Contains(lower, name) {
			weekday = capitalize(name)
			break
		}
	}
	return normalized, weekday
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	return string([]rune{unicode.ToUpper(runes[0])}) + string(runes[1:])
}

func extractLines(cell *goquery.Selection) []string {
	if cell == nil || cell.Length() == 0 {
		return nil
	}

	var lines []string
	var buffer strings.Builder

	flush := func() {
		normalized := cleanText(buffer.String())
		if normalized != "" {
			lines = append(lines, normalized)
		}
		buffer.Reset()
	}

	cell.Contents().Each(func(_ int, n *goquery.Selection) {
		if goquery.NodeName(n) == "br" {
			flush()
			return
		}
		text := n.Text()
		buffer.WriteString(text)
	})
	flush()

	return lines
}

func parseLessonNumber(value string) int {
	text := cleanText(value)
	if text == "" {
		return 0
	}
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(text)
	if match == "" {
		return 0
	}
	num, _ := strconv.Atoi(match)
	return num
}

func isTeacherLine(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	re := regexp.MustCompile(`^[А-ЯЁA-Z][а-яёa-z\-]+\s+[А-ЯЁA-Z]\.\s*[А-ЯЁA-Z]?\.?$`)
	return re.MatchString(text)
}
