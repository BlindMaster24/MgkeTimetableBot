package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
)

func jsonRoundTrip(v any) any {
	data, _ := json.Marshal(v)
	var result any
	json.Unmarshal(data, &result)
	return result
}

const testGroupHTML = `<!DOCTYPE html>
<html><body>
<h2>Группа - 100</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table>
<tr>
<th>Пара</th>
<th colspan="2">Понедельник, 31.08.2026</th>
<th colspan="2">Вторник, 01.09.2026</th>
</tr>
<tr>
<td>1</td>
<td>Математика<br>Лек<br>Иванов И.И.</td>
<td>101</td>
<td>Физика<br>ЛР<br>Петров П.П.</td>
<td>202</td>
</tr>
<tr>
<td>2</td>
<td>Информатика<br>Пр<br>Сидоров С.С.</td>
<td>303</td>
<td>—</td>
<td>—</td>
</tr>
</table>
</body></html>`

const testTeacherHTML = `<!DOCTYPE html>
<html><body>
<h2>Преподаватель - Иванов И.И.</h2>
<h3>31.08.2026 - 06.09.2026</h3>
<table>
<tr>
<th>Пара</th>
<th colspan="2">Понедельник, 31.08.2026</th>
<th colspan="2">Вторник, 01.09.2026</th>
</tr>
<tr>
<td>1</td>
<td>100<br>Математика<br>(Лек)</td>
<td>101</td>
<td>200<br>Физика<br>(ЛР)</td>
<td>202</td>
</tr>
</table>
</body></html>`

const testCallsHTML = `<html><body>
<div class="entry"><div class="content">
<table class="table table-bordered">
<tbody>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br/>8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br/>10.45 &ndash; 11.30</td></tr>
<tr><td>3 пара</td><td>11.50 &ndash; 12.35<br/>12.45 &ndash; 13.30</td></tr>
</tbody>
</table>
</div></div>
</body></html>`

func docFromHTML(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func parseGroupsFromHTML(t *testing.T, html string) (map[string]any, string) {
	t.Helper()
	doc := docFromHTML(t, html)
	p := parser.NewGroupParser(doc)
	groups, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]any)
	for key, group := range groups {
		result[key] = jsonRoundTrip(map[string]any{
			"group": group.Group,
			"days":  group.Days,
		})
	}
	return result, p.ContentHash()
}

func parseTeachersFromHTML(t *testing.T, html string) (map[string]any, string) {
	t.Helper()
	doc := docFromHTML(t, html)
	p := parser.NewTeacherParser(doc)
	teachers, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]any)
	for key, teacher := range teachers {
		result[key] = jsonRoundTrip(map[string]any{
			"teacher": teacher.Teacher,
			"days":    teacher.Days,
		})
	}
	return result, p.ContentHash()
}

func parseCallsFromHTML(t *testing.T, html string) *cache.Schedule {
	t.Helper()
	doc := docFromHTML(t, html)
	return parser.ParseCallsSchedule(doc)
}

