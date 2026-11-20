// Package app wires one bot process: configuration, stores, API, bot, scheduler and parser.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/backup"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/google"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	parserpkg "github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

// runtime carries the objects one bot process wires together. The steps live
// in the order they must run: stores, chats, API, bot, background services.
type runtime struct {
	cfg      *config.Config
	cfgPath  string
	build    build.Info
	log      *logger.Logger
	loc      *i18n.Localizer
	metrics  *health.Tracker
	counters *health.TelegramCounters

	cache   *cache.RaspCache
	archive *archive.Repository
	chats   *telegrambot.Repository
	keys    *apikey.Store

	incidents  *health.IncidentLog
	webhook    *webhookStatusSource
	google     *google.CalendarService
	api        *api.Server
	bot        *telegrambot.Bot
	probeToken string

	calendarSyncEnabled bool
	notifier            *notification.EventNotifier
	schedulerEntries    func() int
	trackedReminders    func() int
}

func Run(ctx context.Context, cfgPath string, buildInfo build.Info) int {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	// The timetable lives in the college's zone; pin it before any
	// scheduler or day calculation reads the clock.
	utils.SiteLocation = cfg.Location()
	fmt.Printf("config loaded: %s\n", cfgPath)

	fileCfg := &logger.FileConfig{}
	if cfg.Logging.File.Enabled {
		fileCfg = &logger.FileConfig{
			Enabled:   true,
			Path:      cfg.Logging.File.Path,
			MaxSizeMB: cfg.Logging.File.MaxSizeMB,
			MaxFiles:  cfg.Logging.File.MaxFiles,
		}
	}

	log := logger.New(cfg.Logging.Level, fileCfg)
	log.Info().Str("version", buildInfo.Version).Str("commit", buildInfo.ShortCommit()).Msg("bot starting")

	r := &runtime{
		cfg:      cfg,
		cfgPath:  cfgPath,
		build:    buildInfo,
		log:      log,
		loc:      i18n.New("ru"),
		metrics:  health.NewTracker(healthThresholds(cfg)),
		counters: health.NewTelegramCounters(),
	}

	ctx, cancel := signal.NotifyContext(ctx, stopSignals()...)
	defer cancel()

	log.Info().Strs("signals", signalNames(stopSignals())).Msg("graceful shutdown armed")

	if !r.openStores() {
		return 1
	}
	defer r.close()

	if !r.openChats() {
		return 1
	}
	if !r.startAPI() {
		return 1
	}
	if !r.startBot() {
		return 1
	}

	r.startMaintenance(ctx)
	r.startCalendar()
	r.startNotifier()
	r.configureMetrics()
	r.startParser(ctx)

	return r.serve(ctx)
}

func (r *runtime) close() {
	if r.chats != nil {
		r.chats.Close()
	}
	if r.archive != nil {
		r.archive.Close()
	}
}

// openStores opens the timetable cache and the archive that mirrors it.
func (r *runtime) openStores() bool {
	raspCache, err := cache.New(r.cfg.ResolvedCacheDir())
	if err != nil {
		r.log.Error().Err(err).Msg("failed to init cache")
		return false
	}
	raspCache.SetCallsPreferSite(r.cfg.Parser.Calls == nil || r.cfg.Parser.Calls.PreferSite)
	r.log.Info().
		Int("groups", len(raspCache.GetGroups())).
		Int("teachers", len(raspCache.GetTeachers())).
		Msg("cache loaded from disk")

	archivePath := r.cfg.ResolvedDBPath()
	if adopted, ok := config.AdoptLegacyDBPath(archivePath); ok {
		r.log.Warn().
			Str("path", absolutePath(adopted)).
			Msg("archive still lives under the legacy sqlite3.db name, rename it to timetable_archive.db")
		archivePath = adopted
	}

	archiveRepo, err := archive.NewWithOptions(archivePath, archive.Options{
		BusyTimeout: r.cfg.ResolvedSQLiteBusyTimeout(),
		Retries:     r.cfg.ResolvedSQLiteBusyRetries(),
		OnRetry:     func(int, error) { r.metrics.ArchiveBusyRetry() },
		OnFailure:   func(error) { r.metrics.ArchiveBusyFailure() },
	})
	if err != nil {
		r.log.Error().Err(err).Msg("failed to open archive DB")
		return false
	}

	r.cache = raspCache
	r.archive = archiveRepo
	r.log.Info().Str("path", absolutePath(archivePath)).Msg("archive DB opened")

	r.syncArchive(context.Background(), "startup")
	return true
}

