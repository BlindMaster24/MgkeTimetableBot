package notification

import (
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	cfg      *config.Config
	cache    *cache.RaspCache
	log      *logger.Logger
	sender   EventSender
	chats    EventChatFinder
	notifier *EventNotifier
}

func NewScheduler(cfg *config.Config, c *cache.RaspCache, log *logger.Logger, sender EventSender, chats EventChatFinder) *Scheduler {
	return &Scheduler{
		cron:  cron.New(cron.WithSeconds()),
		cfg:   cfg,
		cache: c,
		log:   log,
	}
}

func (s *Scheduler) Start() {
	s.notifier = NewEventNotifier(s.cache, s.cfg, s.log, s.sender, s.chats)

	s.registerSlots(s.cfg.Timetable.Weekdays, "1-5")
	s.registerSlots(s.cfg.Timetable.Saturday, "6")

	s.cron.Start()
}

func (s *Scheduler) registerSlots(slots [][2][2]string, weekRange string) {
	if len(slots) == 0 {
		return
	}

	for index, times := range slots {
		endTime := times[1][1]
		if endTime == "" {
			continue
		}

		parts := strings.SplitN(endTime, ":", 2)
		if len(parts) != 2 {
			continue
		}
		hour, min := parts[0], parts[1]

		cronExpr := "0 " + min + " " + hour + " * * " + weekRange
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

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
