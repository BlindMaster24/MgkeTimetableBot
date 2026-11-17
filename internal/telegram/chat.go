package telegram

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/sqlitedb"
)

type ChatMode string

const (
	ModeStudent ChatMode = "student"
	ModeTeacher ChatMode = "teacher"
	ModeParent  ChatMode = "parent"
	ModeGuest   ChatMode = "guest"
)

type Chat struct {
	ID                       int64
	Service                  string
	PeerID                   int64
	Accepted                 bool
	EULA                     bool
	Scene                    string
	Mode                     ChatMode
	Group                    string
	Teacher                  string
	GoogleEmail              string
	Formatter                int
	ShowAbout                bool
	ShowDaily                bool
	ShowWeekly               bool
	ShowCalls                bool
	ShowFastGroup            bool
	ShowFastTeacher          bool
	HidePastDays             bool
	DeleteLastMsg            bool
	DeleteUserMsg            bool
	LastMsgID                int64
	AllowSendMess            bool
	NoticeChanges            bool
	NoticeNextWeek           bool
	NoticeCalls              bool
	NoticeParserErrors       bool
	ReminderLeadMinutes      int
	ShowParserTime           bool
	ShowHints                bool
	DiffEnabled              bool
	DiffAutoInWeek           bool
	DiffAutoInUpdates        bool
	DiffShowBeforeAfter      bool
	DiffMaxLines             int
	LastMsgTime              int64
	SubscribeDistribution    bool
	NeedUpdateButtons        bool
	DeactivateSecondaryCheck bool
	CallsEditInput           string
	CallsEditReason          string
	TextFormat               string
	CallsCampus              string
	Ref                      string
	HistoryGroup             []string
	HistoryTeacher           []string
}

type Repository struct {
	db              *sql.DB
	path            string
	mu              sync.RWMutex
	defaultAccepted bool
}

func New(dbPath string) (*Repository, error) {
	return NewWithBusyTimeout(dbPath, sqlitedb.DefaultBusyTimeout)
}

func NewWithBusyTimeout(dbPath string, busyTimeout time.Duration) (*Repository, error) {
	db, err := sqlitedb.Open(dbPath, sqlitedb.Options{BusyTimeout: busyTimeout})
	if err != nil {
		return nil, err
	}

	r := &Repository{db: db, path: dbPath, defaultAccepted: true}
	if err := r.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) SetScene(chat *Chat, scene string) {
	chat.Scene = scene
}

func marshalStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func (chat *Chat) AppendGroupHistory(group string) {
	history := []string{group}
	for _, g := range chat.HistoryGroup {
		if g != group {
			history = append(history, g)
		}
	}
	if len(history) > 5 {
		history = history[:5]
	}
	chat.HistoryGroup = history
}

func (chat *Chat) AppendTeacherHistory(teacher string) {
	history := []string{teacher}
	for _, t := range chat.HistoryTeacher {
		if t != teacher {
			history = append(history, t)
		}
	}
	if len(history) > 5 {
		history = history[:5]
	}
	chat.HistoryTeacher = history
}

func (r *Repository) Save(chat *Chat) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	toInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	_, err := r.db.Exec(
		`UPDATE bot_chats SET
			accepted=?, eula=?, scene=?, mode=?, "group"=?, teacher=?,
			google_email=?, formatter=?, show_about=?, show_daily=?, show_weekly=?,
			show_calls=?, show_fast_group=?, show_fast_teacher=?, hide_past_days=?,
			delete_last_msg=?, delete_user_msg=?, last_msg_id=?, allow_send_mess=?, notice_changes=?,
			notice_next_week=?, notice_calls=?, notice_parser_errors=?,
			reminder_lead_minutes=?,
			show_parser_time=?, show_hints=?, diff_enabled=?, diff_auto_in_week=?,
			diff_auto_in_updates=?, diff_show_before_after=?, diff_max_lines=?, ref=?,
			history_group=?, history_teacher=?, last_msg_time=?, subscribe_distribution=?,
			need_update_buttons=?, deactivate_secondary_check=?, calls_edit_input=?, calls_edit_reason=?, calls_campus=?,
			text_format=?
		 WHERE id=?`,
		toInt(chat.Accepted), toInt(chat.EULA), chat.Scene, string(chat.Mode), chat.Group, chat.Teacher,
		chat.GoogleEmail, chat.Formatter, toInt(chat.ShowAbout), toInt(chat.ShowDaily),
		toInt(chat.ShowWeekly), toInt(chat.ShowCalls), toInt(chat.ShowFastGroup),
		toInt(chat.ShowFastTeacher), toInt(chat.HidePastDays), toInt(chat.DeleteLastMsg),
		toInt(chat.DeleteUserMsg), chat.LastMsgID, toInt(chat.AllowSendMess), toInt(chat.NoticeChanges),
		toInt(chat.NoticeNextWeek), toInt(chat.NoticeCalls), toInt(chat.NoticeParserErrors),
		chat.ReminderLeadMinutes,
		toInt(chat.ShowParserTime), toInt(chat.ShowHints), toInt(chat.DiffEnabled),
		toInt(chat.DiffAutoInWeek), toInt(chat.DiffAutoInUpdates), toInt(chat.DiffShowBeforeAfter),
		chat.DiffMaxLines, chat.Ref,
		marshalStringSlice(chat.HistoryGroup), marshalStringSlice(chat.HistoryTeacher),
		chat.LastMsgTime, toInt(chat.SubscribeDistribution), toInt(chat.NeedUpdateButtons),
		toInt(chat.DeactivateSecondaryCheck), chat.CallsEditInput, chat.CallsEditReason, chat.CallsCampus,
		chat.TextFormat, chat.ID,
	)
	return err
}

func NewChatRepo(dbPath string, busyTimeout time.Duration) (*Repository, error) {
	return NewWithBusyTimeout(dbPath, busyTimeout)
}

func (r *Repository) LoadState(key string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var value string
	err := r.db.QueryRow(`SELECT value FROM bot_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (r *Repository) SaveState(key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(
		`INSERT INTO bot_state(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
