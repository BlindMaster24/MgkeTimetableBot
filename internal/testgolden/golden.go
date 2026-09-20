package testgolden

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

var (
	dateTimeRe   = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4},? \d{2}:\d{2}`)
	fullDateRe   = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)
	shortDateRe  = regexp.MustCompile(`\d{2}\.\d{2}`)
	weekdayRe    = regexp.MustCompile(`Понедельник|Вторник|Среда|Четверг|Пятница|Суббота|Воскресенье|Пн|Вт|Ср|Чт|Пт|Сб|Вс`)
	weekNumberRe = regexp.MustCompile(`№\s*\d+`)
	dayHintRe    = regexp.MustCompile(`\((сегодня|завтра)\)`)
	loadedAtRe   = regexp.MustCompile(`загружена \d.*? назад`)

	callbackDigitRe = regexp.MustCompile(`\d+`)
)

const (
	TodayToken    = "{{today}}"
	TomorrowToken = "{{tomorrow}}"
)

func Normalize(text string, now time.Time) string {
	out := dateTimeRe.ReplaceAllString(text, "{{datetime}}")
	out = strings.ReplaceAll(out, now.Format("02.01.2006"), TodayToken)
	out = strings.ReplaceAll(out, now.AddDate(0, 0, 1).Format("02.01.2006"), TomorrowToken)
	out = fullDateRe.ReplaceAllString(out, "{{date}}")
	out = shortDateRe.ReplaceAllString(out, "{{dd.mm}}")
	out = weekdayRe.ReplaceAllString(out, "{{wd}}")
	out = weekNumberRe.ReplaceAllString(out, "№{{week}}")
	out = dayHintRe.ReplaceAllString(out, "{{hint}}")
	out = loadedAtRe.ReplaceAllString(out, "загружена {{ago}} назад")
	out = strings.ReplaceAll(out, "👉 ", "{{now}} ")
	out = strings.ReplaceAll(out, " 👈", " {{now}}")
	return out
}

func NormalizeWeekNumber(data string, week int) string {
	out := callbackDigitRe.ReplaceAllStringFunc(data, func(digits string) string {
		number, err := strconv.Atoi(digits)
		if err != nil {
			return digits
		}
		switch number {
		case week:
			return "{{week}}"
		case week - 1:
			return "{{week-1}}"
		case week + 1:
			return "{{week+1}}"
		case week + 2:
			return "{{week+2}}"
		}
		return digits
	})
	return weekNumberRe.ReplaceAllString(out, "№{{week}}")
}

func NormalizeWeek(text string, now time.Time) string {
	out := Normalize(text, now)
	out = strings.ReplaceAll(out, " <i>{{hint}}</i>", "")
	out = strings.ReplaceAll(out, TodayToken, "{{date}}")
	out = strings.ReplaceAll(out, TomorrowToken, "{{date}}")
	return out
}

func NormalizeLineEndings(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func FirstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var wantLine, gotLine string
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n  want: " + wantLine + "\n  got:  " + gotLine
		}
	}
	return "no line difference found"
}

func RelevantWeek(now time.Time) utils.WeekIndex {
	week := utils.WeekIndexFromDate(now)
	if now.Weekday() == time.Sunday {
		week = utils.WeekIndexFromNumber(week.Value() + 1)
	}
	return week
}

func WeekDates(week utils.WeekIndex, days int) []string {
	first := week.FirstDayDate()
	dates := make([]string, 0, days)
	for i := 0; i < days; i++ {
		dates = append(dates, first.AddDate(0, 0, i).Format("02.01.2006"))
	}
	return dates
}
