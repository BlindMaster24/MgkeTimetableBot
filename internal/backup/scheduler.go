package backup

import (
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	log      *logger.Logger
	schedule string
	task     func()
	started  bool
}

func NewScheduler(schedule string, log *logger.Logger, task func()) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		log:      log,
		schedule: strings.TrimSpace(schedule),
		task:     task,
	}
}

func (s *Scheduler) Start() bool {
	if s.schedule == "" || s.task == nil {
		return false
	}
	if _, err := s.cron.AddFunc(s.schedule, s.task); err != nil {
		s.log.Error().Err(err).Str("expr", s.schedule).Msg("failed to schedule archive maintenance")
		return false
	}

	s.cron.Start()
	s.started = true
	s.log.Info().Str("expr", s.schedule).Msg("archive maintenance scheduled")
	return true
}

func (s *Scheduler) Stop() {
	if !s.started {
		return
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.started = false
}
