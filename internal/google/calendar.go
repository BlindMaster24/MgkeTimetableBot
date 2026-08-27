package google

import (
	"context"
	"fmt"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarService struct {
	cfg *config.Config
}

func NewCalendarService(cfg *config.Config) *CalendarService {
	return &CalendarService{cfg: cfg}
}

func (s *CalendarService) ServiceAccountClient(ctx context.Context) (*calendar.Service, error) {
	conf := &jwt.Config{
		Email:      s.cfg.Google.ServiceAccount.ClientEmail,
		PrivateKey: []byte(s.cfg.Google.ServiceAccount.PrivateKey),
		Scopes:     []string{calendar.CalendarScope},
		TokenURL:   "https://oauth2.googleapis.com/token",
	}

	client := conf.Client(ctx)
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (s *CalendarService) OAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID:     s.cfg.Google.OAuth.ClientID,
		ClientSecret: s.cfg.Google.OAuth.ClientSecret,
		RedirectURL:  fmt.Sprintf("%s%s", s.cfg.Google.RedirectDomain, s.cfg.Google.URL),
	}
}

func (s *CalendarService) SyncGroupDay(ctx context.Context, calendarID string, group string, day string, lessons []LessonEvent) error {
	svc, err := s.ServiceAccountClient(ctx)
	if err != nil {
		return err
	}

	t, err := time.Parse("02.01.2006", day)
	if err != nil {
		return fmt.Errorf("parse date %s: %w", day, err)
	}

	for _, lesson := range lessons {
		startHour := 8 + lesson.Index
		start := time.Date(t.Year(), t.Month(), t.Day(), startHour, 0, 0, 0, time.UTC)
		end := start.Add(time.Hour)

		summary := fmt.Sprintf("%d. %s", lesson.Index, lesson.Text)

		event := &calendar.Event{
			Summary:     summary,
			Description: fmt.Sprintf("Группа: %s", group),
			Start: &calendar.EventDateTime{
				DateTime: start.Format(time.RFC3339),
				TimeZone: "Europe/Moscow",
			},
			End: &calendar.EventDateTime{
				DateTime: end.Format(time.RFC3339),
				TimeZone: "Europe/Moscow",
			},
		}

		_, err := svc.Events.Insert(calendarID, event).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}

	return nil
}

type LessonEvent struct {
	Index int
	Text  string
}

func (s *CalendarService) ListCalendars(ctx context.Context) ([]*calendar.CalendarListEntry, error) {
	svc, err := s.ServiceAccountClient(ctx)
	if err != nil {
		return nil, err
	}

	list, err := svc.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}
