package apikey

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestCreateIVIsRandomAndSixteenBytes(t *testing.T) {
	tool := NewTool("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	first, err := tool.CreateIV()
	if err != nil {
		t.Fatalf("CreateIV: %v", err)
	}
	second, err := tool.CreateIV()
	if err != nil {
		t.Fatalf("CreateIV: %v", err)
	}
	if len(first) != ivLength || len(second) != ivLength {
		t.Fatalf("iv lengths = %d and %d, want %d", len(first), len(second), ivLength)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two IVs must not repeat")
	}
}

func TestEncodeAsKeepsTheTypeInTheToken(t *testing.T) {
	tool := NewTool("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	iv, err := tool.CreateIV()
	if err != nil {
		t.Fatal(err)
	}

	apiToken, err := tool.EncodeAs(137, apiKeyType, iv)
	if err != nil {
		t.Fatalf("EncodeAs: %v", err)
	}
	plainToken, err := tool.Encode(137, iv)
	if err != nil {
		t.Fatal(err)
	}
	if apiToken != plainToken {
		t.Fatal("the api key type must match Encode")
	}
	if id, gotIV, err := tool.Decode(apiToken); err != nil || id != 137 || !bytes.Equal(gotIV, iv) {
		t.Fatalf("Decode = %d %x %v, want the encoded row", id, gotIV, err)
	}

	foreign, err := tool.EncodeAs(137, 3, iv)
	if err != nil {
		t.Fatalf("EncodeAs with another type: %v", err)
	}
	if _, _, err := tool.Decode(foreign); !errors.Is(err, ErrKeyType) {
		t.Fatalf("a token of another type decoded: %v", err)
	}
}

func TestUnpadRejectsBrokenPadding(t *testing.T) {
	padded := pad([]byte("hello"), 16)
	plain, err := unpad(padded, 16)
	if err != nil || string(plain) != "hello" {
		t.Fatalf("unpad = %q %v, want the original text", plain, err)
	}

	if _, err := unpad([]byte("0123456789abcdef"), 16); err == nil {
		t.Fatal("a full block of padding bytes must be rejected")
	}
	if _, err := unpad([]byte("short"), 16); err == nil {
		t.Fatal("an empty block must be rejected")
	}
}

func TestCountsTellTotalFromUsed(t *testing.T) {
	store := setupStore(t)

	if total, active, err := store.Counts(); err != nil || total != 0 || active != 0 {
		t.Fatalf("empty counts = %d/%d %v, want 0/0", total, active, err)
	}
	first, _, err := store.FindOrCreate(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.FindOrCreate(2); err != nil {
		t.Fatal(err)
	}
	if err := store.Touch(first.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	total, active, err := store.Counts()
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if total != 2 || active != 1 {
		t.Fatalf("counts = %d/%d, want 2 total and 1 used", total, active)
	}
}

func TestCountsReportAClosedDatabase(t *testing.T) {
	store := setupStore(t)
	if _, _, err := store.FindOrCreate(1); err != nil {
		t.Fatal(err)
	}
	store.db.Close()

	if _, _, err := store.Counts(); err == nil {
		t.Fatal("Counts must report a closed database")
	}
	if err := store.SetLimit(1, 5); err == nil {
		t.Fatal("SetLimit must report a closed database")
	}
	if _, err := store.ByChatID(1); err == nil {
		t.Fatal("a closed store must not answer queries")
	}
}
