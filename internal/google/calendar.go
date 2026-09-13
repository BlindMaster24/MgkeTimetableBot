package google

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	dateLayout   = "02.01.2006"
	clockLayout  = "15:04"
	timeZoneName = "Europe/Moscow"
)

var moscow = time.FixedZone(timeZoneName, 3*60*60)

type CalendarService struct {
	cfg *config.Config

	mu     sync.Mutex
	client *calendar.Service
}

func NewCalendarService(cfg *config.Config) *CalendarService {
	return &CalendarService{cfg: cfg}
}

func (s *CalendarService) ServiceAccountClient(context.Context) (*calendar.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	ctx := context.Background()
	conf := &jwt.Config{
		Email:      s.cfg.Google.ServiceAccount.ClientEmail,
		PrivateKey: []byte(s.cfg.Google.ServiceAccount.PrivateKey),
		Scopes:     []string{calendar.CalendarScope},
		TokenURL:   "https://oauth2.googleapis.com/token",
	}

	service, err := calendar.NewService(ctx, option.WithHTTPClient(conf.Client(ctx)))
	if err != nil {
		return nil, err
	}
	s.client = service
	return service, nil
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

func (s *CalendarService) SyncEnabled() bool {
	account := s.cfg.Google.ServiceAccount
	return account.ClientEmail != "" && account.PrivateKey != ""
}

type Schedule struct {
	Weekdays [][2][2]string
	Saturday [][2][2]string
}

type DayLesson struct {
	Index       int
	Title       string
	Description string
	Location    string
}

func (s *CalendarService) SyncDay(ctx context.Context, calendarID string, date string, lessons []DayLesson, calls Schedule) error {
	service, err := s.ServiceAccountClient(ctx)
	if err != nil {
		return err
	}

	day, err := time.ParseInLocation(dateLayout, date, moscow)
	if err != nil {
		return fmt.Errorf("parse date %s: %w", date, err)
	}

	if err := clearDay(ctx, service, calendarID, day); err != nil {
		return err
	}

	for _, lesson := range lessons {
		start, end, err := lessonTimes(calls, day, lesson.Index)
		if err != nil {
			return err
		}

		if _, err := service.Events.Insert(calendarID, buildEvent(lesson, start, end)).Context(ctx).Do(); err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}

	return nil
}

func clearDay(ctx context.Context, service *calendar.Service, calendarID string, day time.Time) error {
	events, err := service.Events.List(calendarID).
		TimeMin(day.Format(time.RFC3339)).
		TimeMax(day.Add(24 * time.Hour).Format(time.RFC3339)).
		SingleEvents(true).
		ShowDeleted(false).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("list day events: %w", err)
	}

	for _, event := range events.Items {
		if err := service.Events.Delete(calendarID, event.Id).Context(ctx).Do(); err != nil {
			return fmt.Errorf("delete event %s: %w", event.Id, err)
		}
	}

	return nil
}

func lessonTimes(calls Schedule, day time.Time, index int) (time.Time, time.Time, error) {
	bounds := lessonBounds(calls, day, index)
	if bounds[0][0] == "" || bounds[1][1] == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("no calls schedule for %s", day.Format(dateLayout))
	}

	start, err := parseClock(day, bounds[0][0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseClock(day, bounds[1][1])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		end = start.Add(45 * time.Minute)
	}

	return start, end, nil
}

func lessonBounds(calls Schedule, day time.Time, index int) [2][2]string {
	table := calls.Weekdays
	if day.Weekday() == time.Saturday {
		table = calls.Saturday
	}
	if len(table) == 0 {
		return [2][2]string{}
	}
	if index >= 0 && index < len(table) {
		return table[index]
	}
	return table[len(table)-1]
}

func parseClock(day time.Time, value string) (time.Time, error) {
	parsed, err := time.Parse(clockLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %s: %w", value, err)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, moscow), nil
}

func buildEvent(lesson DayLesson, start, end time.Time) *calendar.Event {
	event := &calendar.Event{
		Summary:     lesson.Title,
		Description: lesson.Description,
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: timeZoneName,
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: timeZoneName,
		},
	}
	if lesson.Location != "" {
		event.Location = lesson.Location
	}
	return event
}
