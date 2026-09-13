package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron          *cron.Cron
	location      *time.Location
	cfg           *config.Config
	cache         *cache.RaspCache
	log           *logger.Logger
	sender        EventSender
	chats         EventChatFinder
	notifier      *EventNotifier
	healthTracker *health.Tracker
	health        *HealthNotifier
	store         health.StateStore
	incidents     *health.IncidentLog
}

func NewScheduler(cfg *config.Config, c *cache.RaspCache, log *logger.Logger, sender EventSender, chats EventChatFinder, tracker *health.Tracker, store health.StateStore) *Scheduler {
	return &Scheduler{
		cron:          cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		location:      time.Local,
		cfg:           cfg,
		cache:         c,
		log:           log,
		sender:        sender,
		chats:         chats,
		healthTracker: tracker,
		store:         store,
	}
}

func (s *Scheduler) Location() *time.Location {
	return s.location
}

func (s *Scheduler) SetIncidents(incidents *health.IncidentLog) {
	s.incidents = incidents
	if s.health != nil {
		s.health.SetIncidents(incidents)
	}
}

func (s *Scheduler) Start() {
	s.notifier = NewEventNotifier(s.cache, s.cfg, s.log, s.sender, s.chats)

	s.registerSlots(s.cfg.Timetable.Weekdays, "1-5")
	s.registerSlots(s.cfg.Timetable.Saturday, "6")
	s.registerHealthCheck()

	s.cron.Start()
}

func (s *Scheduler) registerHealthCheck() {
	if s.healthTracker == nil || (s.cfg.Health != nil && s.cfg.Health.Disabled) {
		return
	}

	minutes := healthCheckMinutes(s.cfg.Health)
	cooldown := time.Duration(0)
	if s.cfg.Health != nil {
		cooldown = time.Duration(s.cfg.Health.CooldownMinutes) * time.Minute
	}

	s.health = NewHealthNotifier(s.healthTracker, s.log, s.sender, s.chats, cooldown, s.store)
	s.health.SetIncidents(s.incidents)
	expr := fmt.Sprintf("0 */%d * * * *", minutes)
	if _, err := s.cron.AddFunc(expr, s.health.Check); err != nil {
		s.log.Error().Err(err).Msg("failed to schedule health check")
	}
}

func healthCheckMinutes(cfg *config.HealthConfig) int {
	if cfg == nil || cfg.CheckMinutes <= 0 {
		return 1
	}
	return cfg.CheckMinutes
}

func (s *Scheduler) registerSlots(slots [][2][2]string, weekRange string) {
	if len(slots) == 0 {
		return
	}

	for index, times := range slots {
		cronExpr, ok := slotCronExpr(times[1][1], weekRange)
		if !ok {
			continue
		}

		idx := index
		_, err := s.cron.AddFunc(cronExpr, func() {
			s.notifier.CronDay(cache.KindGroups, idx, false)
			s.notifier.CronDay(cache.KindTeachers, idx, false)
		})
		if err != nil {
			s.log.Error().Err(err).Str("expr", cronExpr).Msg("failed to schedule notification")
			continue
		}
		s.log.Info().Str("expr", cronExpr).Int("slot", idx).Msg("notification scheduled")
	}
}

func slotCronExpr(endTime, weekRange string) (string, bool) {
	parts := strings.SplitN(endTime, ":", 2)
	if len(parts) != 2 || !isClockNumber(parts[0]) || !isClockNumber(parts[1]) {
		return "", false
	}
	return "0 " + parts[1] + " " + parts[0] + " * * " + weekRange, true
}

func isClockNumber(value string) bool {
	if value == "" {
		return false
	}
	for _, symbol := range value {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return true
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