func TestIntegrationParseToCacheToDisplay(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.New(dir + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	groups, hash := parseGroupsFromHTML(t, testGroupHTML)
	if len(groups) == 0 {
		t.Fatal("expected groups from HTML")
	}
	t.Logf("parsed %d groups, hash: %s", len(groups), hash)
	c.SetGroups(groups, hash)

	teachers, tHash := parseTeachersFromHTML(t, testTeacherHTML)
	if len(teachers) == 0 {
		t.Fatal("expected teachers from HTML")
	}
	t.Logf("parsed %d teachers, hash: %s", len(teachers), tHash)
	c.SetTeachers(teachers, tHash)

	calls := parseCallsFromHTML(t, testCallsHTML)
	t.Logf("parsed %d weekday call slots", len(calls.Weekdays))
	if len(calls.Weekdays) == 0 {
		t.Fatal("expected call slots from HTML")
	}
	c.SetCalls(*calls, cache.Schedule{}, "site")

	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	c2, err := cache.New(dir + "/cache")
	if err != nil {
		t.Fatal(err)
	}

	cachedGroups := c2.GetGroups()
	if len(cachedGroups) == 0 {
		t.Fatal("expected groups after reload")
	}

	cachedTeachers := c2.GetTeachers()
	if len(cachedTeachers) == 0 {
		t.Fatal("expected teachers after reload")
	}

	cachedCalls := c2.GetCalls()
	if len(cachedCalls.Active.Schedule.Weekdays) == 0 {
		t.Fatal("expected call slots after reload")
	}

	for _, f := range formatter.AllFormatters {
		for groupName, data := range cachedGroups {
			text := f.FormatGroupFull(groupName, extractDaysFromCache(data), formatter.FormatOptions{
				IsTelegram: true,
			})
			if text == "" {
				t.Errorf("formatter %s produced empty output for group %s", f.Label(), groupName)
			} else {
				t.Logf("formatter %s, group %s:\n%s", f.Label(), groupName, text)
			}
		}
	}

	for teacherName, data := range cachedTeachers {
		for _, f := range formatter.AllFormatters {
			text := f.FormatTeacherFull(teacherName, extractDaysFromCache(data), formatter.FormatOptions{
				IsTelegram: true,
			})
			if text == "" {
				t.Errorf("formatter %s produced empty output for teacher %s", f.Label(), teacherName)
			}
		}
	}
}

func TestIntegrationGroupScheduleVariants(t *testing.T) {
	variants := []struct {
		name  string
		html  string
		must  string
		minN  int
	}{
		{"standard", testGroupHTML, "100", 1},
		{"multi_line", `<html><body>
<h2>Группа - 200</h2>
<table>
<tr><th>Пара</th><th colspan="2">Понедельник, 31.08.2026</th></tr>
<tr><td>1</td><td>ЛР<br>СпецП<br>Тест<br>101</td><td>101</td></tr>
</table></body></html>`, "200", 1},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			groups, _ := parseGroupsFromHTML(t, v.html)
			if len(groups) < v.minN {
				t.Errorf("expected at least %d groups, got %d", v.minN, len(groups))
			}
			found := false
			for name := range groups {
				if strings.Contains(name, v.must) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected group containing %q", v.must)
			}
		})
	}
}

func TestIntegrationTeacherScheduleVariants(t *testing.T) {
	teachers, _ := parseTeachersFromHTML(t, testTeacherHTML)
	if len(teachers) == 0 {
		t.Fatal("expected teachers")
	}
	for name, data := range teachers {
		t.Logf("teacher: %s", name)
		daysRaw, ok := data.(map[string]any)
		if !ok {
			t.Fatal("expected map data")
		}
		daysArr, ok := daysRaw["days"].([]any)
		if !ok || len(daysArr) == 0 {
			t.Fatal("expected days array")
		}
	}
}

func TestIntegrationCallsParseMultipleTables(t *testing.T) {
	html := `<html><body>
<div class="entry"><div class="content">
<table>
<tbody>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br/>8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br/>10.45 &ndash; 11.30</td></tr>
</tbody>
</table>
<table>
<tbody>
<tr><td>1 пара</td><td>8.00 &ndash; 9.00</td></tr>
</tbody>
</table>
</div></div>
</body></html>`

	calls := parseCallsFromHTML(t, html)
	if calls == nil {
		t.Skip("multi-table HTML not supported by parser")
	}
	if len(calls.Weekdays) == 0 {
		t.Error("expected call slots from multi-table HTML")
	}
	t.Logf("parsed %d weekday slots from multi-table", len(calls.Weekdays))
}

