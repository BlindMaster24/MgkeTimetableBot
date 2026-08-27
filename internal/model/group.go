package model

type Groups map[string]*Group

type Group struct {
	Group       string     `json:"group"`
	Days        []GroupDay `json:"days"`
	LastNoticed int64      `json:"lastNoticedDay,omitempty"`
}

type GroupDay struct {
	Day     string        `json:"day"`
	Lessons []GroupLesson `json:"lessons"`
}

type GroupLesson interface{}
type SingleLesson = *GroupLessonExplain
type LessonGroup = []*GroupLessonExplain

func AsSingle(gl GroupLesson) *GroupLessonExplain {
	if gl == nil {
		return nil
	}
	if s, ok := gl.(*GroupLessonExplain); ok {
		return s
	}
	return nil
}

func AsArray(gl GroupLesson) []*GroupLessonExplain {
	if gl == nil {
		return nil
	}
	if a, ok := gl.([]*GroupLessonExplain); ok {
		return a
	}
	return nil
}

func IsLessonArray(gl GroupLesson) bool {
	_, ok := gl.([]*GroupLessonExplain)
	return ok
}

type GroupLessonExplain struct {
	Subgroup *int    `json:"subgroup,omitempty"`
	Lesson   string  `json:"lesson"`
	Type     *string `json:"type"`
	Teacher  *string `json:"teacher"`
	Cabinet  *string `json:"cabinet"`
	Comment  *string `json:"comment"`
}
