// Command migrate-pg copies chats from the legacy PostgreSQL bot into SQLite.
//
// The old schema keeps the settings in bot_chats and the platform identity in
// bot_chats_tg: bot_chats.id is an internal autoincrement, while the Telegram
// chat id lives in bot_chats_tg."peerId" (bigint). The migration therefore
// joins the two tables instead of trusting bot_chats.id — a Telegram id such
// as 6409603928 does not even fit into the int4 column.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// OldChat is one row of the legacy schema, already joined with bot_chats_tg.
type OldChat struct {
	PeerID                   int64
	Accepted                 bool
	EULA                     bool
	Ref                      string
	Scene                    string
	Mode                     string
	Group                    string
	Teacher                  string
	GoogleEmail              string
	ShowAbout                bool
	ShowDaily                bool
	ShowWeekly               bool
	ShowCalls                bool
	ShowFastGroup            bool
	ShowFastTeacher          bool
	HidePastDays             bool
	DeleteLastMsg            bool
	DeleteUserMsg            bool
	LastMsgID                int
	LastMsgTime              int64
	AllowSendMess            bool
	SubscribeDistribution    bool
	NeedUpdateButtons        bool
	DeactivateSecondaryCheck bool
	NoticeChanges            bool
	NoticeNextWeek           bool
	NoticeParserErrors       bool
	ShowParserTime           bool
	ShowHints                bool
	HistoryGroup             string
	HistoryTeacher           string
	ScheduleFormatter        int
}

