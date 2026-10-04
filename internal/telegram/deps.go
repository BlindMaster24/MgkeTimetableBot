package telegram

import (
	"context"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

type botDeps struct {
	cfg          *config.Config
	log          *logger.Logger
	i18n         *i18n.Localizer
	cache        *cache.RaspCache
	archive      archiveStore
	google       googleService
	health       healthSource
	keys         *apikey.Store
	incidents    *health.IncidentLog
	parseFunc    func() error
	noticeDay    func(index int)
	calendarSync func(ctx context.Context) (int, error)
	apiProbe     func(ctx context.Context) []apiprobe.Result
	now          func() time.Time
	startTime    time.Time
	buildInfo    build.Info
}
