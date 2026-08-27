package archive

import (
	"database/sql"
	"encoding/json"

	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	_ "modernc.org/sqlite"
)

type ArchiveRow struct {
	Day     int64
	Group   *string
	Teacher *string
	Data    string
}

type Bounds struct {
	Min int64
	Max int64
}

type Repository struct {
	db *sql.DB
}

func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) DayIndexBounds() (Bounds, error) {
	var b Bounds
	err := r.db.QueryRow("SELECT COALESCE(MIN(day),0), COALESCE(MAX(day),0) FROM timetable_archive").Scan(&b.Min, &b.Max)
	return b, err
}

func (r *Repository) Groups() ([]string, error) {
	rows, err := r.db.Query("SELECT DISTINCT \"group\" FROM timetable_archive WHERE \"group\" IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (r *Repository) Teachers() ([]string, error) {
	rows, err := r.db.Query("SELECT DISTINCT teacher FROM timetable_archive WHERE teacher IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *Repository) GroupDay(dayIndex int64, group string) (*model.GroupDay, error) {
	var data string
	err := r.db.QueryRow("SELECT data FROM timetable_archive WHERE day = ? AND \"group\" = ?", dayIndex, group).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return parseGroupDay(dayIndex, data)
}

func (r *Repository) TeacherDay(dayIndex int64, teacher string) (*model.TeacherDay, error) {
	var data string
	err := r.db.QueryRow("SELECT data FROM timetable_archive WHERE day = ? AND teacher = ?", dayIndex, teacher).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return parseTeacherDay(dayIndex, data)
}

func (r *Repository) GroupDaysByRange(min, max int64, group string) ([]model.GroupDay, error) {
	rows, err := r.db.Query("SELECT day, data FROM timetable_archive WHERE \"group\" = ? AND day BETWEEN ? AND ? ORDER BY day ASC", group, min, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanGroupDays(rows)
}

func (r *Repository) TeacherDaysByRange(min, max int64, teacher string) ([]model.TeacherDay, error) {
	rows, err := r.db.Query("SELECT day, data FROM timetable_archive WHERE teacher = ? AND day BETWEEN ? AND ? ORDER BY day ASC", teacher, min, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTeacherDays(rows)
}

func (r *Repository) GroupDays(group string, fromDay *int64) ([]model.GroupDay, error) {
	query := "SELECT day, data FROM timetable_archive WHERE \"group\" = ?"
	args := []interface{}{group}

	if fromDay != nil {
		query += " AND day >= ?"
		args = append(args, *fromDay)
	}

	query += " ORDER BY day ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanGroupDays(rows)
}

func (r *Repository) TeacherDays(teacher string, fromDay *int64) ([]model.TeacherDay, error) {
	query := "SELECT day, data FROM timetable_archive WHERE teacher = ?"
	args := []interface{}{teacher}

	if fromDay != nil {
		query += " AND day >= ?"
		args = append(args, *fromDay)
	}

	query += " ORDER BY day ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTeacherDays(rows)
}

type AppendDay struct {
	Type  string
	Value string
	Day   interface{}
}

func (r *Repository) AppendDays(entries []AppendDay) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtGroup, err := tx.Prepare("INSERT INTO timetable_archive (day, \"group\", data) VALUES (?, ?, ?) ON CONFLICT(day, \"group\") DO UPDATE SET data = excluded.data")
	if err != nil {
		return err
	}
	defer stmtGroup.Close()

	stmtTeacher, err := tx.Prepare("INSERT INTO timetable_archive (day, teacher, data) VALUES (?, ?, ?) ON CONFLICT(day, teacher) DO UPDATE SET data = excluded.data")
	if err != nil {
		return err
	}
	defer stmtTeacher.Close()

	for _, e := range entries {
		switch e.Type {
		case "group":
			day, data := extractDayData(e.Day)
			if _, err := stmtGroup.Exec(day, e.Value, data); err != nil {
				return err
			}
		case "teacher":
			day, data := extractDayData(e.Day)
			if _, err := stmtTeacher.Exec(day, e.Value, data); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func scanGroupDays(rows *sql.Rows) ([]model.GroupDay, error) {
	var result []model.GroupDay
	for rows.Next() {
		var dayIndex int64
		var data string
		if err := rows.Scan(&dayIndex, &data); err != nil {
			return nil, err
		}
		d, err := parseGroupDay(dayIndex, data)
		if err != nil {
			return nil, err
		}
		if d != nil {
			result = append(result, *d)
		}
	}
	return result, rows.Err()
}

func scanTeacherDays(rows *sql.Rows) ([]model.TeacherDay, error) {
	var result []model.TeacherDay
	for rows.Next() {
		var dayIndex int64
		var data string
		if err := rows.Scan(&dayIndex, &data); err != nil {
			return nil, err
		}
		d, err := parseTeacherDay(dayIndex, data)
		if err != nil {
			return nil, err
		}
		if d != nil {
			result = append(result, *d)
		}
	}
	return result, rows.Err()
}

func parseGroupDay(dayIndex int64, data string) (*model.GroupDay, error) {
	var lessons []model.GroupLesson
	if err := json.Unmarshal([]byte(data), &lessons); err != nil {
		return nil, err
	}
	return &model.GroupDay{
		Day:     dayIndexToString(dayIndex),
		Lessons: lessons,
	}, nil
}

func parseTeacherDay(dayIndex int64, data string) (*model.TeacherDay, error) {
	var lessons []model.TeacherLesson
	if err := json.Unmarshal([]byte(data), &lessons); err != nil {
		return nil, err
	}
	return &model.TeacherDay{
		Day:     dayIndexToString(dayIndex),
		Lessons: lessons,
	}, nil
}

func extractDayData(v interface{}) (int64, string) {
	switch d := v.(type) {
	case *model.GroupDay:
		data, _ := json.Marshal(d.Lessons)
		return dateStringToIndex(d.Day), string(data)
	case *model.TeacherDay:
		data, _ := json.Marshal(d.Lessons)
		return dateStringToIndex(d.Day), string(data)
	}
	return 0, "[]"
}

func dayIndexToString(dayIndex int64) string {
	return DayIndexToDate(dayIndex)
}

func dateStringToIndex(date string) int64 {
	return DateToDayIndex(date)
}
