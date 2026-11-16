package main

import (
	"database/sql"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := createChatsSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestInsertChatMapsEveryColumn(t *testing.T) {
	db := openTestDB(t)

	chat := OldChat{
		ID:                  77,
		Service:             "tg",
		Accepted:            true,
		Ref:                 "ref-1",
		Scene:               "set_group",
		Mode:                "student",
		Group:               "100",
		Teacher:             "Иванов И.И.",
		GoogleEmail:         "bot@example.com",
		ShowAbout:           true,
		ShowDaily:           true,
		ShowWeekly:          true,
		ShowCalls:           true,
		ShowFastGroup:       true,
		ShowFastTeacher:     true,
		HidePastDays:        true,
		DeleteLastMsg:       true,
		LastMsgID:           4242,
		AllowSendMess:       true,
		NoticeChanges:       true,
		NoticeNextWeek:      true,
		NoticeCalls:         true,
		NoticeParserErrors:  true,
		ShowParserTime:      true,
		ShowHints:           true,
		DiffEnabled:         true,
		DiffAutoInWeek:      true,
		DiffAutoInUpdates:   true,
		DiffShowBeforeAfter: true,
		DiffMaxLines:        31,
		ScheduleFormatter:   2,
	}
	if err := insertChat(db, chat); err != nil {
		t.Fatal(err)
	}

	var (
		service, scene, mode, group, teacher, email, ref string
		peerID, formatter, lastMsgID, diffMaxLines       int
		accepted, showDaily, allowSend, noticeChanges    int
		showFastTeacher, deleteLastMsg                   int
	)
	err := db.QueryRow(`SELECT service, peer_id, accepted, scene, mode, "group", teacher,
		google_email, formatter, last_msg_id, allow_send_mess, notice_changes,
		show_daily, show_fast_teacher, delete_last_msg, diff_max_lines, ref
		FROM bot_chats WHERE peer_id = ?`, chat.ID).Scan(
		&service, &peerID, &accepted, &scene, &mode, &group, &teacher,
		&email, &formatter, &lastMsgID, &allowSend, &noticeChanges,
		&showDaily, &showFastTeacher, &deleteLastMsg, &diffMaxLines, &ref,
	)
	if err != nil {
		t.Fatal(err)
	}

	if service != "telegram" {
		t.Errorf("service = %q, the SQLite schema expects telegram", service)
	}
	if peerID != chat.ID {
		t.Errorf("peer_id = %d, want %d", peerID, chat.ID)
	}
	for name, got := range map[string]string{"scene": scene, "mode": mode, "group": group, "teacher": teacher, "email": email, "ref": ref} {
		if got == "" {
			t.Errorf("%s was not copied", name)
		}
	}
	if accepted != 1 || showDaily != 1 || allowSend != 1 || noticeChanges != 1 || showFastTeacher != 1 || deleteLastMsg != 1 {
		t.Error("booleans must land as 1")
	}
	if formatter != 2 {
		t.Errorf("formatter = %d, want 2", formatter)
	}
	if lastMsgID != 4242 {
		t.Errorf("last_msg_id = %d, want 4242", lastMsgID)
	}
	if diffMaxLines != 31 {
		t.Errorf("diff_max_lines = %d, want 31", diffMaxLines)
	}
}

func TestInsertChatReplacesTheSamePeer(t *testing.T) {
	db := openTestDB(t)

	first := OldChat{ID: 77, Accepted: true, Group: "100", DiffMaxLines: 20}
	if err := insertChat(db, first); err != nil {
		t.Fatal(err)
	}

	second := OldChat{ID: 77, Accepted: true, Group: "200", DiffMaxLines: 5}
	if err := insertChat(db, second); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bot_chats`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rows = %d, the migration must not duplicate a peer", count)
	}

	var group string
	var maxLines int
	if err := db.QueryRow(`SELECT "group", diff_max_lines FROM bot_chats WHERE peer_id = 77`).Scan(&group, &maxLines); err != nil {
		t.Fatal(err)
	}
	if group != "200" || maxLines != 5 {
		t.Fatalf("the second run must win, got %q / %d", group, maxLines)
	}
}

func TestCreateChatsSchemaIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := createChatsSchema(db); err != nil {
		t.Fatalf("a second schema pass must pass: %v", err)
	}
	if err := createChatsSchema(db); err != nil {
		t.Fatalf("a third schema pass must pass: %v", err)
	}
}
