package google

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const userInfoScope = "https://www.googleapis.com/auth/userinfo.email"

type Credentials struct {
	RefreshToken string
	AccessToken  string
	Expiry       int64
}

type CalendarItem struct {
	Type       string
	Value      string
	CalendarID string
}

type UserAPI struct {
	service *calendar.Service
	info    *oauth2api.Service
}

type persistSource struct {
	source oauth2.TokenSource
	save   func(Credentials)
}

func (p *persistSource) Token() (*oauth2.Token, error) {
	token, err := p.source.Token()
	if err != nil {
		return nil, err
	}
	if p.save != nil {
		p.save(Credentials{
			RefreshToken: token.RefreshToken,
			AccessToken:  token.AccessToken,
			Expiry:       token.Expiry.UnixMilli(),
		})
	}
	return token, nil
}

func (s *CalendarService) oauthConfig() *oauth2.Config {
	conf := s.OAuthConfig()
	return &oauth2.Config{
		ClientID:     conf.ClientID,
		ClientSecret: conf.ClientSecret,
		RedirectURL:  strings.TrimRight(conf.RedirectURL, "/"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{calendar.CalendarScope, userInfoScope},
	}
}

func (s *CalendarService) Configured() bool {
	conf := s.OAuthConfig()
	return conf.ClientID != "" && conf.ClientSecret != ""
}

func (s *CalendarService) AuthURL(state string) string {
	return s.oauthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *CalendarService) Exchange(ctx context.Context, code string) (Credentials, string, error) {
	conf := s.oauthConfig()
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		return Credentials{}, "", err
	}

	client := conf.Client(ctx, token)
	email, err := userEmail(ctx, client)
	if err != nil {
		return Credentials{}, "", err
	}

	return Credentials{
		RefreshToken: token.RefreshToken,
		AccessToken:  token.AccessToken,
		Expiry:       token.Expiry.UnixMilli(),
	}, email, nil
}

func userEmail(ctx context.Context, client *http.Client) (string, error) {
	service, err := oauth2api.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", err
	}
	info, err := service.Userinfo.Get().Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", errors.New("email not provided")
	}
	return info.Email, nil
}

func (s *CalendarService) UserClient(ctx context.Context, creds Credentials, save func(Credentials)) (*UserAPI, error) {
	conf := s.oauthConfig()
	source := conf.TokenSource(ctx, &oauth2.Token{
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		Expiry:       time.UnixMilli(creds.Expiry),
	})
	client := oauth2.NewClient(ctx, &persistSource{source: source, save: save})

	calendarService, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	infoService, err := oauth2api.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &UserAPI{service: calendarService, info: infoService}, nil
}

func (a *UserAPI) Email(ctx context.Context) (string, error) {
	info, err := a.info.Userinfo.Get().Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return info.Email, nil
}

func (a *UserAPI) ListCalendarIDs(ctx context.Context) ([]string, error) {
	list, err := a.service.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var out []string
	for _, entry := range list.Items {
		if len(strings.Split(entry.Id, "@")[0]) == 64 {
			out = append(out, entry.Id)
		}
	}
	return out, nil
}

func (a *UserAPI) CreateCalendar(ctx context.Context, summary string) (string, error) {
	created, err := a.service.Calendars.Insert(&calendar.Calendar{Summary: summary}).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return created.Id, nil
}

func (a *UserAPI) AddCalendarToUser(ctx context.Context, calendarID string) error {
	_, err := a.service.CalendarList.Insert(&calendar.CalendarListEntry{Id: calendarID}).Context(ctx).Do()
	return err
}

func (a *UserAPI) RemoveCalendarFromUser(ctx context.Context, calendarID string) error {
	return a.service.CalendarList.Delete(calendarID).Context(ctx).Do()
}

func (a *UserAPI) SetUserRole(ctx context.Context, calendarID, email, role string) error {
	if role != "writer" {
		role = "reader"
	}
	rule := &calendar.AclRule{
		Role:  role,
		Scope: &calendar.AclRuleScope{Type: "user", Value: email},
	}
	_, err := a.service.Acl.Insert(calendarID, rule).Context(ctx).Do()
	return err
}
