package apikey

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupStore(t *testing.T) *Store {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	store := NewStore(db, testSecret)
	if err := store.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestStoreIssuesAKeyWithTheDefaultLimit(t *testing.T) {
	store := setupStore(t)

	key, created, err := store.FindOrCreate(4242)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("the first call must create the key")
	}
	if key.ChatID != 4242 || key.LimitPerSec != DefaultLimitPerSec {
		t.Fatalf("unexpected key: %+v", key)
	}
	if key.Used {
		t.Fatal("a new key must not be marked as used")
	}

	token, err := store.Token(key)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("the key must produce a token")
	}

	found, err := store.ByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != key.ID || found.ChatID != 4242 {
		t.Fatalf("unexpected key from the token: %+v", found)
	}
}

func TestStoreReturnsTheSameKeyOnTheSecondCall(t *testing.T) {
	store := setupStore(t)

	first, _, err := store.FindOrCreate(42)
	if err != nil {
		t.Fatal(err)
	}
	second, created, err := store.FindOrCreate(42)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("the second call must reuse the row")
	}
	if first.ID != second.ID {
		t.Fatalf("ids differ: %d and %d", first.ID, second.ID)
	}
}

func TestStoreRotatesTheToken(t *testing.T) {
	store := setupStore(t)

	key, _, err := store.FindOrCreate(7)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := store.Token(key)

	if err := store.Rotate(7); err != nil {
		t.Fatal(err)
	}

	after, err := store.ByChatID(7)
	if err != nil {
		t.Fatal(err)
	}
	rotated, _ := store.Token(after)
	if before == rotated {
		t.Fatal("the rotated token must differ")
	}

	if _, err := store.ByToken(before); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("the old token must stop working, got %v", err)
	}
	if _, err := store.ByToken(rotated); err != nil {
		t.Fatalf("the rotated token must work, got %v", err)
	}
}

func TestStoreRejectsTokensOfUnknownKeys(t *testing.T) {
	store := setupStore(t)

	tool := newTool(testSecret)
	iv, _ := tool.CreateIV()
	foreign, err := tool.Encode(999, iv)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.ByToken(foreign); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("err = %v, want ErrInvalidKey", err)
	}
}

func TestStoreTracksTheLastUsage(t *testing.T) {
	store := setupStore(t)

	key, _, err := store.FindOrCreate(11)
	if err != nil {
		t.Fatal(err)
	}

	at := time.UnixMilli(1767225600000)
	if err := store.Touch(key.ID, at); err != nil {
		t.Fatal(err)
	}

	updated, err := store.ByChatID(11)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Used {
		t.Fatal("the key must be marked as used")
	}
	if !updated.LastUsed.Equal(at) {
		t.Fatalf("lastUsed = %s, want %s", updated.LastUsed, at)
	}
}

func TestStoreSetsTheLimit(t *testing.T) {
	store := setupStore(t)

	if _, _, err := store.FindOrCreate(12); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLimit(12, 25); err != nil {
		t.Fatal(err)
	}

	key, err := store.ByChatID(12)
	if err != nil {
		t.Fatal(err)
	}
	if key.LimitPerSec != 25 {
		t.Fatalf("limit = %d, want 25", key.LimitPerSec)
	}

	if err := store.SetLimit(404, 1); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreServesTheSystemKey(t *testing.T) {
	store := setupStore(t)

	token, err := store.SystemToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("the system key must produce a token")
	}

	key, err := store.ByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if key.ChatID != SystemChatID || key.LimitPerSec != SystemLimitPerSec {
		t.Fatalf("unexpected system key: %+v", key)
	}

	again, err := store.SystemToken()
	if err != nil {
		t.Fatal(err)
	}
	if again != token {
		t.Fatal("the system key must be stable")
	}
}

func TestStoreStaysOffWithoutASecret(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db, "")
	if store.Enabled() {
		t.Fatal("an empty secret must leave the store disabled")
	}
	if err := store.EnsureSchema(); err != nil {
		t.Fatal(err)
	}

	key, _, err := store.FindOrCreate(5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Token(key); !errors.Is(err, ErrShortSecret) {
		t.Fatalf("err = %v, want ErrShortSecret", err)
	}
	if _, err := store.ByToken("anything"); !errors.Is(err, ErrShortSecret) {
		t.Fatalf("err = %v, want ErrShortSecret", err)
	}
}
