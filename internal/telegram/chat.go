package telegram

import (
	"database/sql"
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
	ID                    int64
	Service               string
	PeerID                int64
	Accepted              bool
	Scene                 string
	Mode                  ChatMode
	Group                 string
	Teacher               string
	GoogleEmail           string
	Formatter             int
	ShowAbout             bool
	ShowDaily             bool
	ShowWeekly            bool
	ShowCalls             bool
	ShowFastGroup         bool
	ShowFastTeacher       bool
	HidePastDays          bool
	DeleteLastMsg         bool
	LastMsgID             int64
	AllowSendMess         bool
	NoticeChanges         bool
	NoticeNextWeek        bool
	NoticeCalls           bool
	NoticeParserErrors    bool
	ShowParserTime        bool
	ShowHints             bool
	DiffEnabled           bool
	DiffAutoInWeek        bool
	DiffAutoInUpdates     bool
	DiffShowBeforeAfter   bool
	DiffMaxLines          int
	Ref                   string
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
		show_about INTEGER NOT NULL DEFAULT 0,
		show_daily INTEGER NOT NULL DEFAULT 0,
		show_weekly INTEGER NOT NULL DEFAULT 0,
		show_calls INTEGER NOT NULL DEFAULT 0,
		show_fast_group INTEGER NOT NULL DEFAULT 0,
		show_fast_teacher INTEGER NOT NULL DEFAULT 0,
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
		ref TEXT,
		UNIQUE(service, peer_id)
	)`)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chats_peer ON bot_chats(service, peer_id)`)
	return err
}

func (r *Repository) FindOrCreate(service string, peerID int64) (*Chat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	chat, err := r.findByPeerID(service, peerID)
	if err == nil && chat != nil {
		return chat, nil
	}

	_, err = r.db.Exec(
		`INSERT OR IGNORE INTO bot_chats (service, peer_id) VALUES (?, ?)`,
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
		        diff_auto_in_updates, diff_show_before_after, diff_max_lines, ref
		 FROM bot_chats WHERE service = ? AND peer_id = ?`,
		service, peerID,
	)

	chat := &Chat{}
	var accepted, showAbout, showDaily, showWeekly, showCalls, showFastGroup, showFastTeacher int
	var hidePastDays, deleteLastMsg, allowSendMess, noticeChanges, noticeNextWeek, noticeCalls, noticeParserErrors int
	var showParserTime, showHints, diffEnabled, diffAutoInWeek, diffAutoInUpdates, diffShowBeforeAfter int

	var nsScene, nsMode, nsGroup, nsTeacher, nsGoogleEmail, nsRef sql.NullString
	err := row.Scan(
		&chat.ID, &chat.Service, &chat.PeerID, &accepted, &nsScene, &nsMode,
		&nsGroup, &nsTeacher, &nsGoogleEmail, &chat.Formatter,
		&showAbout, &showDaily, &showWeekly, &showCalls, &showFastGroup, &showFastTeacher,
		&hidePastDays, &deleteLastMsg, &chat.LastMsgID, &allowSendMess, &noticeChanges,
		&noticeNextWeek, &noticeCalls, &noticeParserErrors, &showParserTime, &showHints,
		&diffEnabled, &diffAutoInWeek, &diffAutoInUpdates, &diffShowBeforeAfter,
		&chat.DiffMaxLines, &nsRef,
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

	return chat, nil
}

func (r *Repository) SetScene(chat *Chat, scene string) {
	chat.Scene = scene
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
			diff_auto_in_updates=?, diff_show_before_after=?, diff_max_lines=?, ref=?
		 WHERE id=?`,
		toInt(chat.Accepted), chat.Scene, string(chat.Mode), chat.Group, chat.Teacher,
		chat.GoogleEmail, chat.Formatter, toInt(chat.ShowAbout), toInt(chat.ShowDaily),
		toInt(chat.ShowWeekly), toInt(chat.ShowCalls), toInt(chat.ShowFastGroup),
		toInt(chat.ShowFastTeacher), toInt(chat.HidePastDays), toInt(chat.DeleteLastMsg),
		chat.LastMsgID, toInt(chat.AllowSendMess), toInt(chat.NoticeChanges),
		toInt(chat.NoticeNextWeek), toInt(chat.NoticeCalls), toInt(chat.NoticeParserErrors),
		toInt(chat.ShowParserTime), toInt(chat.ShowHints), toInt(chat.DiffEnabled),
		toInt(chat.DiffAutoInWeek), toInt(chat.DiffAutoInUpdates), toInt(chat.DiffShowBeforeAfter),
		chat.DiffMaxLines, chat.Ref, chat.ID,
	)
	return err
}

