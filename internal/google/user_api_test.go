package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/calendar/v3"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// newUserAPI builds the live wrapper against a stub server that answers the
// Google endpoints with the bodies in routes.
func newUserAPI(t *testing.T, handler http.HandlerFunc) *UserAPI {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(server.Client()), option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatalf("new calendar service: %v", err)
	}
	info, err := oauth2api.NewService(context.Background(),
		option.WithHTTPClient(server.Client()), option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatalf("new oauth service: %v", err)
	}
	return &UserAPI{service: service, info: info}
}

func TestUserAPIEmailReadsUserinfo(t *testing.T) {
	api := newUserAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/userinfo") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"email": "user@example.com"}`))
	})

	email, err := api.Email(context.Background())
	if err != nil {
		t.Fatalf("Email: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", email)
	}
}

func TestUserAPIEmailReportsBrokenUserinfo(t *testing.T) {
	api := newUserAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"code": 403, "message": "denied"}}`))
	})

	if _, err := api.Email(context.Background()); err == nil {
		t.Fatal("a denied userinfo call must be reported")
	}
}

func TestUserAPIListCalendarIDsAcceptsOnlyManagedCalendars(t *testing.T) {
	managed := strings.Repeat("a", 64) + "@group.calendar.google.com"
	api := newUserAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/users/me/calendarList") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items": [
			{"id": "` + managed + `"},
			{"id": "owner@gmail.com"},
			{"id": "short@group.calendar.google.com"}
		]}`))
	})

	ids, err := api.ListCalendarIDs(context.Background())
	if err != nil {
		t.Fatalf("ListCalendarIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != managed {
		t.Fatalf("ids = %v, want only the 64-character calendar", ids)
	}
}

func TestUserAPICreateCalendarReturnsTheID(t *testing.T) {
	var summary string
	api := newUserAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/calendars") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			Summary string `json:"summary"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		summary = payload.Summary
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "fresh-calendar-id"}`))
	})

	id, err := api.CreateCalendar(context.Background(), "Группа 100")
	if err != nil {
		t.Fatalf("CreateCalendar: %v", err)
	}
	if id != "fresh-calendar-id" {
		t.Fatalf("id = %q, want fresh-calendar-id", id)
	}
	if summary != "Группа 100" {
		t.Fatalf("summary = %q, want the requested one", summary)
	}
}

func TestUserAPISetUserRoleNormalisesTheRole(t *testing.T) {
	var roles []string
	api := newUserAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/acl") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			Role  string `json:"role"`
			Scope struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"scope"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		roles = append(roles, payload.Role+"|"+payload.Scope.Type+"|"+payload.Scope.Value)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"role": "writer"}`))
	})

	if err := api.SetUserRole(context.Background(), "cal-1", "friend@example.com", "writer"); err != nil {
		t.Fatalf("SetUserRole(writer): %v", err)
	}
	if err := api.SetUserRole(context.Background(), "cal-1", "friend@example.com", "owner"); err != nil {
		t.Fatalf("SetUserRole(owner): %v", err)
	}

	want := []string{"writer|user|friend@example.com", "reader|user|friend@example.com"}
	if len(roles) != 2 || roles[0] != want[0] || roles[1] != want[1] {
		t.Fatalf("roles = %v, want %v", roles, want)
	}
}
