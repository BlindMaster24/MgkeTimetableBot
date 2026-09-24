package telegram

import (
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) FindAllWithNotifications(service string) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, service, peer_id, COALESCE(mode, ''), COALESCE("group", ''), COALESCE(teacher, ''),
		 COALESCE(notice_changes, 1), COALESCE(notice_next_week, 1), COALESCE(notice_calls, 1)
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
		var mode string
		var noticeChanges, noticeNextWeek, noticeCalls int
		err := rows.Scan(&chat.ID, &chat.Service, &chat.PeerID, &mode, &chat.Group, &chat.Teacher, &noticeChanges, &noticeNextWeek, &noticeCalls)
		if err != nil {
			continue
		}
		chat.Mode = ChatMode(mode)
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

func (r *Repository) CountTGChats() (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total, allowed int
	err := r.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(allow_send_mess), 0) FROM bot_chats WHERE service = 'telegram' AND accepted = 1`,
	).Scan(&total, &allowed)
	return total, allowed, err
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

func (r *Repository) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(groups) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(groups))
	args := make([]any, 0, len(groups)+1)
	args = append(args, service)
	for i, g := range groups {
		placeholders[i] = "?"
		args = append(args, g)
	}

	query := `SELECT ` + selectEventChats("") + ` FROM bot_chats
		 WHERE service = ? AND "group" IN (` + strings.Join(placeholders, ",") + `)
		 AND accepted = 1 AND allow_send_mess = 1
		 AND (mode IN ('student', 'parent') OR mode IS NULL OR mode = '')`
	query += noticeChangesFilter("", noticeChanges)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}

func (r *Repository) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(teachers) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(teachers))
	args := make([]any, 0, len(teachers)+1)
	args = append(args, service)
	for i, t := range teachers {
		placeholders[i] = "?"
		args = append(args, t)
	}

	query := `SELECT ` + selectEventChats("") + ` FROM bot_chats
		 WHERE service = ? AND teacher IN (` + strings.Join(placeholders, ",") + `)
		 AND accepted = 1 AND allow_send_mess = 1
		 AND (mode = 'teacher' OR mode IS NULL OR mode = '')`
	query += noticeChangesFilter("", noticeChanges)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}

var eventChatColumns = []string{
	"%sid", "%speer_id",
	"COALESCE(%smode, '')", "COALESCE(%s\"group\", '')", "COALESCE(%steacher, '')",
	"COALESCE(%sallow_send_mess, 1)", "COALESCE(%snotice_changes, 1)", "COALESCE(%snotice_next_week, 1)",
	"COALESCE(%snotice_calls, 1)", "COALESCE(%snotice_parser_errors, 1)", "COALESCE(%sformatter, 0)",
	"COALESCE(%shide_past_days, 0)", "COALESCE(%sshow_hints, 1)", "COALESCE(%sshow_parser_time, 0)",
}

func selectEventChats(prefix string) string {
	columns := make([]string, 0, len(eventChatColumns))
	for _, column := range eventChatColumns {
		columns = append(columns, fmt.Sprintf(column, prefix))
	}
	return strings.Join(columns, ", ")
}

func noticeChangesFilter(prefix string, noticeChanges bool) string {
	if noticeChanges {
		return " AND " + prefix + "notice_changes = 1"
	}
	return ""
}

func scanEventChats(rows *sql.Rows) ([]*Chat, error) {
	var result []*Chat
	for rows.Next() {
		var (
			chat       Chat
			mode       string
			allowSend  int
			noticeCh   int
			noticeWeek int
			noticeCall int
			noticeErr  int
			hidePast   int
			hints      int
			parserTime int
		)
		err := rows.Scan(&chat.ID, &chat.PeerID, &mode, &chat.Group, &chat.Teacher,
			&allowSend, &noticeCh, &noticeWeek, &noticeCall, &noticeErr, &chat.Formatter,
			&hidePast, &hints, &parserTime)
		if err != nil {
			continue
		}
		chat.Mode = ChatMode(mode)
		chat.AllowSendMess = allowSend != 0
		chat.NoticeChanges = noticeCh != 0
		chat.NoticeNextWeek = noticeWeek != 0
		chat.NoticeCalls = noticeCall != 0
		chat.NoticeParserErrors = noticeErr != 0
		chat.HidePastDays = hidePast != 0
		chat.ShowHints = hints != 0
		chat.ShowParserTime = parserTime != 0
		result = append(result, &chat)
	}
	return result, nil
}

func (r *Repository) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `SELECT ` + selectEventChats("c.") + ` FROM bot_chats c
		 JOIN subscriptions s ON s.chat_id = c.id AND s.type = 'group' AND s.value = ?
		 WHERE c.service = ? AND c.accepted = 1 AND c.allow_send_mess = 1`
	query += noticeChangesFilter("c.", noticeChanges)

	rows, err := r.db.Query(query, group, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}

func (r *Repository) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `SELECT ` + selectEventChats("c.") + ` FROM bot_chats c
		 JOIN subscriptions s ON s.chat_id = c.id AND s.type = 'teacher' AND s.value = ?
		 WHERE c.service = ? AND c.accepted = 1 AND c.allow_send_mess = 1`
	query += noticeChangesFilter("c.", noticeChanges)

	rows, err := r.db.Query(query, teacher, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}

func (r *Repository) FindChatsWithNotice(service, notice string) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	column := "notice_calls"
	switch notice {
	case "notice_calls":
		column = "notice_calls"
	case "notice_parser_errors":
		column = "notice_parser_errors"
	case "notice_next_week":
		column = "notice_next_week"
	}

	rows, err := r.db.Query(
		`SELECT `+selectEventChats("")+` FROM bot_chats
		 WHERE service = ? AND accepted = 1 AND allow_send_mess = 1 AND `+column+` = 1`,
		service,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}

func (r *Repository) FindAdminChats(service string, adminIDs []int64) ([]*Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(adminIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(adminIDs))
	args := make([]any, 0, len(adminIDs)+1)
	args = append(args, service)
	for i, id := range adminIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	rows, err := r.db.Query(
		`SELECT `+selectEventChats("")+` FROM bot_chats
		 WHERE service = ? AND peer_id IN (`+strings.Join(placeholders, ",")+`)
		 AND accepted = 1 AND allow_send_mess = 1`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventChats(rows)
}
