package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func TestOAuthHandlerRejectsAMissingCode(t *testing.T) {
	loc := i18n.New("ru")
	handler := googleOAuthHandler(nil, nil, nil, logger.New("error", nil), loc)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/oauth", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), loc.T("ru", "oauth_missing_code", nil)) {
		t.Fatalf("body = %q, want the missing code hint", rec.Body.String())
	}
}

func TestOAuthHandlerRejectsABrokenState(t *testing.T) {
	loc := i18n.New("ru")
	handler := googleOAuthHandler(nil, nil, nil, logger.New("error", nil), loc)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/oauth?code=abc&state=not-base64", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), loc.T("ru", "oauth_missing_state", nil)) {
		t.Fatalf("body = %q, want the broken state hint", rec.Body.String())
	}
}
