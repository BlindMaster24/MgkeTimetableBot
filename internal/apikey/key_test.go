package apikey

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func timeAt(millis int) time.Time {
	return time.UnixMilli(int64(millis))
}

const keyAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func tamperSymbol(token string, index int) string {
	replacement := byte('A')
	for i := 0; i < len(keyAlphabet); i++ {
		if keyAlphabet[i] != token[index] {
			replacement = keyAlphabet[i]
			break
		}
	}
	return token[:index] + string(replacement) + token[index+1:]
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tool := newTool(testSecret)

	iv, err := tool.CreateIV()
	if err != nil {
		t.Fatal(err)
	}

	token, err := tool.Encode(4242, iv)
	if err != nil {
		t.Fatal(err)
	}

	id, parsedIV, err := tool.Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if id != 4242 {
		t.Fatalf("id = %d, want 4242", id)
	}
	if string(parsedIV) != string(iv) {
		t.Fatal("the iv must survive the round trip")
	}
}

func TestEncodeProducesURLSafeTokens(t *testing.T) {
	tool := newTool(testSecret)
	iv, _ := tool.CreateIV()

	token, err := tool.Encode(7, iv)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("the token must be url safe: %q", token)
	}
}

func TestEncodeGivesDifferentTokensForTheSameRow(t *testing.T) {
	tool := newTool(testSecret)

	firstIV, _ := tool.CreateIV()
	secondIV, _ := tool.CreateIV()

	first, _ := tool.Encode(3, firstIV)
	second, _ := tool.Encode(3, secondIV)

	if first == second {
		t.Fatal("a rotated key must differ from the previous one")
	}
}

func TestDecodeRejectsAnotherSecret(t *testing.T) {
	tool := newTool(testSecret)
	iv, _ := tool.CreateIV()
	token, _ := tool.Encode(1, iv)

	other := newTool("ffffffffffffffffffffffffffffffff")
	if _, _, err := other.Decode(token); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("err = %v, want ErrInvalidKey", err)
	}
}

func TestDecodeRejectsBrokenTokens(t *testing.T) {
	tool := newTool(testSecret)
	iv, _ := tool.CreateIV()
	token, _ := tool.Encode(9, iv)

	cases := map[string]string{
		"truncated":        token[:len(token)-4],
		"garbage":          "not-a-key",
		"empty":            "",
		"flippedHmac":      tamperSymbol(token, 3),
		"flippedIv":        tamperSymbol(token, 24),
		"flippedPayload":   tamperSymbol(token, 45),
		"duplicated":       token + token,
		"paddedCiphertext": token + "AAAA",
	}

	for name, broken := range cases {
		if _, _, err := tool.Decode(broken); err == nil {
			t.Errorf("%s token must not decode", name)
		}
	}
}

func TestDecodeRejectsAForeignKeyType(t *testing.T) {
	tool := newTool(testSecret)
	iv, _ := tool.CreateIV()

	token, err := tool.encode(5, 3, iv)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := tool.Decode(token); !errors.Is(err, ErrKeyType) {
		t.Fatalf("err = %v, want ErrKeyType", err)
	}
}

func TestToolRejectsAShortSecret(t *testing.T) {
	tool := newTool("too-short")
	if tool.Enabled() {
		t.Fatal("a short secret must not be usable")
	}
	if _, err := tool.Encode(1, make([]byte, ivLength)); !errors.Is(err, ErrShortSecret) {
		t.Fatalf("err = %v, want ErrShortSecret", err)
	}
	if _, _, err := tool.Decode("whatever"); !errors.Is(err, ErrShortSecret) {
		t.Fatalf("err = %v, want ErrShortSecret", err)
	}
}

func TestLimiterAllowsUpToTheLimit(t *testing.T) {
	now := timeAt(0)
	limiter := NewLimiter(func() time.Time { return now })

	first := limiter.Allow(1, 2)
	second := limiter.Allow(1, 2)
	if !first || !second {
		t.Fatal("the first two requests must pass")
	}
	if limiter.Allow(1, 2) {
		t.Fatal("the third request within a second must be limited")
	}

	now = timeAt(1500)
	if !limiter.Allow(1, 2) {
		t.Fatal("the window must slide")
	}
}

func TestLimiterTreatsZeroAsUnlimited(t *testing.T) {
	limiter := NewLimiter(nil)
	for i := 0; i < 50; i++ {
		if !limiter.Allow(3, 0) {
			t.Fatal("a zero limit must disable the limiter")
		}
	}
}

func TestLimiterKeepsKeysApart(t *testing.T) {
	limiter := NewLimiter(nil)
	if !limiter.Allow(1, 1) {
		t.Fatal("the first key must pass")
	}
	if !limiter.Allow(2, 1) {
		t.Fatal("another key must have its own window")
	}
	if limiter.Allow(1, 1) {
		t.Fatal("the first key must be limited")
	}
}

func TestABase64SecretMatchesTheRawKeyBytes(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	fromRaw := NewTool(string(raw))
	fromBase64 := NewTool("base64:" + encoded)
	iv := []byte("0123456789abcdef")

	token, err := fromRaw.Encode(42, iv)
	if err != nil {
		t.Fatal(err)
	}
	id, parsedIV, err := fromBase64.Decode(token)
	if err != nil {
		t.Fatalf("decode with the base64 secret: %v", err)
	}
	if id != 42 || string(parsedIV) != string(iv) {
		t.Fatalf("decoded id = %d, iv = %q", id, parsedIV)
	}

	// A legacy token was built from the raw bytes, so the base64 form must
	// produce exactly the same string.
	legacy, err := fromBase64.Encode(42, iv)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != token {
		t.Fatalf("base64 secret encodes %q, raw bytes encode %q", legacy, token)
	}
}

func TestLegacyTypeScriptTokenDecodesHere(t *testing.T) {
	// Built by the old TypeScript bot from this exact key and iv; keep the pair
	// so a migrated database keeps its tokens valid.
	const legacy = "YD8F7WrLD9NSdUWVtZCN4dDRfVjUcRiURNnPYYexh0IzjOlozD0lVv7ew2mvrYHdQ643xQ"
	tool := NewTool("base64:VrbPG9ccdGJSp3zWF2X/F8PJGqV9FzbHyVXWCNYs7XM=")

	id, iv, err := tool.Decode(legacy)
	if err != nil {
		t.Fatalf("decode the legacy token: %v", err)
	}
	if id != 137 {
		t.Fatalf("id = %d, want 137", id)
	}
	if got := hex.EncodeToString(iv); got != "d471189444d9cf6187b18742338ce968" {
		t.Fatalf("iv = %s", got)
	}
	again, err := tool.Encode(id, iv)
	if err != nil {
		t.Fatal(err)
	}
	if again != legacy {
		t.Fatalf("re-encoded token = %s", again)
	}
}
