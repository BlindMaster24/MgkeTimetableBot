package preflight

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestEndpointsForAppliesEachOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.Parser.Endpoints.TimetableGroup = "group"
	cfg.Parser.Endpoints.TimetableTeacher = "teacher"
	cfg.Parser.Endpoints.BellSchedule = "calls"

	if got := endpointsFor(cfg, Endpoints{}); got.Groups != "group" || got.Teachers != "teacher" || got.Calls != "calls" {
		t.Fatalf("endpoints = %+v, want the config values", got)
	}

	got := endpointsFor(cfg, Endpoints{Groups: "g2", Teachers: "t2", Calls: "c2"})
	if got.Groups != "g2" || got.Teachers != "t2" || got.Calls != "c2" {
		t.Fatalf("overrides = %+v, want every field replaced", got)
	}

	partial := endpointsFor(cfg, Endpoints{Groups: "g2"})
	if partial.Groups != "g2" || partial.Teachers != "teacher" || partial.Calls != "calls" {
		t.Fatalf("a partial override = %+v, want only the group replaced", partial)
	}
}

func TestRSAKeyBitsReadsTheKeyShapes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	generic := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})

	for name, body := range map[string][]byte{"pkcs1": pkcs1, "pkcs8": generic} {
		bits, err := rsaKeyBits(string(body))
		if err != nil || bits != 2048 {
			t.Fatalf("%s key = %d %v, want 2048", name, bits, err)
		}
	}

	if _, err := rsaKeyBits("not a key"); err == nil || !strings.Contains(err.Error(), "PEM block") {
		t.Fatalf("a non-PEM value = %v, want the \n hint", err)
	}
	if _, err := rsaKeyBits(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("broken")}))); err == nil {
		t.Fatal("a broken PKCS8 body must be reported")
	}
}
