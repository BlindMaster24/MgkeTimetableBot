package main

import (
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type OldChat struct {
	ID                    int
	Service               string
	Accepted              bool
	Ref                   string
	Scene                 string
	Mode                  string
	Group                 string
	Teacher               string
	GoogleEmail           string
	ShowAbout             bool
	ShowDaily             bool
	ShowWeekly            bool
	ShowCalls             bool
	ShowFastGroup         bool
	ShowFastTeacher       bool
	HidePastDays          bool
	DeleteLastMsg         bool
	LastMsgID             int
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
	ScheduleFormatter     int
}

func main() {
	pgURL := flag.String("pg", "", "PostgreSQL connection string (postgresql://user:pass@host:port/dbname)")
	sqlitePath := flag.String("sqlite", "sqlite3.db", "Output SQLite file path")
	flag.Parse()

	if *pgURL == "" {
		*pgURL = os.Getenv("DATABASE_URL")
	}
	if *pgURL == "" {
		log.Fatal("Usage: migrate-pg -pg 'postgresql://user:pass@host:port/dbname' [-sqlite sqlite3.db]")
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
		SELECT id, service, accepted, ref, scene, mode, "group", teacher, "googleEmail",
		       "showAbout", "showDaily", "showWeekly", "showCalls", "showFastGroup", "showFastTeacher",
		       "hidePastDays", "deleteLastMsg", "lastMsgId", "allowSendMess",
		       "noticeChanges", "noticeNextWeek", "noticeCalls", "noticeParserErrors",
		       "showParserTime", "showHints", "diffEnabled", "diffAutoInWeek",
		       "diffAutoInUpdates", "diffShowBeforeAfter", "diffMaxLines", "scheduleFormatter"
		FROM bot_chats
		WHERE service = 'tg' AND accepted = true
	`)
	if err != nil {
		log.Fatalf("query pg: %v", err)
	}
	defer rows.Close()

	var chats []OldChat
	for rows.Next() {
		var c OldChat
		err := rows.Scan(
			&c.ID, &c.Service, &c.Accepted, &c.Ref, &c.Scene, &c.Mode, &c.Group, &c.Teacher, &c.GoogleEmail,
			&c.ShowAbout, &c.ShowDaily, &c.ShowWeekly, &c.ShowCalls, &c.ShowFastGroup, &c.ShowFastTeacher,
			&c.HidePastDays, &c.DeleteLastMsg, &c.LastMsgID, &c.AllowSendMess,
			&c.NoticeChanges, &c.NoticeNextWeek, &c.NoticeCalls, &c.NoticeParserErrors,
			&c.ShowParserTime, &c.ShowHints, &c.DiffEnabled, &c.DiffAutoInWeek,
			&c.DiffAutoInUpdates, &c.DiffShowBeforeAfter, &c.DiffMaxLines, &c.ScheduleFormatter,
		)
		if err != nil {
			log.Printf("scan error: %v", err)
			continue
		}
		chats = append(chats, c)
	}

	log.Printf("Found %d chats in PostgreSQL (service='tg', accepted=true)", len(chats))

	if len(chats) == 0 {
		log.Println("Nothing to migrate.")
		return
	}

	os.Remove(*sqlitePath)
	log.Printf("Creating SQLite database: %s", *sqlitePath)
	sqliteDB, err := sql.Open("sqlite", *sqlitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer sqliteDB.Close()

	_, err = sqliteDB.Exec(`CREATE TABLE IF NOT EXISTS bot_chats (
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
		log.Fatalf("create sqlite table: %v", err)
	}

	_, err = sqliteDB.Exec(`CREATE INDEX IF NOT EXISTS idx_chats_peer ON bot_chats(service, peer_id)`)
	if err != nil {
		log.Fatalf("create index: %v", err)
	}

	toInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	migrated := 0
	for _, c := range chats {
		peerID := c.ID

		_, err := sqliteDB.Exec(
			`INSERT OR REPLACE INTO bot_chats (
				service, peer_id, accepted, scene, mode, "group", teacher,
				google_email, formatter, show_about, show_daily, show_weekly,
				show_calls, show_fast_group, show_fast_teacher, hide_past_days,
				delete_last_msg, last_msg_id, allow_send_mess, notice_changes,
				notice_next_week, notice_calls, notice_parser_errors,
				show_parser_time, show_hints, diff_enabled, diff_auto_in_week,
				diff_auto_in_updates, diff_show_before_after, diff_max_lines, ref
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"telegram",
			peerID,
			toInt(c.Accepted),
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
			c.LastMsgID,
			toInt(c.AllowSendMess),
			toInt(c.NoticeChanges),
			toInt(c.NoticeNextWeek),
			toInt(c.NoticeCalls),
			toInt(c.NoticeParserErrors),
			toInt(c.ShowParserTime),
			toInt(c.ShowHints),
			toInt(c.DiffEnabled),
			toInt(c.DiffAutoInWeek),
			toInt(c.DiffAutoInUpdates),
			toInt(c.DiffShowBeforeAfter),
			c.DiffMaxLines,
			c.Ref,
		)
		if err != nil {
			log.Printf("insert error for peer_id=%d: %v", peerID, err)
			continue
		}
		migrated++
	}

	log.Printf("Migrated %d/%d chats to %s", migrated, len(chats), *sqlitePath)

	var count int
	sqliteDB.QueryRow(`SELECT COUNT(*) FROM bot_chats`).Scan(&count)
	log.Printf("Verification: %d rows in SQLite", count)
}
