package telegram

import (
	"database/sql"
	"encoding/json"
	"sync"

	_ "modernc.org/sqlite"
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
	LastMsgID                int64
	AllowSendMess            bool
	NoticeChanges            bool
	NoticeNextWeek           bool
	NoticeCalls              bool
	NoticeParserErrors       bool
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
	CallsCampus              string
	Ref                      string
	HistoryGroup             []string
	HistoryTeacher           []string
}

type Repository struct {
	db *sql.DB
	mu sync.RWMutex
}

func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	r := &Repository{db: db}
	if err := r.migrate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS bot_chats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service TEXT NOT NULL DEFAULT 'telegram',
		peer_id INTEGER NOT NULL,
		accepted INTEGER NOT NULL DEFAULT 1,
		scene TEXT,
		mode TEXT,
		"group" TEXT,
		teacher TEXT,
		google_email TEXT,
		formatter INTEGER NOT NULL DEFAULT 0,
		show_about INTEGER NOT NULL DEFAULT 1,
		show_daily INTEGER NOT NULL DEFAULT 1,
		show_weekly INTEGER NOT NULL DEFAULT 1,
		show_calls INTEGER NOT NULL DEFAULT 1,
		show_fast_group INTEGER NOT NULL DEFAULT 1,
		show_fast_teacher INTEGER NOT NULL DEFAULT 1,
		hide_past_days INTEGER NOT NULL DEFAULT 0,
		delete_last_msg INTEGER NOT NULL DEFAULT 0,
		last_msg_id INTEGER NOT NULL DEFAULT 0,
		allow_send_mess INTEGER NOT NULL DEFAULT 1,
		notice_changes INTEGER NOT NULL DEFAULT 1,
		notice_next_week INTEGER NOT NULL DEFAULT 1,
		notice_calls INTEGER NOT NULL DEFAULT 1,
		notice_parser_errors INTEGER NOT NULL DEFAULT 1,
		notice_week INTEGER NOT NULL DEFAULT 1,
		show_parser_time INTEGER NOT NULL DEFAULT 0,
		show_hints INTEGER NOT NULL DEFAULT 1,
		diff_enabled INTEGER NOT NULL DEFAULT 1,
		diff_auto_in_week INTEGER NOT NULL DEFAULT 1,
		diff_auto_in_updates INTEGER NOT NULL DEFAULT 1,
		diff_show_before_after INTEGER NOT NULL DEFAULT 1,
		diff_max_lines INTEGER NOT NULL DEFAULT 20,
		last_msg_time INTEGER NOT NULL DEFAULT 0,
		subscribe_distribution INTEGER NOT NULL DEFAULT 1,
		need_update_buttons INTEGER NOT NULL DEFAULT 0,
		deactivate_secondary_check INTEGER NOT NULL DEFAULT 0,
		calls_edit_input TEXT,
		calls_edit_reason TEXT,
		calls_campus TEXT,
		ref TEXT,
		history_group TEXT NOT NULL DEFAULT '[]',
		history_teacher TEXT NOT NULL DEFAULT '[]',
		UNIQUE(service, peer_id)
	)`)
	if err != nil {
		return err
	}

	for _, col := range []struct{ name, ddl string }{
		{"history_group", "ALTER TABLE bot_chats ADD COLUMN history_group TEXT NOT NULL DEFAULT '[]'"},
		{"history_teacher", "ALTER TABLE bot_chats ADD COLUMN history_teacher TEXT NOT NULL DEFAULT '[]'"},
		{"last_msg_time", "ALTER TABLE bot_chats ADD COLUMN last_msg_time INTEGER NOT NULL DEFAULT 0"},
		{"subscribe_distribution", "ALTER TABLE bot_chats ADD COLUMN subscribe_distribution INTEGER NOT NULL DEFAULT 1"},
		{"need_update_buttons", "ALTER TABLE bot_chats ADD COLUMN need_update_buttons INTEGER NOT NULL DEFAULT 0"},
		{"deactivate_secondary_check", "ALTER TABLE bot_chats ADD COLUMN deactivate_secondary_check INTEGER NOT NULL DEFAULT 0"},
		{"calls_edit_input", "ALTER TABLE bot_chats ADD COLUMN calls_edit_input TEXT"},
		{"calls_edit_reason", "ALTER TABLE bot_chats ADD COLUMN calls_edit_reason TEXT"},
		{"calls_campus", "ALTER TABLE bot_chats ADD COLUMN calls_campus TEXT"},
	} {
		var count int
		r.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('bot_chats') WHERE name = ?`, col.name).Scan(&count)
		if count == 0 {
			if _, err := r.db.Exec(col.ddl); err != nil {
				return err
			}
		}
	}

	_, err = r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chats_peer ON bot_chats(service, peer_id)`)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`CREATE TABLE IF NOT EXISTS bot_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	if err := r.migrateShowButtons(); err != nil {
		return err
	}

	if err := r.migrateSubscriptions(); err != nil {
		return err
	}

	return r.migrateGoogle()
}

