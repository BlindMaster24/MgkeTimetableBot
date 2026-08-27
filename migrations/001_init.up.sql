CREATE TABLE IF NOT EXISTS timetable_archive (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    day INTEGER NOT NULL,
    "group" TEXT,
    teacher TEXT,
    data TEXT NOT NULL,
    UNIQUE(day, "group"),
    UNIQUE(day, teacher)
);

CREATE INDEX IF NOT EXISTS idx_group_day ON timetable_archive("group", day);
CREATE INDEX IF NOT EXISTS idx_teacher_day ON timetable_archive(teacher, day);
