package google

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestCalendarServiceInit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Google.RedirectDomain = "https://example.com"
	cfg.Google.URL = "/google/oauth"
	cfg.Google.OAuth.ClientID = "test-client-id"
	cfg.Google.OAuth.ClientSecret = "test-secret"

	svc := NewCalendarService(cfg)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestOAuthConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Google.RedirectDomain = "https://mgke.keller.by"
	cfg.Google.URL = "/google/oauth"
	cfg.Google.OAuth.ClientID = "id123"
	cfg.Google.OAuth.ClientSecret = "secret456"

	svc := NewCalendarService(cfg)
	oauth := svc.OAuthConfig()

	if oauth.ClientID != "id123" {
		t.Errorf("expected client id id123, got %s", oauth.ClientID)
	}
	if oauth.ClientSecret != "secret456" {
		t.Errorf("expected client secret secret456, got %s", oauth.ClientSecret)
	}
	if oauth.RedirectURL != "https://mgke.keller.by/google/oauth" {
		t.Errorf("expected redirect URL, got %s", oauth.RedirectURL)
	}
}
