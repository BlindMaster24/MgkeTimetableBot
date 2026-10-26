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
)

func Run(ctx context.Context, cfgPath string, buildInfo build.Info) int {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

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
	loc := i18n.New("ru")

	metrics := health.NewTracker(healthThresholds(cfg))
	telegramCounters := health.NewTelegramCounters()

	ctx, cancel := signal.NotifyContext(ctx, stopSignals()...)
	defer cancel()

	log.Info().Strs("signals", signalNames(stopSignals())).Msg("graceful shutdown armed")

	raspCache, err := cache.New(cfg.ResolvedCacheDir())
	if err != nil {
		log.Error().Err(err).Msg("failed to init cache")
		return 1
	}
	raspCache.SetCallsPreferSite(cfg.Parser.Calls == nil || cfg.Parser.Calls.PreferSite)
	log.Info().
		Int("groups", len(raspCache.GetGroups())).
		Int("teachers", len(raspCache.GetTeachers())).
		Msg("cache loaded from disk")

	archivePath := cfg.ResolvedDBPath()
	if adopted, ok := config.AdoptLegacyDBPath(archivePath); ok {
		log.Warn().
			Str("path", absolutePath(adopted)).
			Msg("archive still lives under the legacy sqlite3.db name, rename it to timetable_archive.db")
		archivePath = adopted
	}

	archiveRepo, err := archive.NewWithOptions(archivePath, archive.Options{
		BusyTimeout: cfg.ResolvedSQLiteBusyTimeout(),
		Retries:     cfg.ResolvedSQLiteBusyRetries(),
		OnRetry:     func(int, error) { metrics.ArchiveBusyRetry() },
		OnFailure:   func(error) { metrics.ArchiveBusyFailure() },
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to open archive DB")
		return 1
	}
	defer archiveRepo.Close()
	log.Info().Str("path", absolutePath(archivePath)).Msg("archive DB opened")

	syncArchive := func(tag string) {
		if err := archiveRepo.SyncFromCache(ctx, raspCache.GetGroups(), raspCache.GetTeachers()); err != nil {
			log.Error().Err(err).Str("tag", tag).Msg("archive sync failed")
		} else {
			log.Info().Str("tag", tag).Msg("archive synced from cache")
		}
	}

	syncArchive("startup")

	chatRepo, err := telegrambot.NewChatRepo(cfg.ResolvedChatDBPath(), cfg.ResolvedSQLiteBusyTimeout())
	if err != nil {
		log.Error().Err(err).Msg("failed to open chat DB")
		return 1
	}
	defer chatRepo.Close()
	chatRepo.SetDefaultAccepted(cfg.Accept.Private)

	keyStore := apikey.NewStore(chatRepo.DB(), cfg.EncryptKey)
	if err := keyStore.EnsureSchema(); err != nil {
		log.Error().Err(err).Msg("failed to prepare the api key table")
		return 1
	}
	if !keyStore.Enabled() {
		log.Warn().Msg("encrypt_key is missing or shorter than 32 characters, every /api request will be rejected")
	}

	probeToken := ""
	if keyStore.Enabled() {
		if token, err := keyStore.SystemToken(); err != nil {
			log.Error().Err(err).Msg("failed to prepare the api probe key")
		} else {
			probeToken = token
		}
	}

	apiServer := api.NewServer(raspCache, cfg.HTTP.Port, metrics, buildInfo, keyStore, loc)

	if err := metrics.Restore(chatRepo); err != nil {
		log.Warn().Err(err).Msg("failed to restore health metrics")
	}

	saveMetrics := func(tag string) {
		if err := metrics.Flush(chatRepo); err != nil {
			log.Warn().Err(err).Str("tag", tag).Msg("failed to persist health metrics")
		}
	}

	bot, err := telegrambot.NewBot(cfg, log, loc, chatRepo, raspCache, archiveRepo)
	if err != nil {
		log.Error().Err(err).Msg("failed to create bot")
		return 1
	}
	bot.SetBuildInfo(buildInfo)
	bot.SetHealthSource(metrics)
	bot.SetCounterSink(telegramCounters)
	bot.SetKeyStore(keyStore)
	bot.SetAPIProbeFunc(func(ctx context.Context) []apiprobe.Result {
		return apiprobe.Run(ctx, apiProbeClient, apiProbeBaseURL(cfg), apiprobe.Targets(raspCache), probeToken)
	})

	incidents := health.NewIncidentLog(chatRepo, health.IncidentLimit)
	stamp := health.StartupStamp{Build: buildInfo.Summary(), Config: configStamp(cfgPath)}
	incidents.SetStartupStamp(stamp)
	bot.SetIncidentLog(incidents)

	for _, record := range incidents.NoteStartup(health.ScopeAPI, stamp, func(change health.StartupChange, previous, current health.StartupStamp) string {
		switch change {
		case health.ChangeDeploy:
			return loc.T("ru", "incident_note_deploy", map[string]interface{}{"Current": current.Build, "Previous": previous.Build})
		case health.ChangeConfig:
			return loc.T("ru", "incident_note_config", map[string]interface{}{"Current": current.Config, "Previous": previous.Config})
		}
		return loc.T("ru", "incident_note_restart", map[string]interface{}{"Current": current.Build})
	}) {
		log.Info().Str("key", record.Key).Str("note", record.Note).Msg("open api incident annotated at startup")
	}

	maintenance := newArchiveMaintenance(cfg, archiveRepo, log, metrics)

	bot.SetBackupFunc(func(ctx context.Context) (telegrambot.BackupReport, error) {
		return maintenance.Run(ctx)
	})

	if cfg.MaintenanceEnabled() {
		scheduler := backup.NewScheduler(cfg.ResolvedMaintenanceSchedule(), log, func() {
			if _, err := maintenance.Run(ctx); err != nil {
				log.Error().Err(err).Msg("archive maintenance failed")
			}
		})
		scheduler.Start()
		defer scheduler.Stop()
		log.Info().
			Str("expr", cfg.ResolvedMaintenanceSchedule()).
			Str("backup_dir", cfg.Archive.BackupDir).
			Int("retention_days", cfg.Archive.RetentionDays).
			Msg("archive maintenance armed")
	}

	googleService := google.NewCalendarService(cfg)
	bot.SetGoogleService(googleService)
	calendarSyncEnabled := googleService.SyncEnabled()
	apiServer.SetWebhookStatus(func(ctx context.Context) any { return bot.WebhookStatus(ctx) })
	apiServer.HandleGoogleOAuth(cfg.Google.URL, googleOAuthHandler(googleService, chatRepo, bot, log, loc))

	go func() {
		log.Info().Int("port", cfg.HTTP.Port).Msg("API server starting")
		if err := apiServer.Run(); err != nil {
			log.Error().Err(err).Msg("API server error")
		}
	}()

	adapter := telegrambot.NewEventChatFinder(chatRepo, cfg.Telegram.AdminIDs)

	syncCalendars := func(ctx context.Context, tag string) (int, error) {
		changes := googleDayChanges(raspCache.DrainDayChanges(ctx))
		if !calendarSyncEnabled {
			return 0, nil
		}

		synced := 0
		var failures []error

		days, err := bot.SyncGoogleCalendarChanges(ctx, changes)
		if err != nil {
			metrics.CalendarFailure(err)
			log.Error().Err(err).Str("tag", tag).Msg("google calendar change sync failed")
			failures = append(failures, err)
		} else {
			metrics.CalendarSuccess(days)
		}
		synced += days

		days, err = bot.SyncGoogleCalendars(ctx)
		if err != nil {
			metrics.CalendarFailure(err)
			log.Error().Err(err).Str("tag", tag).Msg("google calendar reconcile failed")
			failures = append(failures, err)
		} else {
			metrics.CalendarSuccess(days)
		}
		synced += days

		return synced, errors.Join(failures...)
	}

	bot.SetCalendarSyncFunc(func(ctx context.Context) (int, error) {
		return syncCalendars(ctx, "manual")
	})

	var eventNotifier *notification.EventNotifier
	schedulerEntries := func() int { return 0 }
	var trackedReminders func() int
	if cfg.Telegram.Noticer {
		eventNotifier = notification.NewEventNotifier(raspCache, cfg, log, bot, adapter)
		eventNotifier.SetLocalizer(loc)
		notifier := eventNotifier
		bot.SetNoticeDayFunc(notifier.CronDayAll)
		scheduler := notification.NewScheduler(cfg, raspCache, log, bot, adapter, metrics, chatRepo)
		scheduler.SetIncidents(incidents)
		scheduler.SetCounterSink(telegramCounters)
		scheduler.Start()
		defer scheduler.Stop()
		schedulerEntries = scheduler.Entries
		trackedReminders = scheduler.TrackedReminders
		log.Info().Msg("notification scheduler started")
	}

	if cfg.Metrics.Enabled {
		apiServer.SetMetricsConfig(api.MetricsConfig{Path: cfg.ResolvedMetricsPath(), Token: cfg.Metrics.Token})
		apiServer.SetMetricsSources(api.MetricsSources{
			Archive:   archiveRepo.Stats,
			Bounds:    archiveRepo.DayIndexBounds,
			Webhook:   newWebhookMetricsSource(bot.WebhookStatus).Get,
			Scheduler: schedulerEntries,
			Reminders: trackedReminders,
			Telegram:  telegramCounters.TelegramStats,
		})
		log.Info().Str("path", cfg.ResolvedMetricsPath()).Msg("prometheus metrics enabled")
	}

	drainEvents := func(tag string) {
		events := raspCache.DrainEvents(ctx)
		if len(events) == 0 {
			return
		}
		log.Info().Int("count", len(events)).Str("tag", tag).Msg("draining cache events")
		if eventNotifier != nil {
			eventNotifier.HandleEvents(events)
		}
	}

	drainEvents("startup")

	parserLoop := parserpkg.LoopConfigFrom(cfg)
	proxy := ""
	if cfg.Parser.Proxy != nil {
		proxy = *cfg.Parser.Proxy
	}
	onParserReport := func(report parserpkg.Report) {
		metrics.ParserReport(report.Source, parserLayoutIssues(report), guardIssues(report))
		bot.RecordParserReport(report)
	}

	fetcher := parserpkg.NewFetcher(log, raspCache, parserpkg.Options{
		Proxy:    proxy,
		Guard:    parserGuard(cfg),
		OnReport: onParserReport,
	})

	parseOnce := func(force bool) error {
		started := time.Now()

		parseErr := fetcher.Timetable(ctx, cfg.Parser.Endpoints.TimetableGroup, cfg.Parser.Endpoints.TimetableTeacher)
		if parserLoop.CallsEnabled && (force || raspCache.CallsDue(started, parserLoop.CallsInterval)) {
			parseErr = errors.Join(parseErr, fetcher.Calls(ctx, cfg.Parser.Endpoints.BellSchedule))
		}
		if force || raspCache.TeamDue(started, parserLoop.TeamInterval) {
			parseErr = errors.Join(parseErr, fetcher.Team(ctx, cfg.Parser.Endpoints.Team))
		}

		raspCache.SetSuccessUpdate(ctx, parseErr == nil)

		if parseErr != nil {
			metrics.ParserFailure(parseErr)
			bot.AddParseLog(false, parseErr.Error())
			if eventNotifier != nil {
				go eventNotifier.ParserError(parseErr)
			}
		} else {
			metrics.ParserSuccess(time.Since(started))
			bot.AddParseLog(true, fmt.Sprintf("groups=%d teachers=%d", len(raspCache.GetGroups()), len(raspCache.GetTeachers())))
			drainEvents("parse")
		}

		go func() {
			syncArchive("parse")
			syncCalendars(context.Background(), "parse")
			saveMetrics("parse")
		}()

		return parseErr
	}

	bot.SetParseFunc(func() error { return parseOnce(true) })

	if err := bot.SetMyCommands(); err != nil {
		log.Warn().Err(err).Msg("failed to set bot commands")
	}

	if cfg.Parser.Enabled {
		go parserpkg.RunLoop(ctx, parserLoop, log, func() error { return parseOnce(false) })
		log.Info().
			Dur("default", parserLoop.Default).
			Dur("activity", parserLoop.ActivityInterval).
			Dur("error", parserLoop.ErrorDelay).
			Ints("activity_hours", parserLoop.Activity[:]).
			Msg("parser loop started")
	}

	if cfg.Parser.Enabled {
		go func() {
			log.Info().Msg("initial parse starting")
			if err := parseOnce(false); err != nil {
				log.Error().Err(err).Msg("initial parse failed")
				return
			}
			log.Info().Int("groups", len(raspCache.GetGroups())).Int("teachers", len(raspCache.GetTeachers())).Msg("initial parse done")
		}()
	}

	log.Info().Msg("bot starting, press Ctrl+C to stop")
	if err := bot.Run(ctx); err != nil {
		log.Error().Err(err).Msg("bot stopped")
	}

	saveMetrics("shutdown")

	if err := raspCache.Save(ctx); err != nil {
		log.Error().Err(err).Msg("failed to save cache")
	}
	log.Info().Msg("shutdown complete")
	_ = log.Close()
	return 0
}

func absolutePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
