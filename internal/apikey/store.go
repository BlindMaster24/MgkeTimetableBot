package apikey

import (
	"bytes"
	"database/sql"
	"errors"
	"sync"
	"time"
)

const (
	DefaultLimitPerSec = 2
	SystemChatID       = 0
	SystemLimitPerSec  = 0
)

var ErrKeyNotFound = errors.New("api key not found")

type Key struct {
	ID          int64
	ChatID      int64
	LimitPerSec int
	IV          []byte
	LastUsed    time.Time
	Used        bool
}

type Store struct {
	db   *sql.DB
	tool *Tool
	mu   sync.Mutex
}

func NewStore(db *sql.DB, secret string) *Store {
	return &Store{db: db, tool: newTool(secret)}
}

func (s *Store) Enabled() bool {
	return s.tool.Enabled()
}

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

func (s *Store) FindOrCreate(chatID int64) (*Key, bool, error) {
	existing, err := s.ByChatID(chatID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrKeyNotFound) {
		return nil, false, err
	}

	iv, err := s.tool.CreateIV()
	if err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	result, err := s.db.Exec(
		`INSERT OR IGNORE INTO api_keys (chat_id, limit_per_sec, iv) VALUES (?, ?, ?)`,
		chatID, DefaultLimitPerSec, iv,
	)
	s.mu.Unlock()
	if err != nil {
		return nil, false, err
	}

	created := true
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		created = false
	}

	key, err := s.ByChatID(chatID)
	if err != nil {
		return nil, false, err
	}
	return key, created, nil
}

func (s *Store) Rotate(chatID int64) error {
	iv, err := s.tool.CreateIV()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE api_keys SET iv = ? WHERE chat_id = ?`, iv, chatID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func (s *Store) ByChatID(chatID int64) (*Key, error) {
	row := s.db.QueryRow(
		`SELECT id, chat_id, limit_per_sec, iv, last_used FROM api_keys WHERE chat_id = ?`,
		chatID,
	)
	return scanKey(row)
}

func (s *Store) ByToken(token string) (*Key, error) {
	id, iv, err := s.tool.Decode(token)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRow(
		`SELECT id, chat_id, limit_per_sec, iv, last_used FROM api_keys WHERE id = ?`,
		id,
	)
	key, err := scanKey(row)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrInvalidKey
		}
		return nil, err
	}
	if !bytes.Equal(key.IV, iv) {
		return nil, ErrInvalidKey
	}
	return key, nil
}

func (s *Store) Touch(id int64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE api_keys SET last_used = ? WHERE id = ?`, at.UnixMilli(), id)
	return err
}

func (s *Store) SetLimit(chatID int64, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`UPDATE api_keys SET limit_per_sec = ? WHERE chat_id = ?`, limit, chatID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func (s *Store) Token(key *Key) (string, error) {
	return s.tool.Encode(key.ID, key.IV)
}

func (s *Store) SystemToken() (string, error) {
	key, _, err := s.FindOrCreate(SystemChatID)
	if err != nil {
		return "", err
	}
	if key.LimitPerSec != SystemLimitPerSec {
		if err := s.SetLimit(SystemChatID, SystemLimitPerSec); err != nil {
			return "", err
		}
		key.LimitPerSec = SystemLimitPerSec
	}
	return s.Token(key)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKey(row rowScanner) (*Key, error) {
	var (
		key      Key
		lastUsed int64
	)
	err := row.Scan(&key.ID, &key.ChatID, &key.LimitPerSec, &key.IV, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastUsed > 0 {
		key.LastUsed = time.UnixMilli(lastUsed)
		key.Used = true
	}
	return &key, nil
}
