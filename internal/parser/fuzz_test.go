package parser

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
)

var callsTimeValueRe = regexp.MustCompile(`^\d{2}:\d{2}$`)

func fuzzSeeds() []string {
	seeds := []string{
		`<html><body><h2>Группа - 63ТП</h2><table border="1"><tr><th rowspan="2">№</th><th colspan="2">Понедельник, 31.08.2026</th></tr><tr><th class="sub">D</th><th class="sub">A</th></tr><tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td class="sub">101</td></tr></table></body></html>`,
		`<html><body><h2>Преподаватель - Иванов А.А.</h2><table border="1"><tr><th rowspan="2">№</th><th colspan="2">Вторник, 01.09.2026</th></tr><tr><th>1</th><td>63ТП-Математика<br>(Лек)</td><td class="sub">101</td></tr></table></body></html>`,
		`<html><body><h1>Расписание звонков</h1><table><tr><td>1 пара</td><td>8.00 – 8.45<br>8.55 – 9.40</td></tr></table></body></html>`,
		`<html></html>`,
		`<table><tr><td>`,
	}

	for _, name := range []string{"groups.html.gz", "teachers.html.gz"} {
		if data := readFuzzCorpus(name); len(data) > 0 {
			seeds = append(seeds, string(data))
		}
	}

	return seeds
}

func readFuzzCorpus(name string) []byte {
	file, err := os.Open(filepath.Join("testdata", "old_parser", name))
	if err != nil {
		return nil
	}
	defer file.Close()

	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil
	}
	return data
}

func fuzzDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Skipf("html is not parseable: %v", err)
	}
	return doc
}

func assertNoMarkup(t *testing.T, what, value string) {
	t.Helper()

	lower := strings.ToLower(value)
	if strings.Contains(lower, "</") || strings.Contains(lower, "<br") || strings.Contains(lower, "<td") {
		t.Fatalf("%s leaked markup: %q", what, value)
	}
	if strings.Contains(value, "\u00a0") {
		t.Fatalf("%s kept a non-breaking space: %q", what, value)
	}
}

func assertDayLabel(t *testing.T, html, day string) {
	t.Helper()

	if day == "" {
		return
	}
	if _, err := time.Parse("02.01.2006", day); err == nil {
		return
	}
	if weekdayRe.MatchString(day) {
		return
	}
	if strings.Contains(html, day) {
		return
	}
	t.Fatalf("day label %q was invented by the parser", day)
}

func checkGroupExplain(t *testing.T, lesson *model.GroupLessonExplain) {
	t.Helper()

	if lesson == nil {
		return
	}
	assertNoMarkup(t, "group lesson", lesson.Lesson)
	if lesson.Type != nil {
		assertNoMarkup(t, "group type", *lesson.Type)
	}
	if lesson.Teacher != nil {
		assertNoMarkup(t, "group teacher", *lesson.Teacher)
	}
	if lesson.Cabinet != nil {
		assertNoMarkup(t, "group cabinet", *lesson.Cabinet)
	}
	if lesson.Subgroup != nil && *lesson.Subgroup < 1 {
		t.Fatalf("subgroup %d is not a positive number", *lesson.Subgroup)
	}
}

func checkGroupLesson(t *testing.T, lesson model.GroupLesson) {
	t.Helper()

	switch value := lesson.(type) {
	case nil:
	case *model.GroupLessonExplain:
		checkGroupExplain(t, value)
	case []*model.GroupLessonExplain:
		for _, item := range value {
			checkGroupExplain(t, item)
		}
	default:
		t.Fatalf("unexpected group lesson type %T", lesson)
	}
}

func checkTeacherDay(t *testing.T, html string, day model.TeacherDay) {
	t.Helper()

	assertDayLabel(t, html, day.Day)
	for _, lesson := range day.Lessons {
		if lesson == nil {
			continue
		}
		assertNoMarkup(t, "teacher lesson", lesson.Lesson)
		assertNoMarkup(t, "teacher group", lesson.Group)
		if lesson.Type != nil {
			assertNoMarkup(t, "teacher type", *lesson.Type)
		}
		if lesson.Cabinet != nil {
			assertNoMarkup(t, "teacher cabinet", *lesson.Cabinet)
		}
		if lesson.Subgroup != nil && *lesson.Subgroup < 1 {
			t.Fatalf("subgroup %d is not a positive number", *lesson.Subgroup)
		}
	}
}

