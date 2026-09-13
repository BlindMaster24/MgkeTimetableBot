package telegram

import (
	"database/sql"
	"strings"
)

type GoogleAccount struct {
	Email              string
	RefreshToken       string
	AccessToken        string
	AccessTokenExpires int64
}

type GoogleCalendar struct {
	ID                  int64
	Type                string
	Value               string
	CalendarID          string
	LastManualSyncedDay int64
}

func (r *Repository) migrateGoogle() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS google_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		refresh_token TEXT,
		access_token TEXT,
		access_token_expires INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`CREATE TABLE IF NOT EXISTS google_calendars (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		calendar_id TEXT NOT NULL UNIQUE,
		last_manual_synced_day INTEGER NOT NULL DEFAULT 0,
		UNIQUE(type, value)
	)`)
	return err
}

func (r *Repository) SaveGoogleAccount(account *GoogleAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(
		`INSERT INTO google_accounts (email, refresh_token, access_token, access_token_expires)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET
			refresh_token = excluded.refresh_token,
			access_token = excluded.access_token,
			access_token_expires = excluded.access_token_expires`,
		account.Email, account.RefreshToken, account.AccessToken, account.AccessTokenExpires,
	)
	return err
}

func (r *Repository) GoogleAccountByEmail(email string) (*GoogleAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	row := r.db.QueryRow(
		`SELECT email, refresh_token, access_token, access_token_expires FROM google_accounts WHERE email = ?`,
		email,
	)
	account := &GoogleAccount{}
	var refresh, access sql.NullString
	if err := row.Scan(&account.Email, &refresh, &access, &account.AccessTokenExpires); err != nil {
		return nil, err
	}
	account.RefreshToken = refresh.String
	account.AccessToken = access.String
	return account, nil
}

func (r *Repository) GoogleCalendarByTypeValue(kind, value string) (*GoogleCalendar, error) {
	return r.scanGoogleCalendar(r.db.QueryRow(
		`SELECT id, type, value, calendar_id, last_manual_synced_day FROM google_calendars WHERE type = ? AND value = ?`,
		kind, value,
	))
}

func (r *Repository) GoogleCalendarByLocalID(id int64) (*GoogleCalendar, error) {
	return r.scanGoogleCalendar(r.db.QueryRow(
		`SELECT id, type, value, calendar_id, last_manual_synced_day FROM google_calendars WHERE id = ?`,
		id,
	))
}

func (r *Repository) scanGoogleCalendar(row *sql.Row) (*GoogleCalendar, error) {
	calendar := &GoogleCalendar{}
	err := row.Scan(&calendar.ID, &calendar.Type, &calendar.Value, &calendar.CalendarID, &calendar.LastManualSyncedDay)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return calendar, nil
}

func (r *Repository) GoogleCalendarsByIDs(ids []string) ([]*GoogleCalendar, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	rows, err := r.db.Query(
		`SELECT id, type, value, calendar_id, last_manual_synced_day FROM google_calendars WHERE calendar_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GoogleCalendar
	for rows.Next() {
		calendar := &GoogleCalendar{}
		if err := rows.Scan(&calendar.ID, &calendar.Type, &calendar.Value, &calendar.CalendarID, &calendar.LastManualSyncedDay); err != nil {
			return nil, err
		}
		out = append(out, calendar)
	}
	return out, rows.Err()
}

func (r *Repository) AllGoogleCalendars() ([]*GoogleCalendar, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(`SELECT id, type, value, calendar_id, last_manual_synced_day FROM google_calendars ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GoogleCalendar
	for rows.Next() {
		calendar := &GoogleCalendar{}
		if err := rows.Scan(&calendar.ID, &calendar.Type, &calendar.Value, &calendar.CalendarID, &calendar.LastManualSyncedDay); err != nil {
			return nil, err
		}
		out = append(out, calendar)
	}
	return out, rows.Err()
}

func (r *Repository) SaveGoogleCalendar(calendar *GoogleCalendar) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(
		`INSERT INTO google_calendars (type, value, calendar_id, last_manual_synced_day)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(calendar_id) DO UPDATE SET
			type = excluded.type,
			value = excluded.value,
			last_manual_synced_day = excluded.last_manual_synced_day`,
		calendar.Type, calendar.Value, calendar.CalendarID, calendar.LastManualSyncedDay,
	)
	if err != nil {
		return err
	}

	if calendar.ID == 0 {
		r.db.QueryRow(`SELECT id FROM google_calendars WHERE calendar_id = ?`, calendar.CalendarID).Scan(&calendar.ID)
	}
	return nil
}

func (r *Repository) DeleteGoogleCalendar(calendarID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`DELETE FROM google_calendars WHERE calendar_id = ?`, calendarID)
	return err
}
