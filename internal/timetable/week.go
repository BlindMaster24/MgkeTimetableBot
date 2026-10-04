package timetable

import (
	"fmt"
	"sort"
)

type Week struct {
	Days []Day `json:"days"`
}

func (w Week) Validate() error {
	seen := make(map[string]bool, len(w.Days))
	for i := range w.Days {
		if err := w.Days[i].Validate(); err != nil {
			return err
		}
		if seen[w.Days[i].Date] {
			return fmt.Errorf("week repeats day %q", w.Days[i].Date)
		}
		seen[w.Days[i].Date] = true
	}
	return nil
}

func (w *Week) Sort() {
	for i := range w.Days {
		w.Days[i].SortLessons()
	}
	sort.Slice(w.Days, func(i, j int) bool {
		left, leftErr := ParseDate(w.Days[i].Date)
		right, rightErr := ParseDate(w.Days[j].Date)
		if leftErr != nil || rightErr != nil {
			return w.Days[i].Date < w.Days[j].Date
		}
		return left.Before(right)
	})
}
