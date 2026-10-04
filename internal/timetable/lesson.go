package timetable

import (
	"fmt"
	"strings"
)

type Lesson struct {
	Num      int            `json:"num"`
	Subject  string         `json:"subject"`
	Type     string         `json:"type,omitempty"`
	Teacher  string         `json:"teacher,omitempty"`
	Group    string         `json:"group,omitempty"`
	Room     string         `json:"room,omitempty"`
	Comment  string         `json:"comment,omitempty"`
	Subgroup int            `json:"subgroup,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

func (l Lesson) Validate() error {
	if l.Num < 1 {
		return fmt.Errorf("lesson number must be at least 1, got %d", l.Num)
	}
	if strings.TrimSpace(l.Subject) == "" {
		return fmt.Errorf("lesson %d must name a subject", l.Num)
	}
	if l.Subgroup < 0 {
		return fmt.Errorf("lesson %d holds a negative subgroup %d", l.Num, l.Subgroup)
	}
	return nil
}
