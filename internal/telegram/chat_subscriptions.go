package telegram

import ()

func (r *Repository) CountSubscriptionsByType(service, subType string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(DISTINCT s.chat_id) FROM subscriptions s
		 JOIN bot_chats c ON c.id = s.chat_id
		 WHERE s.type = ? AND c.service = ? AND c.accepted = 1 AND c.allow_send_mess = 1`,
		subType, service,
	).Scan(&count)
	return count, err
}

type Subscription struct {
	ID     int64
	ChatID int64
	Type   string
	Value  string
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

func (r *Repository) CountSubscriptions(chatID int64) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE chat_id = ?`, chatID).Scan(&count)
	return count, err
}