func main() {
	pgURL := flag.String("pg", "", "PostgreSQL connection string (postgresql://user:pass@host:port/dbname)")
	sqlitePath := flag.String("sqlite", "bot_chats.db", "Output SQLite file path")
	force := flag.Bool("force", false, "Replace the output file if it already exists")
	flag.Parse()

	if *pgURL == "" {
		*pgURL = os.Getenv("DATABASE_URL")
	}
	if *pgURL == "" {
		log.Fatal("Usage: migrate-pg -pg 'postgresql://user:pass@host:port/dbname' [-sqlite bot_chats.db] [-force]")
	}
	if err := prepareOutput(*sqlitePath, *force); err != nil {
		log.Fatal(err)
	}

	log.Printf("Connecting to PostgreSQL...")
	pgDB, err := sql.Open("postgres", *pgURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer pgDB.Close()

	if err := pgDB.Ping(); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	log.Printf("Reading bot_chats from PostgreSQL...")
	rows, err := pgDB.Query(`
		SELECT t."peerId", c.accepted, c.eula, coalesce(c.ref, ''), coalesce(c.scene, ''),
		       coalesce(c.mode::text, ''), coalesce(c."group", ''), coalesce(c.teacher, ''),
		       coalesce(c."googleEmail", ''), c."showAbout", c."showDaily", c."showWeekly",
		       c."showCalls", c."showFastGroup", c."showFastTeacher", c."hidePastDays",
		       c."deleteLastMsg", c."deleteUserMsg", c."lastMsgId", c."lastMsgTime",
		       c."allowSendMess", c."subscribeDistribution", c."needUpdateButtons",
		       c."deactivateSecondaryCheck", c."noticeChanges", c."noticeNextWeek",
		       c."noticeParserErrors", c."showParserTime", c."showHints",
		       coalesce(c."historyGroup", '[]'), coalesce(c."historyTeacher", '[]'),
		       c."scheduleFormatter"
		FROM bot_chats c
		JOIN bot_chats_tg t ON t."chatId" = c.id
		WHERE c.service::text = 'tg'
		ORDER BY t."peerId"
	`)
	if err != nil {
		log.Fatalf("query pg: %v", err)
	}
	defer rows.Close()

	var chats []OldChat
	for rows.Next() {
		var c OldChat
		err := rows.Scan(
			&c.PeerID, &c.Accepted, &c.EULA, &c.Ref, &c.Scene, &c.Mode, &c.Group, &c.Teacher,
			&c.GoogleEmail, &c.ShowAbout, &c.ShowDaily, &c.ShowWeekly, &c.ShowCalls,
			&c.ShowFastGroup, &c.ShowFastTeacher, &c.HidePastDays, &c.DeleteLastMsg,
			&c.DeleteUserMsg, &c.LastMsgID, &c.LastMsgTime, &c.AllowSendMess,
			&c.SubscribeDistribution, &c.NeedUpdateButtons, &c.DeactivateSecondaryCheck,
			&c.NoticeChanges, &c.NoticeNextWeek, &c.NoticeParserErrors, &c.ShowParserTime,
			&c.ShowHints, &c.HistoryGroup, &c.HistoryTeacher, &c.ScheduleFormatter,
		)
		if err != nil {
			log.Printf("scan error: %v", err)
			continue
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("read pg rows: %v", err)
	}

	log.Printf("Found %d Telegram chats in PostgreSQL", len(chats))
	if len(chats) == 0 {
		log.Println("Nothing to migrate.")
		return
	}

	log.Printf("Creating SQLite database: %s", *sqlitePath)
	sqliteDB, err := sql.Open("sqlite", *sqlitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer sqliteDB.Close()

	if err := createChatsSchema(sqliteDB); err != nil {
		log.Fatalf("create the chats schema: %v", err)
	}

	migrated := 0
	for _, c := range chats {
		if err := insertChat(sqliteDB, c); err != nil {
			log.Printf("insert error for peer_id=%d: %v", c.PeerID, err)
			continue
		}
		migrated++
	}

	log.Printf("Migrated %d/%d chats to %s", migrated, len(chats), *sqlitePath)

	var count int
	sqliteDB.QueryRow(`SELECT COUNT(*) FROM bot_chats`).Scan(&count)
	log.Printf("Verification: %d rows in SQLite", count)
}

// prepareOutput makes sure the SQLite target can be written: an existing file
// is only replaced when the caller asked for it.
func prepareOutput(path string, force bool) error {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("%s already exists, pass -force to replace it", path)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

const createChatsTable = `CREATE TABLE IF NOT EXISTS bot_chats (
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
		hide_past_days INTEGER NOT NULL DEFAULT 1,
		delete_last_msg INTEGER NOT NULL DEFAULT 0,
		delete_user_msg INTEGER NOT NULL DEFAULT 0,
		last_msg_id INTEGER NOT NULL DEFAULT 0,
		last_msg_time INTEGER NOT NULL DEFAULT 0,
		allow_send_mess INTEGER NOT NULL DEFAULT 1,
		subscribe_distribution INTEGER NOT NULL DEFAULT 1,
		need_update_buttons INTEGER NOT NULL DEFAULT 0,
		deactivate_secondary_check INTEGER NOT NULL DEFAULT 0,
		notice_changes INTEGER NOT NULL DEFAULT 1,
		notice_next_week INTEGER NOT NULL DEFAULT 1,
		notice_calls INTEGER NOT NULL DEFAULT 1,
		notice_parser_errors INTEGER NOT NULL DEFAULT 1,
		reminder_lead_minutes INTEGER NOT NULL DEFAULT 0,
		notice_week INTEGER NOT NULL DEFAULT 1,
		show_parser_time INTEGER NOT NULL DEFAULT 0,
		show_hints INTEGER NOT NULL DEFAULT 1,
		history_group TEXT NOT NULL DEFAULT '[]',
		history_teacher TEXT NOT NULL DEFAULT '[]',
		diff_enabled INTEGER NOT NULL DEFAULT 1,
		diff_auto_in_week INTEGER NOT NULL DEFAULT 1,
		diff_auto_in_updates INTEGER NOT NULL DEFAULT 1,
		diff_show_before_after INTEGER NOT NULL DEFAULT 1,
		diff_max_lines INTEGER NOT NULL DEFAULT 20,
		ref TEXT,
		UNIQUE(service, peer_id)
	)`

const createChatsIndex = `CREATE INDEX IF NOT EXISTS idx_chats_peer ON bot_chats(service, peer_id)`

func createChatsSchema(db *sql.DB) error {
	if _, err := db.Exec(createChatsTable); err != nil {
		return err
	}
	_, err := db.Exec(createChatsIndex)
	return err
}

const insertChatSQL = `INSERT OR REPLACE INTO bot_chats (
	service, peer_id, accepted, eula, scene, mode, "group", teacher,
	google_email, formatter, show_about, show_daily, show_weekly, show_calls,
	show_fast_group, show_fast_teacher, hide_past_days, delete_last_msg,
	delete_user_msg, last_msg_id, last_msg_time, allow_send_mess,
	subscribe_distribution, need_update_buttons, deactivate_secondary_check,
	notice_changes, notice_next_week, notice_calls, notice_parser_errors,
	show_parser_time, show_hints, history_group, history_teacher,
	diff_enabled, diff_auto_in_week, diff_auto_in_updates, diff_show_before_after,
	diff_max_lines, ref
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// insertChat writes one legacy row as a bot_chats row.
func insertChat(db *sql.DB, c OldChat) error {
	toInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	_, err := db.Exec(insertChatSQL,
		"telegram",
		c.PeerID,
		toInt(c.Accepted),
		toInt(c.EULA),
		c.Scene,
		c.Mode,
		c.Group,
		c.Teacher,
		c.GoogleEmail,
		c.ScheduleFormatter,
		toInt(c.ShowAbout),
		toInt(c.ShowDaily),
		toInt(c.ShowWeekly),
		toInt(c.ShowCalls),
		toInt(c.ShowFastGroup),
		toInt(c.ShowFastTeacher),
		toInt(c.HidePastDays),
		toInt(c.DeleteLastMsg),
		toInt(c.DeleteUserMsg),
		c.LastMsgID,
		c.LastMsgTime,
		toInt(c.AllowSendMess),
		toInt(c.SubscribeDistribution),
		toInt(c.NeedUpdateButtons),
		toInt(c.DeactivateSecondaryCheck),
		toInt(c.NoticeChanges),
		toInt(c.NoticeNextWeek),
		1, // the legacy bot had no calls notification switch, the new one ships it on
		toInt(c.NoticeParserErrors),
		toInt(c.ShowParserTime),
		toInt(c.ShowHints),
		c.HistoryGroup,
		c.HistoryTeacher,
		1, 1, 1, 1, // diff switches default to on
		20, // and the diff is limited to twenty lines
		c.Ref,
	)
	return err
}
