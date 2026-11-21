package telegram

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/sqlitedb"
)

const (
	chatApplicationID = 0x4D474B43
	chatSchemaVersion = 1
)

var chatTables = []string{
	"bot_chats",
	"bot_state",
	"aliases",
	"subscriptions",
	"google_accounts",
	"google_calendars",
	"api_keys",
}

func (r *Repository) migrate() error {
	if err := sqlitedb.CheckIdentity(r.db, r.path, sqlitedb.Identity{
		ApplicationID: chatApplicationID,
		SchemaVersion: chatSchemaVersion,
		Tables:        chatTables,
	}); err != nil {
		return err
	}

	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS bot_chats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service TEXT NOT NULL DEFAULT 'telegram',
		peer_id INTEGER NOT NULL,
		accepted INTEGER NOT NULL DEFAULT 1,
		eula INTEGER NOT NULL DEFAULT 0,
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
		show_gcal INTEGER NOT NULL DEFAULT 1,
		show_ics INTEGER NOT NULL DEFAULT 1,
		hide_past_days INTEGER NOT NULL DEFAULT 1,
		delete_last_msg INTEGER NOT NULL DEFAULT 0,
		delete_user_msg INTEGER NOT NULL DEFAULT 0,
		last_msg_id INTEGER NOT NULL DEFAULT 0,
		allow_send_mess INTEGER NOT NULL DEFAULT 1,
		notice_changes INTEGER NOT NULL DEFAULT 1,
		notice_next_week INTEGER NOT NULL DEFAULT 1,
		notice_calls INTEGER NOT NULL DEFAULT 1,
		notice_parser_errors INTEGER NOT NULL DEFAULT 1,
		reminder_lead_minutes INTEGER NOT NULL DEFAULT 0,
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
		text_format TEXT NOT NULL DEFAULT '',
		UNIQUE(service, peer_id)
	)`)
	if err != nil {
		return err
	}

	for _, col := range []struct{ name, ddl string }{
		{"eula", "ALTER TABLE bot_chats ADD COLUMN eula INTEGER NOT NULL DEFAULT 0"},
		{"delete_user_msg", "ALTER TABLE bot_chats ADD COLUMN delete_user_msg INTEGER NOT NULL DEFAULT 0"},
		{"history_group", "ALTER TABLE bot_chats ADD COLUMN history_group TEXT NOT NULL DEFAULT '[]'"},
		{"history_teacher", "ALTER TABLE bot_chats ADD COLUMN history_teacher TEXT NOT NULL DEFAULT '[]'"},
		{"last_msg_time", "ALTER TABLE bot_chats ADD COLUMN last_msg_time INTEGER NOT NULL DEFAULT 0"},
		{"subscribe_distribution", "ALTER TABLE bot_chats ADD COLUMN subscribe_distribution INTEGER NOT NULL DEFAULT 1"},
		{"need_update_buttons", "ALTER TABLE bot_chats ADD COLUMN need_update_buttons INTEGER NOT NULL DEFAULT 0"},
		{"deactivate_secondary_check", "ALTER TABLE bot_chats ADD COLUMN deactivate_secondary_check INTEGER NOT NULL DEFAULT 0"},
		{"calls_edit_input", "ALTER TABLE bot_chats ADD COLUMN calls_edit_input TEXT"},
		{"calls_edit_reason", "ALTER TABLE bot_chats ADD COLUMN calls_edit_reason TEXT"},
		{"calls_campus", "ALTER TABLE bot_chats ADD COLUMN calls_campus TEXT"},
		{"reminder_lead_minutes", "ALTER TABLE bot_chats ADD COLUMN reminder_lead_minutes INTEGER NOT NULL DEFAULT 0"},
		{"text_format", "ALTER TABLE bot_chats ADD COLUMN text_format TEXT NOT NULL DEFAULT ''"},
		{"show_gcal", "ALTER TABLE bot_chats ADD COLUMN show_gcal INTEGER NOT NULL DEFAULT 1"},
		{"show_ics", "ALTER TABLE bot_chats ADD COLUMN show_ics INTEGER NOT NULL DEFAULT 1"},
	} {
		var count int
		if err := r.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('bot_chats') WHERE name = ?`, col.name).Scan(&count); err != nil {
			return err
		}
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

// TouchLastMessage records the moment the chat last spoke to the bot without
// rewriting the rest of the row, which handlers keep editing meanwhile.
func (r *Repository) TouchLastMessage(service string, peerID int64, moment time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE bot_chats SET last_msg_time = ? WHERE service = ? AND peer_id = ?`,
		moment.UnixMilli(), service, peerID)
	return err
}

// SetTextFormat stores the outgoing message format of the chat: "md" for
// MarkdownV2 (the default) or "html" for the HTML parser.
func (r *Repository) SetTextFormat(service string, peerID int64, format string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE bot_chats SET text_format = ? WHERE service = ? AND peer_id = ?`, format, service, peerID)
	return err
}

