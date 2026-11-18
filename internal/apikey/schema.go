package apikey

func (s *Store) EnsureSchema() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL UNIQUE,
		limit_per_sec INTEGER NOT NULL DEFAULT 2,
		iv BLOB NOT NULL,
		last_used INTEGER NOT NULL DEFAULT 0
	)`)
	return err
}