func canonicalJSON(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func FuzzGroupParserStaysStableAndClean(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, html string) {
		doc := fuzzDocument(t, html)

		groups, _ := NewGroupParser(doc).Parse()
		for name, group := range groups {
			if strings.TrimSpace(name) == "" {
				t.Fatal("an empty group key was stored")
			}
			if group == nil {
				t.Fatalf("group %q has no body", name)
			}
			if group.Group == "" {
				t.Fatalf("group %q lost its label", name)
			}
			for _, day := range group.Days {
				assertDayLabel(t, html, day.Day)
				for _, lesson := range day.Lessons {
					checkGroupLesson(t, lesson)
				}
			}
		}

		first := canonicalJSON(t, groups)
		reparsed, _ := NewGroupParser(doc).Parse()
		if second := canonicalJSON(t, reparsed); second != first {
			t.Fatalf("reparsing the same document changed the result:\nfirst:  %s\nsecond: %s", first, second)
		}
	})
}

func FuzzTeacherParserStaysStableAndClean(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, html string) {
		doc := fuzzDocument(t, html)

		teachers, _ := NewTeacherParser(doc).Parse()
		for name, teacher := range teachers {
			if strings.TrimSpace(name) == "" {
				t.Fatal("an empty teacher key was stored")
			}
			if teacher == nil {
				t.Fatalf("teacher %q has no body", name)
			}
			if teacher.Teacher == "" {
				t.Fatalf("teacher %q lost its label", name)
			}
			for _, day := range teacher.Days {
				checkTeacherDay(t, html, day)
			}
		}

		first := canonicalJSON(t, teachers)
		reparsed, _ := NewTeacherParser(doc).Parse()
		if second := canonicalJSON(t, reparsed); second != first {
			t.Fatalf("reparsing the same document changed the result:\nfirst:  %s\nsecond: %s", first, second)
		}
	})
}

func FuzzCallsParserStaysStable(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, html string) {
		doc := fuzzDocument(t, html)

		variants, _ := ParseCallsVariants(doc)
		for _, variant := range variants {
			assertNoMarkup(t, "calls campus", variant.Name)
			for _, slot := range variant.Schedule.Weekdays {
				assertCallsSlot(t, slot)
			}
			for _, slot := range variant.Schedule.Saturday {
				assertCallsSlot(t, slot)
			}
		}

		first := canonicalJSON(t, variants)
		reparsed, _ := ParseCallsVariants(doc)
		if second := canonicalJSON(t, reparsed); second != first {
			t.Fatalf("reparsing the same document changed the result:\nfirst:  %s\nsecond: %s", first, second)
		}
	})
}

func assertCallsSlot(t *testing.T, slot [2][2]string) {
	t.Helper()

	for _, pair := range slot {
		for _, value := range pair {
			if !callsTimeValueRe.MatchString(value) {
				t.Fatalf("calls time %q is not HH:MM", value)
			}
			if _, err := time.Parse("15:04", value); err != nil {
				t.Fatalf("calls time %q does not parse: %v", value, err)
			}
		}
		if pair[1] != "00:00" && pair[0] > pair[1] {
			t.Fatalf("calls pair %v starts after it ends", pair)
		}
	}
}

func FuzzParseCallSlotKeepsTimesValid(f *testing.F) {
	for _, seed := range []string{
		"1 пара 8.00 – 8.45\n8.55 – 9.40",
		"09:00-09:45",
		"8.00&ndash;8.45",
		"",
		"00:00 – 23:59",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		slot := parseCallSlot(text)
		if slot == nil {
			return
		}
		assertCallsSlot(t, *slot)
	})
}
