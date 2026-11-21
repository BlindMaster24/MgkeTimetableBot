package main

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		PeerID:                   6409603928,
		Accepted:                 true,
		EULA:                     true,
		Ref:                      "ref-1",
		Scene:                    "set_group",
		Mode:                     "student",
		Group:                    "100",
		Teacher:                  "Иванов И.И.",
		GoogleEmail:              "bot@example.com",
		ShowAbout:                true,
		ShowDaily:                true,
		ShowWeekly:               true,
		ShowCalls:                true,
		ShowFastGroup:            true,
		ShowFastTeacher:          true,
		HidePastDays:             true,
		DeleteLastMsg:            true,
		DeleteUserMsg:            true,
		LastMsgID:                4242,
		LastMsgTime:              1795171844000,
		AllowSendMess:            true,
		SubscribeDistribution:    true,
		NeedUpdateButtons:        true,
		DeactivateSecondaryCheck: true,
		NoticeChanges:            true,
		NoticeNextWeek:           true,
		NoticeParserErrors:       true,
		ShowParserTime:           true,
		ShowHints:                true,
		HistoryGroup:             `["71"]`,
		HistoryTeacher:           `["Петров"]`,
		ScheduleFormatter:        2,
	}
	if err := insertChat(db, chat); err != nil {
		t.Fatal(err)
	}

	var (
		service, scene, mode, group, teacher, email, ref                   string
		historyGroup, historyTeacher                                       string
		peerID, formatter, lastMsgID, lastMsgTime                          int64
		accepted, eula, showDaily, allowSend, noticeChanges, deleteUserMsg int
		showFastTeacher, deleteLastMsg, needUpdate, deactivateSecondary    int
	)
	err := db.QueryRow(`SELECT service, peer_id, accepted, eula, scene, mode, "group", teacher,
		google_email, formatter, last_msg_id, last_msg_time, allow_send_mess, notice_changes,
		show_daily, show_fast_teacher, delete_last_msg, delete_user_msg, need_update_buttons,
		deactivate_secondary_check, history_group, history_teacher, ref
		FROM bot_chats WHERE peer_id = ?`, chat.PeerID).Scan(
		&service, &peerID, &accepted, &eula, &scene, &mode, &group, &teacher,
		&email, &formatter, &lastMsgID, &lastMsgTime, &allowSend, &noticeChanges,
		&showDaily, &showFastTeacher, &deleteLastMsg, &deleteUserMsg, &needUpdate,
		&deactivateSecondary, &historyGroup, &historyTeacher, &ref,
	)
	if err != nil {
		t.Fatal(err)
	}

	if service != "telegram" {
		t.Errorf("service = %q, the SQLite schema expects telegram", service)
	}
	if peerID != chat.PeerID {
		t.Errorf("peer_id = %d, want the Telegram id %d, not the surrogate row id", peerID, chat.PeerID)
	}
	for name, got := range map[string]string{"scene": scene, "mode": mode, "group": group, "teacher": teacher, "email": email, "ref": ref} {
		if got == "" {
			t.Errorf("%s was not copied", name)
		}
	}
	if accepted != 1 || eula != 1 || showDaily != 1 || allowSend != 1 || noticeChanges != 1 ||
		showFastTeacher != 1 || deleteLastMsg != 1 || deleteUserMsg != 1 || needUpdate != 1 ||
		deactivateSecondary != 1 {
		t.Error("booleans must land as 1")
	}
	if formatter != 2 {
		t.Errorf("formatter = %d, want 2", formatter)
	}
	if lastMsgID != 4242 || lastMsgTime != 1795171844000 {
		t.Errorf("message bookkeeping = %d / %d", lastMsgID, lastMsgTime)
	}
	if historyGroup != `["71"]` || historyTeacher != `["Петров"]` {
		t.Errorf("history = %q / %q", historyGroup, historyTeacher)
	}
}

func TestInsertChatReplacesTheSamePeer(t *testing.T) {
	db := openTestDB(t)

	first := OldChat{PeerID: 77, Accepted: true, Group: "100"}
	if err := insertChat(db, first); err != nil {
		t.Fatal(err)
	}

	second := OldChat{PeerID: 77, Accepted: true, Group: "200"}
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
	if err := db.QueryRow(`SELECT "group" FROM bot_chats WHERE peer_id = 77`).Scan(&group); err != nil {
		t.Fatal(err)
	}
	if group != "200" {
		t.Fatalf("the second run must win, got %q", group)
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

func TestRunDemandsAConnectionString(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	err := run(nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "usage: migrate-pg") {
		t.Fatalf("run without a source must explain the usage, got %v", err)
	}
}

func TestRunFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://127.0.0.1:1/mgkt?sslmode=disable&connect_timeout=1")
	target := filepath.Join(t.TempDir(), "chats.db")
	err := run([]string{"-sqlite", target}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "ping postgres") {
		t.Fatalf("the environment must supply the source, got %v", err)
	}
}

func TestRunRefusesToOverwriteWithoutForce(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://127.0.0.1:1/mgkt?sslmode=disable&connect_timeout=1")
	target := filepath.Join(t.TempDir(), "chats.db")
	if err := os.WriteFile(target, []byte("already here"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"-pg", "postgresql://127.0.0.1:1/mgkt?sslmode=disable&connect_timeout=1", "-sqlite", target}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already exists, pass -force") {
		t.Fatalf("an existing target without -force must stop before the connection, got %v", err)
	}
	body, readErr := os.ReadFile(target)
	if readErr != nil || string(body) != "already here" {
		t.Fatalf("the existing database must stay untouched, got %q (%v)", body, readErr)
	}
}

func TestWriteChatsReportsTheInsertedRows(t *testing.T) {
	db := openTestDB(t)
	if err := createChatsSchema(db); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "chats.db")
	if err := writeChats([]OldChat{{PeerID: 6409603928, Mode: "student", Group: "167", Accepted: true, EULA: true}}, path); err != nil {
		t.Fatalf("writeChats: %v", err)
	}
	check, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var peer int64
	if err := check.QueryRow(`SELECT peer_id FROM bot_chats`).Scan(&peer); err != nil {
		t.Fatal(err)
	}
	if peer != 6409603928 {
		t.Fatalf("peer id = %d, want 6409603928", peer)
	}
}

func TestPrepareOutputReplacesOnlyWithForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chats.db")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := prepareOutput(path, true); err != nil {
		t.Fatalf("prepareOutput with -force: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("-force must remove the previous file")
	}
	if err := prepareOutput(path, false); err != nil {
		t.Fatalf("a missing target needs no force: %v", err)
	}
}

func TestReadChatsReportsABrokenConnectionString(t *testing.T) {
	if _, err := readChats("postgres://[::1"); err == nil {
		t.Fatal("a malformed connection string must be reported")
	}
}

func TestWriteChatsReportsAnUnwritableTarget(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := writeChats([]OldChat{{PeerID: 1}}, filepath.Join(blocker, "chats.db"))
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("an unwritable target must be reported, got %v", err)
	}
}

func TestRunContinuesPastTheForceCheck(t *testing.T) {
	target := filepath.Join(t.TempDir(), "chats.db")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"-pg", "postgresql://127.0.0.1:1/mgkt?sslmode=disable&connect_timeout=1", "-sqlite", target, "-force"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "ping postgres") {
		t.Fatalf("with -force the run must go on to the connection, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("-force removed the old file before the failed connection")
	}
}