func TestIntegrationCacheSaveLoadFullCycle(t *testing.T) {
	dir := t.TempDir()
	c, _ := cache.New(dir + "/cache")

	groups, gHash := parseGroupsFromHTML(t, testGroupHTML)
	c.SetGroups(groups, gHash)

	teachers, tHash := parseTeachersFromHTML(t, testTeacherHTML)
	c.SetTeachers(teachers, tHash)

	calls := parseCallsFromHTML(t, testCallsHTML)
	c.SetCalls(*calls, cache.Schedule{}, "site")

	c.SetTeam(map[string]string{"100": "Тест"}, []string{"team_hash"})
	c.SetSuccessUpdate(true)

	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	stats := c.Stats()
	if stats.GroupsCount == 0 {
		t.Error("expected groups in stats")
	}
	if stats.TeachersCount == 0 {
		t.Error("expected teachers in stats")
	}
	if !stats.SuccessUpdate {
		t.Error("expected SuccessUpdate true")
	}

	c2, _ := cache.New(dir + "/cache")
	stats2 := c2.Stats()
	if stats2.GroupsCount != stats.GroupsCount {
		t.Errorf("groups count mismatch: %d vs %d", stats2.GroupsCount, stats.GroupsCount)
	}
	if stats2.TeachersCount != stats.TeachersCount {
		t.Errorf("teachers count mismatch: %d vs %d", stats2.TeachersCount, stats.TeachersCount)
	}
}

func TestIntegrationFormatAllGroupsAllFormatters(t *testing.T) {
	groups, hash := parseGroupsFromHTML(t, testGroupHTML)
	if len(groups) == 0 {
		t.Fatal("no groups parsed")
	}

	c, _ := cache.New(t.TempDir() + "/cache")
	c.SetGroups(groups, hash)

	for i, f := range formatter.AllFormatters {
		for name, data := range groups {
			days := extractDaysFromCache(data)
			text := f.FormatGroupFull(name, days, formatter.FormatOptions{
				IsTelegram: true,
				ShowHeader: true,
				WeekLabel:  buildWeekLabelFromDays(days),
			})
			if text == "" {
				t.Errorf("formatter %d (%s) empty for group %s", i, f.Label(), name)
			}
		}
	}
}

func TestIntegrationCallsSlotsCount(t *testing.T) {
	calls := parseCallsFromHTML(t, testCallsHTML)
	if len(calls.Weekdays) < 2 {
		t.Errorf("expected at least 2 call slots, got %d", len(calls.Weekdays))
	}
	for i, slot := range calls.Weekdays {
		if slot[0][0] == "" || slot[0][1] == "" {
			t.Errorf("slot %d has empty time", i)
		}
	}
}

func TestIntegrationCacheConcurrentAccess(t *testing.T) {
	c, _ := cache.New(t.TempDir() + "/cache")

	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				groups, hash := parseGroupsFromHTML(t, testGroupHTML)
				c.SetGroups(groups, hash)
				c.GetGroups()
				c.GetTeachers()
				c.GetCalls()
				c.RecordHit()
			}
		}()
	}
	for i := 0; i < 5; i++ {
		<-done
	}

	stats := c.Stats()
	if stats.Hits != 250 {
		t.Errorf("expected 250 hits, got %d", stats.Hits)
	}
}

func extractDaysFromCache(data any) []map[string]any {
	daysRaw, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	daysArr, ok := daysRaw["days"].([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, d := range daysArr {
		if m, ok := d.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func buildWeekLabelFromDays(days []map[string]any) string {
	if len(days) == 0 {
		return ""
	}
	firstDay, _ := days[0]["day"].(string)
	lastDay, _ := days[len(days)-1]["day"].(string)
	t1, err1 := time.Parse("02.01.2006", firstDay)
	t2, err2 := time.Parse("02.01.2006", lastDay)
	if err1 != nil || err2 != nil {
		return ""
	}
	weekNum, _ := t1.ISOWeek()
	_ = weekNum
	return "Учебная неделя (" + t1.Format("02.01") + "-" + t2.Format("02.01") + ")"
}
