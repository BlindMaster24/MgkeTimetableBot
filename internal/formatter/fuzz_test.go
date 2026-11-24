package formatter

import (
	"encoding/json"
	"testing"
)

// FuzzFormatGroupFull feeds arbitrary day/lesson JSON through the default
// formatter: the cache stores whatever the site returns, so a panic here would
// take the whole reply path down.
func FuzzFormatGroupFull(f *testing.F) {
	seeds := []string{
		`[{"day":"23.11.2026","weekday":"Понедельник","lessons":[{"lesson":"Математика","cabinet":"101","type":"Лек"}]}]`,
		`[{"day":"23.11.2026","lessons":[{"lesson":"A","subgroup":1},{"lesson":"B","subgroup":2}]}]`,
		`[{"day":"23.11.2026","lessons":[]}]`,
		`[{"day":"","lessons":null}]`,
		`[]`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		var days []map[string]any
		if err := json.Unmarshal([]byte(raw), &days); err != nil {
			return
		}
		opts := FormatOptions{}
		for _, index := range []int{0, 1, 2, 3} {
			f := GetByIndex(index)
			f.FormatGroupFull("100", days, opts)
			f.FormatTeacherFull("Иванов И.И.", days, opts)
		}
	})
}
