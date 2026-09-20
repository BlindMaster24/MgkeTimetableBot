package apikey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const (
	apiKeyType    = 2
	ivLength      = 16
	hmacLength    = 20
	secretMinimum = 32
)

var (
	ErrInvalidKey  = errors.New("invalid key")
	ErrKeyType     = errors.New("key type error")
	ErrShortSecret = errors.New("encrypt_key must be at least 32 characters")
)

type Tool struct {
	secret []byte
}

func newTool(secret string) *Tool {
	return &Tool{secret: []byte(secret)}
}

func NewTool(secret string) *Tool {
	return newTool(secret)
}

func (t *Tool) EncodeAs(id int64, keyType byte, iv []byte) (string, error) {
	return t.encode(id, keyType, iv)
}

func (t *Tool) Enabled() bool {
	return len(t.secret) >= secretMinimum
}

func (t *Tool) CreateIV() ([]byte, error) {
	iv := make([]byte, ivLength)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	return iv, nil
}

func (t *Tool) Encode(id int64, iv []byte) (string, error) {
	return t.encode(id, apiKeyType, iv)
}

func (t *Tool) encode(id int64, keyType byte, iv []byte) (string, error) {
	if !t.Enabled() {
		return "", ErrShortSecret
	}
	if len(iv) != ivLength {
		return "", ErrInvalidKey
	}

	plain := make([]byte, 1+8)
	plain[0] = keyType << 6
	binary.BigEndian.PutUint64(plain[1:], uint64(id))

	block, err := aes.NewCipher(t.cipherKey())
	if err != nil {
		return "", err
	}

	padded := pad(plain, aes.BlockSize)
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)

	mac := hmac.New(sha1.New, t.secret)
	mac.Write(iv)
	mac.Write(encrypted)
	sum := mac.Sum(nil)[:hmacLength]

	raw := make([]byte, 0, hmacLength+ivLength+len(encrypted))
	raw = append(raw, sum...)
	raw = append(raw, iv...)
	raw = append(raw, encrypted...)

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (t *Tool) Decode(token string) (int64, []byte, error) {
	if !t.Enabled() {
		return 0, nil, ErrShortSecret
	}

	raw, err := decodeBase64(token)
	if err != nil {
		return 0, nil, ErrInvalidKey
	}
	if len(raw) < hmacLength+ivLength+aes.BlockSize {
		return 0, nil, ErrInvalidKey
	}

	sum := raw[:hmacLength]
	iv := raw[hmacLength : hmacLength+ivLength]
	encrypted := raw[hmacLength+ivLength:]

	mac := hmac.New(sha1.New, t.secret)
	mac.Write(iv)
	mac.Write(encrypted)
	if !hmac.Equal(mac.Sum(nil)[:hmacLength], sum) {
		return 0, nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(t.cipherKey())
	if err != nil {
		return 0, nil, err
	}
	if len(encrypted)%aes.BlockSize != 0 {
		return 0, nil, ErrInvalidKey
	}

	plain := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encrypted)

	plain, err = unpad(plain, aes.BlockSize)
	if err != nil || len(plain) < 9 {
		return 0, nil, ErrInvalidKey
	}
	if plain[0]>>6 != apiKeyType {
		return 0, nil, ErrKeyType
	}

	id := int64(binary.BigEndian.Uint64(plain[1:9]))
	parsedIV := append([]byte(nil), iv...)

	return id, parsedIV, nil
}

func (t *Tool) cipherKey() []byte {
	return t.secret[:secretMinimum]
}

func decodeBase64(token string) ([]byte, error) {
	if raw, err := base64.RawURLEncoding.DecodeString(token); err == nil {
		return raw, nil
	}
	return base64.URLEncoding.DecodeString(token)
}

func pad(data []byte, size int) []byte {
	fill := size - len(data)%size
	out := make([]byte, len(data)+fill)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(fill)
	}
	return out
}

func unpad(data []byte, size int) ([]byte, error) {
	if len(data) == 0 || len(data)%size != 0 {
		return nil, ErrInvalidKey
	}
	fill := int(data[len(data)-1])
	if fill == 0 || fill > size || fill > len(data) {
		return nil, ErrInvalidKey
	}
	for _, b := range data[len(data)-fill:] {
		if int(b) != fill {
			return nil, ErrInvalidKey
		}
	}
	return data[:len(data)-fill], nil
}