func (r *Repository) SetAllowSendMess(service string, peerID int64, allow bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	value := 0
	if allow {
		value = 1
	}

	_, err := r.db.Exec(`UPDATE bot_chats SET allow_send_mess = ? WHERE service = ? AND peer_id = ?`, value, service, peerID)
	return err
}

func (r *Repository) SetDefaultAccepted(accepted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultAccepted = accepted
}

func (r *Repository) FindOrCreate(service string, peerID int64) (*Chat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	chat, err := r.findByPeerID(service, peerID)
	if err == nil && chat != nil {
		return chat, nil
	}

	accepted := 0
	if r.defaultAccepted {
		accepted = 1
	}

	_, err = r.db.Exec(
		// hide_past_days is spelled out because the TypeScript bot hid the
		// passed lessons by default, and old databases still carry DEFAULT 0.
		`INSERT OR IGNORE INTO bot_chats (
			service, peer_id, accepted, show_about, show_daily, show_weekly,
			show_calls, show_fast_group, show_fast_teacher, hide_past_days
		) VALUES (?, ?, ?, 1, 1, 1, 1, 1, 1, 1)`,
		service, peerID, accepted,
	)
	if err != nil {
		return nil, err
	}

	return r.findByPeerID(service, peerID)
}

func (r *Repository) Accept(service string, peerID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.Exec(
		`UPDATE bot_chats SET accepted = 1 WHERE service = ? AND peer_id = ?`,
		service, peerID,
	)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *Repository) findByPeerID(service string, peerID int64) (*Chat, error) {
	row := r.db.QueryRow(
		`SELECT id, service, peer_id, accepted, eula, scene, mode, "group", teacher,
		        google_email, formatter, show_about, show_daily, show_weekly,
		        show_calls, show_fast_group, show_fast_teacher, show_gcal, show_ics, hide_past_days,
		        delete_last_msg, delete_user_msg, last_msg_id, allow_send_mess, notice_changes,
		        notice_next_week, notice_calls, notice_parser_errors,
		        reminder_lead_minutes,
		        show_parser_time, show_hints, diff_enabled, diff_auto_in_week,
		        diff_auto_in_updates, diff_show_before_after, diff_max_lines, ref,
		        history_group, history_teacher,
		        last_msg_time, subscribe_distribution, need_update_buttons, deactivate_secondary_check,
		        calls_edit_input, calls_edit_reason, calls_campus, text_format
		 FROM bot_chats WHERE service = ? AND peer_id = ?`,
		service, peerID,
	)

	chat := &Chat{}
	var accepted, eula, showAbout, showDaily, showWeekly, showCalls, showFastGroup, showFastTeacher int
	var hidePastDays, deleteLastMsg, deleteUserMsg, allowSendMess, noticeChanges, noticeNextWeek, noticeCalls, noticeParserErrors int
	var showParserTime, showHints, diffEnabled, diffAutoInWeek, diffAutoInUpdates, diffShowBeforeAfter int
	var subscribeDistribution, needUpdateButtons, deactivateSecondaryCheck int
	var showGoogleCalendar, showICS int

	var nsScene, nsMode, nsGroup, nsTeacher, nsGoogleEmail, nsRef sql.NullString
	var nsHistoryGroup, nsHistoryTeacher sql.NullString
	var nsCallsEditInput, nsCallsEditReason, nsCallsCampus sql.NullString
	var nsTextFormat sql.NullString
	err := row.Scan(
		&chat.ID, &chat.Service, &chat.PeerID, &accepted, &eula, &nsScene, &nsMode,
		&nsGroup, &nsTeacher, &nsGoogleEmail, &chat.Formatter,
		&showAbout, &showDaily, &showWeekly, &showCalls, &showFastGroup, &showFastTeacher,
		&showGoogleCalendar, &showICS, &hidePastDays, &deleteLastMsg, &deleteUserMsg, &chat.LastMsgID, &allowSendMess, &noticeChanges,
		&noticeNextWeek, &noticeCalls, &noticeParserErrors, &chat.ReminderLeadMinutes, &showParserTime, &showHints,
		&diffEnabled, &diffAutoInWeek, &diffAutoInUpdates, &diffShowBeforeAfter,
		&chat.DiffMaxLines, &nsRef, &nsHistoryGroup, &nsHistoryTeacher,
		&chat.LastMsgTime, &subscribeDistribution, &needUpdateButtons, &deactivateSecondaryCheck,
		&nsCallsEditInput, &nsCallsEditReason, &nsCallsCampus, &nsTextFormat,
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
	chat.EULA = eula != 0
	chat.ShowAbout = showAbout != 0
	chat.ShowDaily = showDaily != 0
	chat.ShowWeekly = showWeekly != 0
	chat.ShowCalls = showCalls != 0
	chat.ShowFastGroup = showFastGroup != 0
	chat.ShowFastTeacher = showFastTeacher != 0
	chat.ShowGoogleCalendar = showGoogleCalendar != 0
	chat.ShowICS = showICS != 0
	chat.HidePastDays = hidePastDays != 0
	chat.DeleteLastMsg = deleteLastMsg != 0
	chat.DeleteUserMsg = deleteUserMsg != 0
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
	chat.TextFormat = nsTextFormat.String

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
