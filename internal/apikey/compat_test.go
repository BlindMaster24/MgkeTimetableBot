package apikey

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

const oldBotSecret = "0123456789abcdef0123456789abcdef"

func oldBotIV(t *testing.T) []byte {
	t.Helper()

	iv, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	return iv
}

func TestTokensMatchTheOldTypeScriptLayout(t *testing.T) {
	tool := newTool(oldBotSecret)
	iv := oldBotIV(t)

	cases := map[int64]string{
		0:          "z8qfPPDcR3gnUDzbA810lvI50gsAESIzRFVmd4iZqrvM3e7_Sv0ps3GL83567STvkJGqMw",
		1:          "v4fLHL5Y6jnYaLiXCXTSD0l8s_UAESIzRFVmd4iZqrvM3e7_m_2DfjBblVLNX9Uv5RnyLg",
		4242:       "5v5RUlBzLZrEfhlMR8ijvKI9LrcAESIzRFVmd4iZqrvM3e7_fXqtc0TC1U35uQAjN8XXDg",
		4294967296: "8bwnF18kS5HP-mXln8FfR8dFH8kAESIzRFVmd4iZqrvM3e7_yQkJAT4rU_5NNI7xheHqfg",
	}

	for id, expected := range cases {
		token, err := tool.Encode(id, iv)
		if err != nil {
			t.Fatal(err)
		}
		if token != expected {
			t.Fatalf("id %d: the token must match the old bot byte for byte\n got %s\nwant %s", id, token, expected)
		}

		decoded, parsedIV, err := tool.Decode(expected)
		if err != nil {
			t.Fatalf("id %d: the old token must decode: %v", id, err)
		}
		if decoded != id {
			t.Fatalf("id %d: decoded %d", id, decoded)
		}
		if !bytes.Equal(parsedIV, iv) {
			t.Fatalf("id %d: the iv must survive the round trip", id)
		}
	}
}

func TestTheOldLayoutCarriesTheApiKeyType(t *testing.T) {
	tool := newTool(oldBotSecret)

	if _, _, err := tool.Decode("S9EBaffbVqbKT6DpCD9WFbhswr4AESIzRFVmd4iZqrvM3e7_mIaV67oiYulxgl1cij7XYQ"); !errors.Is(err, ErrKeyType) {
		t.Fatalf("a token of another key type must be rejected, got %v", err)
	}
}

func TestStoredTokensKeepTheSixtyTwoByteShape(t *testing.T) {
	tool := newTool(oldBotSecret)
	iv := oldBotIV(t)

	token, err := tool.Encode(1, iv)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 70 {
		t.Fatalf("token length = %d, want 70 characters for 52 bytes", len(token))
	}

	raw, err := decodeBase64(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != hmacLength+ivLength+16 {
		t.Fatalf("raw length = %d, want %d", len(raw), hmacLength+ivLength+16)
	}
	if !bytes.Equal(raw[hmacLength:hmacLength+ivLength], iv) {
		t.Fatal("the iv must follow the hmac")
	}
}