// openChats opens the chat database and prepares the API keys.
func (r *runtime) openChats() bool {
	chatRepo, err := telegrambot.NewChatRepo(r.cfg.ResolvedChatDBPath(), r.cfg.ResolvedSQLiteBusyTimeout())
	if err != nil {
		r.log.Error().Err(err).Msg("failed to open chat DB")
		return false
	}
	chatRepo.SetDefaultAccepted(r.cfg.Accept.Private)
	r.chats = chatRepo

	keyStore := apikey.NewStore(chatRepo.DB(), r.cfg.EncryptKey)
	if err := keyStore.EnsureSchema(); err != nil {
		r.log.Error().Err(err).Msg("failed to prepare the api key table")
		return false
	}
	if !keyStore.Enabled() {
		r.log.Warn().Msg("encrypt_key is missing or shorter than 32 characters, every /api request will be rejected")
	}
	r.keys = keyStore

	if keyStore.Enabled() {
		if token, err := keyStore.SystemToken(); err != nil {
			r.log.Error().Err(err).Msg("failed to prepare the api probe key")
		} else {
			r.probeToken = token
		}
	}
	return true
}

// startAPI creates the HTTP server and restores the persisted health state.
func (r *runtime) startAPI() bool {
	r.api = api.NewServer(r.cache, r.cfg.HTTP.Port, r.metrics, r.build, r.keys, r.loc)

	if err := r.metrics.Restore(r.chats); err != nil {
		r.log.Warn().Err(err).Msg("failed to restore health metrics")
	}
	return true
}

// startBot creates the bot and gives it every source it reports through.
func (r *runtime) startBot() bool {
	bot, err := telegrambot.NewBot(r.cfg, r.log, r.loc, r.chats, r.cache, r.archive)
	if err != nil {
		r.log.Error().Err(err).Msg("failed to create bot")
		return false
	}
	bot.SetBuildInfo(r.build)
	bot.SetHealthSource(r.metrics)
	bot.SetCounterSink(r.counters)
	bot.SetKeyStore(r.keys)
	bot.SetAPIProbeFunc(func(ctx context.Context) []apiprobe.Result {
		return apiprobe.Run(ctx, apiProbeClient, apiProbeBaseURL(r.cfg), apiprobe.Targets(r.cache), r.probeToken)
	})

	incidents := health.NewIncidentLog(r.chats, health.IncidentLimit)
	stamp := health.StartupStamp{Build: r.build.Summary(), Config: configStamp(r.cfgPath)}
	incidents.SetStartupStamp(stamp)
	incidents.SetErrorHandler(func(err error) {
		r.log.Warn().Err(err).Msg("failed to persist the incident history")
	})
	bot.SetIncidentLog(incidents)
	bot.SetShutdownFunc(func() {
		r.saveMetrics("restart")
		if err := r.cache.Save(context.Background()); err != nil {
			r.log.Error().Err(err).Msg("failed to save the cache before a restart")
		}
	})

	for _, record := range incidents.NoteStartup(health.ScopeAPI, stamp, func(change health.StartupChange, previous, current health.StartupStamp) string {
		switch change {
		case health.ChangeDeploy:
			return r.loc.T("ru", "incident_note_deploy", map[string]interface{}{"Current": current.Build, "Previous": previous.Build})
		case health.ChangeConfig:
			return r.loc.T("ru", "incident_note_config", map[string]interface{}{"Current": current.Config, "Previous": previous.Config})
		}
		return r.loc.T("ru", "incident_note_restart", map[string]interface{}{"Current": current.Build})
	}) {
		r.log.Info().Str("key", record.Key).Str("note", record.Note).Msg("open api incident annotated at startup")
	}

	r.bot = bot
	r.incidents = incidents
	return true
}

// startMaintenance arms the archive backup and prune schedule.
func (r *runtime) startMaintenance(ctx context.Context) {
	maintenance := newArchiveMaintenance(r.cfg, r.archive, r.log, r.metrics)
	r.bot.SetBackupFunc(func(ctx context.Context) (telegrambot.BackupReport, error) {
		return maintenance.Run(ctx)
	})

	if !r.cfg.MaintenanceEnabled() {
		return
	}

	scheduler := backup.NewScheduler(r.cfg.ResolvedMaintenanceSchedule(), r.log, func() {
		if _, err := maintenance.Run(ctx); err != nil {
			r.log.Error().Err(err).Msg("archive maintenance failed")
		}
	})
	scheduler.Start()
	r.log.Info().
		Str("expr", r.cfg.ResolvedMaintenanceSchedule()).
		Str("backup_dir", r.cfg.Archive.BackupDir).
		Int("retention_days", r.cfg.Archive.RetentionDays).
		Msg("archive maintenance armed")
}

