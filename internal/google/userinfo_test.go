package google

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type stubTransport struct {
	body   string
	status int
	gotURL *string
}

func (s stubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if s.gotURL != nil {
		*s.gotURL = request.URL.String()
	}
	status := s.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    request,
	}, nil
}

func TestUserEmailReadsTheUserInfo(t *testing.T) {
	var called string
	client := &http.Client{Transport: stubTransport{body: `{"email":"bot@example.com"}`, gotURL: &called}}

	email, err := userEmail(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if email != "bot@example.com" {
		t.Fatalf("email = %q", email)
	}
	if !strings.Contains(called, "userinfo") {
		t.Fatalf("the user info endpoint was not called: %q", called)
	}
}

func TestUserEmailRejectsAnAnswerWithoutEmail(t *testing.T) {
	client := &http.Client{Transport: stubTransport{body: `{"sub":"42"}`}}

	if _, err := userEmail(context.Background(), client); err == nil || !strings.Contains(err.Error(), "email not provided") {
		t.Fatalf("an answer without an email must fail, got %v", err)
	}
}

func TestPersistSourceSavesTheFreshToken(t *testing.T) {
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	source := &persistSource{
		source: oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken:  "access",
			RefreshToken: "refresh",
			Expiry:       expiry,
		}),
	}
	var saved []Credentials
	source.save = func(credentials Credentials) { saved = append(saved, credentials) }

	token, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" {
		t.Fatalf("token = %+v", token)
	}
	if len(saved) != 1 {
		t.Fatalf("saved = %+v, want one update", saved)
	}
	if saved[0].AccessToken != "access" || saved[0].RefreshToken != "refresh" || saved[0].Expiry != expiry.UnixMilli() {
		t.Fatalf("saved credentials = %+v", saved[0])
	}
}

func TestPersistSourceWithoutASaverJustReturnsTheToken(t *testing.T) {
	source := &persistSource{source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "access"})}

	token, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" {
		t.Fatalf("token = %+v", token)
	}
}
