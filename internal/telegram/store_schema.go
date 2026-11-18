package telegram

func (r *AliasRepository) EnsureTable() {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.db.Exec(`CREATE TABLE IF NOT EXISTS aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		telegram_user_id INTEGER NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		UNIQUE(telegram_user_id, key)
	)`)
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