// startCalendar wires Google Calendar, the webhook status and the HTTP API.
func (r *runtime) startCalendar() {
	r.google = google.NewCalendarService(r.cfg)
	r.bot.SetGoogleService(r.google)
	r.calendarSyncEnabled = r.google.SyncEnabled()

	r.webhook = newWebhookStatusSource(r.bot.WebhookStatus)
	r.api.SetWebhookStatus(func(ctx context.Context) any { return r.webhook.Get(ctx) })
	r.api.HandleGoogleOAuth(r.cfg.Google.URL, googleOAuthHandler(r.google, r.chats, r.bot, r.log, r.loc))

	go func() {
		r.log.Info().Int("port", r.cfg.HTTP.Port).Msg("API server starting")
		if err := r.api.Run(); err != nil {
			r.log.Error().Err(err).Msg("API server error")
		}
	}()

	r.bot.SetCalendarSyncFunc(func(ctx context.Context) (int, error) {
		return r.syncCalendars(ctx, "manual")
	})
}

// startNotifier arms the event notifier and the notification scheduler.
func (r *runtime) startNotifier() {
	r.schedulerEntries = func() int { return 0 }

	if !r.cfg.Telegram.Noticer {
		return
	}

	adapter := telegrambot.NewEventChatFinder(r.chats, r.cfg.Telegram.AdminIDs)
	r.notifier = notification.NewEventNotifier(r.cache, r.cfg, r.log, r.bot, adapter)
	r.notifier.SetLocalizer(r.loc)
	r.bot.SetNoticeDayFunc(r.notifier.CronDayAll)

	scheduler := notification.NewScheduler(r.cfg, r.cache, r.log, r.bot, adapter, r.metrics, r.chats)
	scheduler.SetIncidents(r.incidents)
	scheduler.SetCounterSink(r.counters)
	scheduler.Start()
	r.schedulerEntries = scheduler.Entries
	r.trackedReminders = scheduler.TrackedReminders
	r.log.Info().Msg("notification scheduler started")
}

// configureMetrics publishes the /metrics sources when Prometheus output is on.
func (r *runtime) configureMetrics() {
	if !r.cfg.Metrics.Enabled {
		return
	}

	r.api.SetMetricsConfig(api.MetricsConfig{Path: r.cfg.ResolvedMetricsPath(), Token: r.cfg.Metrics.Token})
	r.api.SetMetricsSources(api.MetricsSources{
		Archive:   r.archive.Stats,
		Bounds:    r.archive.DayIndexBounds,
		Webhook:   func(ctx context.Context) api.WebhookMetrics { return webhookMetrics(r.webhook.Get(ctx)) },
		Scheduler: r.schedulerEntries,
		Reminders: r.trackedReminders,
		Telegram:  r.counters.TelegramStats,
	})
	r.log.Info().Str("path", r.cfg.ResolvedMetricsPath()).Msg("prometheus metrics enabled")
}

// startParser wires the fetcher and starts the parse loop.

// parseProbe is the part of the fetcher the parse cycle needs. It exists so
// the cycle (due checks, success flag, logs, follow-up work) can be tested
// without the network.
type parseProbe interface {
	Timetable(ctx context.Context, groupURL, teacherURL string) error
	Calls(ctx context.Context, url string) error
	Team(ctx context.Context, urls []string) error
}

// parseOnce runs one parse cycle: the timetable always, calls and team by
// their intervals (or all three when force is set), then records the result
// and refreshes the derived stores in the background.
func (r *runtime) parseOnce(ctx context.Context, probe parseProbe, loop parserpkg.LoopConfig, force bool) error {
	started := time.Now()

	parseErr := probe.Timetable(ctx, r.cfg.Parser.Endpoints.TimetableGroup, r.cfg.Parser.Endpoints.TimetableTeacher)
	if loop.CallsEnabled && (force || r.cache.CallsDue(started, loop.CallsInterval)) {
		parseErr = errors.Join(parseErr, probe.Calls(ctx, r.cfg.Parser.Endpoints.BellSchedule))
	}
	if force || r.cache.TeamDue(started, loop.TeamInterval) {
		parseErr = errors.Join(parseErr, probe.Team(ctx, r.cfg.Parser.Endpoints.Team))
	}

	r.cache.SetSuccessUpdate(ctx, parseErr == nil)

	if parseErr != nil {
		r.metrics.ParserFailure(parseErr)
		r.bot.AddParseLog(false, parseErr.Error())
		if r.notifier != nil {
			go r.notifier.ParserError(parseErr)
		}
	} else {
		r.metrics.ParserSuccess(time.Since(started))
		r.bot.AddParseLog(true, fmt.Sprintf("groups=%d teachers=%d", len(r.cache.GetGroups()), len(r.cache.GetTeachers())))
		r.drainEvents(ctx, "parse")
	}

	go r.afterParse(ctx)

	return parseErr
}

