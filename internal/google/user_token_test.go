package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func stubTokenEndpoint(t *testing.T) *httptest.Server {
	t.Helper()

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh-me" {
			http.Error(w, "unexpected grant", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(stub.Close)

	previous := googleEndpoint
	googleEndpoint = oauth2.Endpoint{AuthURL: stub.URL + "/auth", TokenURL: stub.URL + "/token"}
	t.Cleanup(func() { googleEndpoint = previous })
	return stub
}

func TestTheRefreshTokenIsPersistedWhenItIsUsed(t *testing.T) {
	stubTokenEndpoint(t)

	cfg := &config.Config{}
	cfg.Google.OAuth.ClientID = "client-id"
	cfg.Google.OAuth.ClientSecret = "client-secret"
	service := NewCalendarService(cfg)

	conf := service.oauthConfig()
	source := conf.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: "refresh-me",
		Expiry:       time.Now().Add(-time.Hour),
	})

	var saved []Credentials
	persisting := &persistSource{source: source, save: func(creds Credentials) { saved = append(saved, creds) }}

	token, err := persisting.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token.AccessToken != "fresh-token" {
		t.Fatalf("access token = %q, want the stub token", token.AccessToken)
	}
	if len(saved) != 1 || saved[0].AccessToken != "fresh-token" || saved[0].RefreshToken != "refresh-me" {
		t.Fatalf("saved credentials = %+v, want the refreshed pair", saved)
	}
	if saved[0].Expiry <= time.Now().UnixMilli() {
		t.Fatalf("saved expiry = %d, want a future moment", saved[0].Expiry)
	}
}

func TestAUserClientIsBuiltFromTheStoredCredentials(t *testing.T) {
	service := CalendarService{}

	api, err := service.UserClient(context.Background(), Credentials{RefreshToken: "refresh-me"}, nil)
	if err != nil {
		t.Fatalf("UserClient: %v", err)
	}
	if api == nil || api.service == nil || api.info == nil {
		t.Fatal("UserClient must build both the calendar and the userinfo client")
	}
}

func TestAServiceWithoutConfigStaysOff(t *testing.T) {
	service := &CalendarService{}

	if service.SyncEnabled() {
		t.Fatal("a service without config must not claim that the sync is enabled")
	}
	if service.Configured() {
		t.Fatal("a service without config must not claim that OAuth is configured")
	}
	if conf := service.OAuthConfig(); conf.ClientID != "" || conf.RedirectURL != "" {
		t.Fatalf("OAuthConfig = %+v, want the zero value", conf)
	}
}