func (r *Repository) FindOrCreate(service string, peerID int64) (*Chat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	chat, err := r.findByPeerID(service, peerID)
	if err == nil && chat != nil {
		return chat, nil
	}

	_, err = r.db.Exec(
		`INSERT OR IGNORE INTO bot_chats (
			service, peer_id, show_about, show_daily, show_weekly,
			show_calls, show_fast_group, show_fast_teacher
		) VALUES (?, ?, 1, 1, 1, 1, 1, 1)`,
		service, peerID,
	)
	if err != nil {
		return nil, err
	}

	return r.findByPeerID(service, peerID)
}

func (r *Repository) findByPeerID(service string, peerID int64) (*Chat, error) {
	row := r.db.QueryRow(
		`SELECT id, service, peer_id, accepted, scene, mode, "group", teacher,
		        google_email, formatter, show_about, show_daily, show_weekly,
		        show_calls, show_fast_group, show_fast_teacher, hide_past_days,
		        delete_last_msg, last_msg_id, allow_send_mess, notice_changes,
		        notice_next_week, notice_calls, notice_parser_errors,
		        show_parser_time, show_hints, diff_enabled, diff_auto_in_week,
		        diff_auto_in_updates, diff_show_before_after, diff_max_lines, ref,
		        history_group, history_teacher,
		        last_msg_time, subscribe_distribution, need_update_buttons, deactivate_secondary_check,
		        calls_edit_input, calls_edit_reason, calls_campus
		 FROM bot_chats WHERE service = ? AND peer_id = ?`,
		service, peerID,
	)

	chat := &Chat{}
	var accepted, showAbout, showDaily, showWeekly, showCalls, showFastGroup, showFastTeacher int
	var hidePastDays, deleteLastMsg, allowSendMess, noticeChanges, noticeNextWeek, noticeCalls, noticeParserErrors int
	var showParserTime, showHints, diffEnabled, diffAutoInWeek, diffAutoInUpdates, diffShowBeforeAfter int
	var subscribeDistribution, needUpdateButtons, deactivateSecondaryCheck int

	var nsScene, nsMode, nsGroup, nsTeacher, nsGoogleEmail, nsRef sql.NullString
	var nsHistoryGroup, nsHistoryTeacher sql.NullString
	var nsCallsEditInput, nsCallsEditReason, nsCallsCampus sql.NullString
	err := row.Scan(
		&chat.ID, &chat.Service, &chat.PeerID, &accepted, &nsScene, &nsMode,
		&nsGroup, &nsTeacher, &nsGoogleEmail, &chat.Formatter,
		&showAbout, &showDaily, &showWeekly, &showCalls, &showFastGroup, &showFastTeacher,
		&hidePastDays, &deleteLastMsg, &chat.LastMsgID, &allowSendMess, &noticeChanges,
		&noticeNextWeek, &noticeCalls, &noticeParserErrors, &showParserTime, &showHints,
		&diffEnabled, &diffAutoInWeek, &diffAutoInUpdates, &diffShowBeforeAfter,
		&chat.DiffMaxLines, &nsRef, &nsHistoryGroup, &nsHistoryTeacher,
		&chat.LastMsgTime, &subscribeDistribution, &needUpdateButtons, &deactivateSecondaryCheck,
		&nsCallsEditInput, &nsCallsEditReason, &nsCallsCampus,
	)
	if err != nil {
		return nil, err
	}
	chat.Scene = nsScene.String
	chat.Mode = ChatMode(nsMode.String)
	chat.Group = nsGroup.String
	chat.Teacher = nsTeacher.String
	chat.GoogleEmail = nsGoogleEmail.String
	chat.Ref = nsRef.String
	_ = json.Unmarshal([]byte(nsHistoryGroup.String), &chat.HistoryGroup)
	_ = json.Unmarshal([]byte(nsHistoryTeacher.String), &chat.HistoryTeacher)

	chat.Accepted = accepted != 0
	chat.ShowAbout = showAbout != 0
	chat.ShowDaily = showDaily != 0
	chat.ShowWeekly = showWeekly != 0
	chat.ShowCalls = showCalls != 0
	chat.ShowFastGroup = showFastGroup != 0
	chat.ShowFastTeacher = showFastTeacher != 0
	chat.HidePastDays = hidePastDays != 0
	chat.DeleteLastMsg = deleteLastMsg != 0
	chat.AllowSendMess = allowSendMess != 0
	chat.NoticeChanges = noticeChanges != 0
	chat.NoticeNextWeek = noticeNextWeek != 0
	chat.NoticeCalls = noticeCalls != 0
	chat.NoticeParserErrors = noticeParserErrors != 0
	chat.ShowParserTime = showParserTime != 0
	chat.ShowHints = showHints != 0
	chat.DiffEnabled = diffEnabled != 0
	chat.DiffAutoInWeek = diffAutoInWeek != 0
	chat.DiffAutoInUpdates = diffAutoInUpdates != 0
	chat.DiffShowBeforeAfter = diffShowBeforeAfter != 0
	chat.SubscribeDistribution = subscribeDistribution != 0
	chat.NeedUpdateButtons = needUpdateButtons != 0
	chat.DeactivateSecondaryCheck = deactivateSecondaryCheck != 0
	chat.CallsEditInput = nsCallsEditInput.String
	chat.CallsEditReason = nsCallsEditReason.String
	chat.CallsCampus = nsCallsCampus.String

	return chat, nil
}