// afterParse refreshes everything that is derived from the cache. The stores
// are optional so the cycle can run in tests without them.
func (r *runtime) afterParse(ctx context.Context) {
	if r.archive != nil {
		r.syncArchive(ctx, "parse")
	}
	if r.google != nil {
		if _, err := r.syncCalendars(ctx, "parse"); err != nil {
			r.log.Error().Err(err).Msg("calendar sync failed")
		}
	}
	if r.metrics != nil {
		r.saveMetrics("parse")
	}
}

func (r *runtime) startParser(ctx context.Context) {
	r.drainEvents(ctx, "startup")

	parserLoop := parserpkg.LoopConfigFrom(r.cfg)
	proxy := ""
	if r.cfg.Parser.Proxy != nil {
		proxy = *r.cfg.Parser.Proxy
	}

	onParserReport := func(report parserpkg.Report) {
		r.metrics.ParserReport(report.Source, parserLayoutIssues(report), guardIssues(report))
		r.bot.RecordParserReport(report)
	}

	fetcher := parserpkg.NewFetcher(r.log, r.cache, parserpkg.Options{
		Proxy:    proxy,
		Guard:    parserGuard(r.cfg),
		OnReport: onParserReport,
	})

	parseOnce := func(force bool) error { return r.parseOnce(ctx, fetcher, parserLoop, force) }

	r.bot.SetParseFunc(func() error { return parseOnce(true) })

	if err := r.bot.SetMyCommands(); err != nil {
		r.log.Error().Err(err).Msg("failed to set bot commands")
	}

	if !r.cfg.Parser.Enabled {
		return
	}

	go parserpkg.RunLoop(ctx, parserLoop, r.log, func() error { return parseOnce(false) })
	r.log.Info().
		Dur("default", parserLoop.Default).
		Dur("activity", parserLoop.ActivityInterval).
		Dur("error", parserLoop.ErrorDelay).
		Ints("activity_hours", parserLoop.Activity[:]).
		Msg("parser loop started")

	go func() {
		r.log.Info().Msg("initial parse starting")
		if err := parseOnce(false); err != nil {
			r.log.Error().Err(err).Msg("initial parse failed")
			return
		}
		r.log.Info().Int("groups", len(r.cache.GetGroups())).Int("teachers", len(r.cache.GetTeachers())).Msg("initial parse done")
	}()
}

// serve runs the bot until the signal context ends and saves the state.
func (r *runtime) serve(ctx context.Context) int {
	r.log.Info().Msg("bot starting, press Ctrl+C to stop")
	if err := r.bot.Run(ctx); err != nil {
		r.log.Error().Err(err).Msg("bot stopped")
	}

	r.saveMetrics("shutdown")

	if err := r.cache.Save(ctx); err != nil {
		r.log.Error().Err(err).Msg("failed to save cache")
	}
	r.log.Info().Msg("shutdown complete")
	_ = r.log.Close()
	return 0
}

func (r *runtime) syncArchive(ctx context.Context, tag string) {
	if err := r.archive.SyncFromCache(ctx, r.cache.GetGroups(), r.cache.GetTeachers()); err != nil {
		r.log.Error().Err(err).Str("tag", tag).Msg("archive sync failed")
	} else {
		r.log.Info().Str("tag", tag).Msg("archive synced from cache")
	}
}

func (r *runtime) saveMetrics(tag string) {
	if err := r.metrics.Flush(r.chats); err != nil {
		r.log.Warn().Err(err).Str("tag", tag).Msg("failed to persist health metrics")
	}
}

func (r *runtime) drainEvents(ctx context.Context, tag string) {
	events := r.cache.DrainEvents(ctx)
	if len(events) == 0 {
		return
	}
	r.log.Info().Int("count", len(events)).Str("tag", tag).Msg("draining cache events")
	if r.notifier != nil {
		r.notifier.HandleEvents(events)
	}
}

func (r *runtime) syncCalendars(ctx context.Context, tag string) (int, error) {
	changes := googleDayChanges(r.cache.DrainDayChanges(ctx))
	if !r.calendarSyncEnabled {
		return 0, nil
	}

	synced := 0
	var failures []error

	days, err := r.bot.SyncGoogleCalendarChanges(ctx, changes)
	if err != nil {
		r.metrics.CalendarFailure(err)
		r.log.Error().Err(err).Str("tag", tag).Msg("google calendar change sync failed")
		failures = append(failures, err)
	} else {
		r.metrics.CalendarSuccess(days)
	}
	synced += days

	days, err = r.bot.SyncGoogleCalendars(ctx)
	if err != nil {
		r.metrics.CalendarFailure(err)
		r.log.Error().Err(err).Str("tag", tag).Msg("google calendar reconcile failed")
		failures = append(failures, err)
	} else {
		r.metrics.CalendarSuccess(days)
	}
	synced += days

	return synced, errors.Join(failures...)
}

func absolutePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
