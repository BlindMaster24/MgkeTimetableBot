package google

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestSyncEnabledRequiresBothServiceAccountParts(t *testing.T) {
	cases := []struct {
		name  string
		email string
		key   string
		want  bool
	}{
		{"both", "bot@project.iam.gserviceaccount.com", "-----BEGIN KEY-----", true},
		{"email only", "bot@project.iam.gserviceaccount.com", "", false},
		{"key only", "", "-----BEGIN KEY-----", false},
		{"neither", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Google.ServiceAccount.ClientEmail = tc.email
			cfg.Google.ServiceAccount.PrivateKey = tc.key
			if got := NewCalendarService(cfg).SyncEnabled(); got != tc.want {
				t.Fatalf("SyncEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOAuthConfigAndAuthURLFollowTheConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.Google.OAuth.ClientID = "client-id"
	cfg.Google.OAuth.ClientSecret = "client-secret"
	cfg.Google.RedirectDomain = "https://bot.example"
	cfg.Google.URL = "/oauth/callback"

	service := NewCalendarService(cfg)
	conf := service.OAuthConfig()
	if conf.ClientID != "client-id" || conf.ClientSecret != "client-secret" {
		t.Fatalf("OAuthConfig() = %+v", conf)
	}
	if conf.RedirectURL != "https://bot.example/oauth/callback" {
		t.Fatalf("RedirectURL = %q", conf.RedirectURL)
	}
	if !service.Configured() {
		t.Fatal("a client id and a secret must count as configured")
	}

	url := service.AuthURL("state-123")
	for _, want := range []string{"accounts.google.com", "client_id=client-id", "state=state-123", "access_type=offline"} {
		if !strings.Contains(url, want) {
			t.Fatalf("AuthURL misses %q: %s", want, url)
		}
	}

	empty := NewCalendarService(&config.Config{})
	if empty.Configured() {
		t.Fatal("an empty config must not be configured")
	}
}

func TestServiceAccountClientIsBuiltOnce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Google.ServiceAccount.ClientEmail = "bot@project.iam.gserviceaccount.com"
	cfg.Google.ServiceAccount.PrivateKey = "-----BEGIN PRIVATE KEY-----\nnot-a-real-key\n-----END PRIVATE KEY-----"

	service := NewCalendarService(cfg)
	first, err := service.ServiceAccountClient(context.Background())
	if err != nil {
		t.Fatalf("building the client must not sign anything yet: %v", err)
	}
	second, err := service.ServiceAccountClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("the service account client must be built once and reused")
	}
}