func (r *Repository) migrateShowButtons() error {
	const key = "migrate.show_buttons_default"

	var stored string
	err := r.db.QueryRow(`SELECT value FROM bot_state WHERE key = ?`, key).Scan(&stored)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	_, err = r.db.Exec(`UPDATE bot_chats SET
			show_about = 1, show_daily = 1, show_weekly = 1,
			show_calls = 1, show_fast_group = 1, show_fast_teacher = 1
		WHERE show_about = 0 AND show_daily = 0 AND show_weekly = 0
			AND show_calls = 0 AND show_fast_group = 0 AND show_fast_teacher = 0`)
	if err != nil {
		return err
	}

	return r.SaveState(key, "1")
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
			accepted=?, scene=?, mode=?, "group"=?, teacher=?,
			google_email=?, formatter=?, show_about=?, show_daily=?, show_weekly=?,
			show_calls=?, show_fast_group=?, show_fast_teacher=?, hide_past_days=?,
			delete_last_msg=?, last_msg_id=?, allow_send_mess=?, notice_changes=?,
			notice_next_week=?, notice_calls=?, notice_parser_errors=?,
			show_parser_time=?, show_hints=?, diff_enabled=?, diff_auto_in_week=?,
			diff_auto_in_updates=?, diff_show_before_after=?, diff_max_lines=?, ref=?,
			history_group=?, history_teacher=?, last_msg_time=?, subscribe_distribution=?,
			need_update_buttons=?, deactivate_secondary_check=?, calls_edit_input=?, calls_edit_reason=?, calls_campus=?
		 WHERE id=?`,
		toInt(chat.Accepted), chat.Scene, string(chat.Mode), chat.Group, chat.Teacher,
		chat.GoogleEmail, chat.Formatter, toInt(chat.ShowAbout), toInt(chat.ShowDaily),
		toInt(chat.ShowWeekly), toInt(chat.ShowCalls), toInt(chat.ShowFastGroup),
		toInt(chat.ShowFastTeacher), toInt(chat.HidePastDays), toInt(chat.DeleteLastMsg),
		chat.LastMsgID, toInt(chat.AllowSendMess), toInt(chat.NoticeChanges),
		toInt(chat.NoticeNextWeek), toInt(chat.NoticeCalls), toInt(chat.NoticeParserErrors),
		toInt(chat.ShowParserTime), toInt(chat.ShowHints), toInt(chat.DiffEnabled),
		toInt(chat.DiffAutoInWeek), toInt(chat.DiffAutoInUpdates), toInt(chat.DiffShowBeforeAfter),
		chat.DiffMaxLines, chat.Ref,
		marshalStringSlice(chat.HistoryGroup), marshalStringSlice(chat.HistoryTeacher),
		chat.LastMsgTime, toInt(chat.SubscribeDistribution), toInt(chat.NeedUpdateButtons),
		toInt(chat.DeactivateSecondaryCheck), chat.CallsEditInput, chat.CallsEditReason, chat.CallsCampus, chat.ID,
	)
	return err
}

func NewChatRepo(dbPath string) (*Repository, error) {
	return New(dbPath)
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