func (r *Repository) FindAllWithNotifications(service string) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, service, peer_id, mode, "group", teacher, notice_changes, notice_next_week, notice_calls
		 FROM bot_chats WHERE service = ? AND accepted = 1 AND mode IS NOT NULL AND allow_send_mess = 1`,
		service,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Chat
	for rows.Next() {
		chat := &Chat{}
		var noticeChanges, noticeNextWeek, noticeCalls int
		err := rows.Scan(&chat.ID, &chat.Service, &chat.PeerID, &chat.Mode, &chat.Group, &chat.Teacher, &noticeChanges, &noticeNextWeek, &noticeCalls)
		if err != nil {
			continue
		}
		chat.NoticeChanges = noticeChanges != 0
		chat.NoticeNextWeek = noticeNextWeek != 0
		chat.NoticeCalls = noticeCalls != 0
		result = append(result, chat)
	}
	return result, nil
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) FindGroupsForNotification(service string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT DISTINCT "group" FROM bot_chats WHERE service = ? AND mode IN ('student', 'parent') AND "group" IS NOT NULL AND notice_changes = 1 AND allow_send_mess = 1`,
		service,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if rows.Scan(&g) == nil {
			groups = append(groups, g)
		}
	}
	return groups
}

func (r *Repository) FindTeachersForNotification(service string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT DISTINCT teacher FROM bot_chats WHERE service = ? AND mode = 'teacher' AND teacher IS NOT NULL AND notice_changes = 1 AND allow_send_mess = 1`,
		service,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var teachers []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			teachers = append(teachers, t)
		}
	}
	return teachers
}

func (r *Repository) FindByGroup(service string, group string) []*Chat {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, peer_id, mode, "group" FROM bot_chats WHERE service = ? AND "group" = ? AND accepted = 1 AND allow_send_mess = 1`,
		service, group,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*Chat
	for rows.Next() {
		chat := &Chat{}
		if rows.Scan(&chat.ID, &chat.PeerID, &chat.Mode, &chat.Group) == nil {
			result = append(result, chat)
		}
	}
	return result
}

func (r *Repository) FindByTeacher(service string, teacher string) []*Chat {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, peer_id, mode, teacher FROM bot_chats WHERE service = ? AND teacher = ? AND accepted = 1 AND allow_send_mess = 1`,
		service, teacher,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*Chat
	for rows.Next() {
		chat := &Chat{}
		if rows.Scan(&chat.ID, &chat.PeerID, &chat.Mode, &chat.Teacher) == nil {
			result = append(result, chat)
		}
	}
	return result
}

func NewChatRepo(dbPath string) (*Repository, error) {
	return New(dbPath)
}

func (r *Repository) FindAllTGChats() ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, peer_id, mode, "group", teacher, allow_send_mess, notice_changes
		 FROM bot_chats WHERE service = 'telegram' AND accepted = 1 AND allow_send_mess = 1`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Chat
	for rows.Next() {
		chat := &Chat{}
		var allowSend, noticeChanges int
		if err := rows.Scan(&chat.ID, &chat.PeerID, &chat.Mode, &chat.Group, &chat.Teacher, &allowSend, &noticeChanges); err != nil {
			continue
		}
		chat.AllowSendMess = allowSend != 0
		chat.NoticeChanges = noticeChanges != 0
		result = append(result, chat)
	}
	return result, nil
}

func (r *Repository) CountAll() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM bot_chats WHERE service = 'telegram' AND accepted = 1`).Scan(&count)
	return count, err
}

func (r *Repository) CountByMode() (map[string]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT COALESCE(mode, 'none'), COUNT(*) FROM bot_chats WHERE service = 'telegram' AND accepted = 1 GROUP BY mode`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var mode string
		var count int
		if rows.Scan(&mode, &count) == nil {
			result[mode] = count
		}
	}
	return result, err
}

type Subscription struct {
	ID    int64
	ChatID int64
	Type  string
	Value string
}

func (r *Repository) migrateSubscriptions() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		UNIQUE(chat_id, type, value)
	)`)
	return err
}

func (r *Repository) AddSubscription(chatID int64, subType, value string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.Exec(
		`INSERT OR IGNORE INTO subscriptions (chat_id, type, value) VALUES (?, ?, ?)`,
		chatID, subType, value,
	)
	if err != nil {
		return false, err
	}

	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (r *Repository) RemoveSubscription(chatID int64, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`DELETE FROM subscriptions WHERE id = ? AND chat_id = ?`, id, chatID)
	return err
}

func (r *Repository) GetSubscriptions(chatID int64) ([]Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, chat_id, type, value FROM subscriptions WHERE chat_id = ? ORDER BY id ASC`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.ChatID, &s.Type, &s.Value); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (r *Repository) FindSubscriptionsForGroup(group string) ([]Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, chat_id, type, value FROM subscriptions WHERE type = 'group' AND value = ?`,
		group,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.ChatID, &s.Type, &s.Value); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (r *Repository) FindSubscriptionsForTeacher(teacher string) ([]Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, chat_id, type, value FROM subscriptions WHERE type = 'teacher' AND value = ?`,
		teacher,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.ChatID, &s.Type, &s.Value); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (r *Repository) CountSubscriptions(chatID int64) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE chat_id = ?`, chatID).Scan(&count)
	return count, err
}
