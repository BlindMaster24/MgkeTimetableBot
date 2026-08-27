package model

type Teachers map[string]*Teacher

type Teacher struct {
	Teacher      string      `json:"teacher"`
	Days         []TeacherDay `json:"days"`
	LastNoticed  int64       `json:"lastNoticedDay,omitempty"`
}

type TeacherDay struct {
	Day     string          `json:"day"`
	Lessons []TeacherLesson `json:"lessons"`
}

type TeacherLesson = *TeacherLessonExplain

type TeacherLessonExplain struct {
	Lesson   string  `json:"lesson"`
	Type     *string `json:"type"`
	Subgroup *int    `json:"subgroup,omitempty"`
	Group    string  `json:"group"`
	Cabinet  *string `json:"cabinet"`
	Comment  *string `json:"comment"`
}
