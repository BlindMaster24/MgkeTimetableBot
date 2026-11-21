package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

func TestExchangeReportsAFailedTokenExchange(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer stub.Close()

	previous := googleEndpoint
	googleEndpoint = oauth2.Endpoint{AuthURL: stub.URL + "/auth", TokenURL: stub.URL + "/token"}
	t.Cleanup(func() { googleEndpoint = previous })

	cfg := &config.Config{}
	cfg.Google.OAuth.ClientID = "client-id"
	cfg.Google.OAuth.ClientSecret = "client-secret"
	service := NewCalendarService(cfg)

	if _, _, err := service.Exchange(context.Background(), "code"); err == nil {
		t.Fatal("a rejected code must be an error")
	}
}

func TestSyncDayNeedsAServiceAccount(t *testing.T) {
	service := NewCalendarService(&config.Config{})

	if err := service.SyncDay(context.Background(), "calendar", "07.09.2026", nil, Schedule{}); err == nil {
		t.Fatal("a missing service account must be reported before any request")
	}
}
