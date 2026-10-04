package timetable

import (
	"fmt"
	"sort"
	"time"
)

type Day struct {
	Date    string         `json:"day"`
	Index   int64          `json:"index,omitempty"`
	Lessons []Lesson       `json:"lessons"`
	Extra   map[string]any `json:"extra,omitempty"`
}

func (d Day) Time() (time.Time, error) {
	return ParseDate(d.Date)
}

func (d Day) Validate() error {
	if _, err := ParseDate(d.Date); err != nil {
		return err
	}
	for i := range d.Lessons {
		if err := d.Lessons[i].Validate(); err != nil {
			return fmt.Errorf("day %q: %v", d.Date, err)
		}
	}
	return nil
}

func (d *Day) SortLessons() {
	sort.Slice(d.Lessons, func(i, j int) bool { return d.Lessons[i].Num < d.Lessons[j].Num })
}
