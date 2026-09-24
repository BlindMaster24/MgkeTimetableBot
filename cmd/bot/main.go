package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/apiprobe"
	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
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

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"

	apiProbeClient = &http.Client{Timeout: apiprobe.DefaultTimeout}
)

func main() {
	cfgPath := flag.String("config", "", "path to config file (default: configs/config.yaml)")
	flag.Parse()

	if *cfgPath == "" {
		if env := os.Getenv("CONFIG_PATH"); env != "" {
			cfgPath = &env
		} else {
			defaultPath := "configs/config.yaml"
			cfgPath = &defaultPath
		}
	}

	if _, err := os.Stat(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "config file not found: %s\n", *cfgPath)
		os.Exit(1)
	}

	cfg, err := config.LoadWithEnv(*cfgPath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config %s: %v\n", *cfgPath, err)
		os.Exit(1)
	}

	fmt.Printf("config loaded: %s\n", *cfgPath)

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
	buildInfo := build.New(version, commit, date)
	log.Info().Str("version", buildInfo.Version).Str("commit", buildInfo.ShortCommit()).Msg("bot starting")
	loc := i18n.New("ru")

	metrics := health.NewTracker(healthThresholds(cfg))

	stop := stopSignals()

	ctx, cancel := signal.NotifyContext(context.Background(), stop...)
	defer cancel()

	interrupts := make(chan os.Signal, len(stop))
	signal.Notify(interrupts, stop...)
	defer signal.Stop(interrupts)

	log.Info().Strs("signals", signalNames(stop)).Msg("graceful shutdown armed")

	go func() {
		sig := <-interrupts
		log.Info().Str("signal", sig.String()).Msg("stop signal received, shutting down")
		<-interrupts
		log.Warn().Msg("second stop signal received, exiting now")
		os.Exit(1)
	}()

	raspCache, err := cache.New(cfg.ResolvedCacheDir())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init cache")
	}
	raspCache.SetCallsPreferSite(cfg.Parser.Calls == nil || cfg.Parser.Calls.PreferSite)
	log.Info().
		Int("groups", len(raspCache.GetGroups())).
		Int("teachers", len(raspCache.GetTeachers())).
		Msg("cache loaded from disk")

	archiveRepo, err := archive.New(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open archive DB")
	}
	defer archiveRepo.Close()
	log.Info().Msg("archive DB opened")

	syncArchive := func(tag string) {
		if err := archiveRepo.SyncFromCache(raspCache.GetGroups(), raspCache.GetTeachers()); err != nil {
			log.Error().Err(err).Str("tag", tag).Msg("archive sync failed")
		} else {
			log.Info().Str("tag", tag).Msg("archive synced from cache")
		}
	}

	syncArchive("startup")

	chatRepo, err := telegrambot.NewChatRepo(cfg.ResolvedChatDBPath())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open chat DB")
	}
	defer chatRepo.Close()
	chatRepo.SetDefaultAccepted(cfg.Accept.Private)

	keyStore := apikey.NewStore(chatRepo.DB(), cfg.EncryptKey)
	if err := keyStore.EnsureSchema(); err != nil {
		log.Fatal().Err(err).Msg("failed to prepare the api key table")
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
		log.Fatal().Err(err).Msg("failed to create bot")
	}
	bot.SetBuildInfo(buildInfo)
	bot.SetHealthSource(metrics)
	bot.SetKeyStore(keyStore)
	bot.SetAPIProbeFunc(func(ctx context.Context) []apiprobe.Result {
		return apiprobe.Run(ctx, apiProbeClient, apiProbeBaseURL(cfg), apiprobe.Targets(raspCache), probeToken)
	})

	incidents := health.NewIncidentLog(chatRepo, health.IncidentLimit)
	stamp := health.StartupStamp{Build: buildInfo.Summary(), Config: configStamp(*cfgPath)}
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

	googleService := google.NewCalendarService(cfg)
	bot.SetGoogleService(googleService)
	calendarSyncEnabled := googleService.SyncEnabled()
	apiServer.SetWebhookStatus(func(ctx context.Context) any { return bot.WebhookStatus(ctx) })
	apiServer.HandleGoogleOAuth(cfg.Google.URL, googleOAuthHandler(cfg, googleService, chatRepo, bot, log))

	go func() {
		log.Info().Int("port", cfg.HTTP.Port).Msg("API server starting")
		if err := apiServer.Run(); err != nil {
			log.Error().Err(err).Msg("API server error")
		}
	}()

	adapter := telegrambot.NewEventChatFinder(chatRepo, cfg.Telegram.AdminIDs)

	syncCalendars := func(ctx context.Context, tag string) (int, error) {
		changes := googleDayChanges(raspCache.DrainDayChanges())
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
	if cfg.Telegram.Noticer {
		eventNotifier = notification.NewEventNotifier(raspCache, cfg, log, bot, adapter)
		notifier := eventNotifier
		bot.SetNoticeDayFunc(notifier.CronDayAll)
		scheduler := notification.NewScheduler(cfg, raspCache, log, bot, adapter, metrics, chatRepo)
		scheduler.SetIncidents(incidents)
		scheduler.Start()
		defer scheduler.Stop()
		log.Info().Msg("notification scheduler started")
	}

	drainEvents := func(tag string) {
		events := raspCache.DrainEvents()
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

		parseErr := fetcher.Timetable(cfg.Parser.Endpoints.TimetableGroup, cfg.Parser.Endpoints.TimetableTeacher)
		if parserLoop.CallsEnabled && (force || raspCache.CallsDue(started, parserLoop.CallsInterval)) {
			parseErr = errors.Join(parseErr, fetcher.Calls(cfg.Parser.Endpoints.BellSchedule))
		}
		if force || raspCache.TeamDue(started, parserLoop.TeamInterval) {
			parseErr = errors.Join(parseErr, fetcher.Team(cfg.Parser.Endpoints.Team))
		}

		raspCache.SetSuccessUpdate(parseErr == nil)

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

	if err := raspCache.Save(); err != nil {
		log.Error().Err(err).Msg("failed to save cache")
	}
	log.Info().Msg("shutdown complete")
}

func healthThresholds(cfg *config.Config) health.Thresholds {
	thresholds := health.DefaultThresholds()
	if cfg.Health == nil {
		return thresholds
	}

	thresholds.ParserStale = time.Duration(cfg.Health.ParserStaleMinutes) * time.Minute
	thresholds.ParserFailures = cfg.Health.ParserFailures
	thresholds.ParserLayout = cfg.Health.ParserLayoutFailures
	thresholds.ParserGuard = cfg.Health.ParserGuardFailures
	thresholds.CalendarStale = time.Duration(cfg.Health.CalendarStaleMinutes) * time.Minute
	thresholds.CalendarFailures = cfg.Health.CalendarFailures
	thresholds.APIErrors = cfg.Health.APIErrors
	thresholds.APIWindow = time.Duration(cfg.Health.APIWindowMinutes) * time.Minute
	thresholds.APISlow = time.Duration(cfg.Health.APISlowMS) * time.Millisecond

	return thresholds.WithDefaults()
}

func apiProbeBaseURL(cfg *config.Config) string {
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTP.Port)
}

func configStamp(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

func parserGuard(cfg *config.Config) parserpkg.Guard {
	return parserpkg.Guard{
		Disabled:       cfg.Parser.Guard.Disabled,
		MinItems:       cfg.Parser.Guard.MinItems,
		MaxDropPercent: cfg.Parser.Guard.MaxDropPercent,
	}
}

func parserLayoutIssues(report parserpkg.Report) []health.LayoutIssue {
	issues := make([]health.LayoutIssue, 0, len(report.Failing()))
	for _, probe := range report.Failing() {
		issues = append(issues, health.LayoutIssue{
			Source:   report.Source,
			Selector: probe.Selector,
			Expected: probe.Expected,
			Found:    probe.Found,
		})
	}
	return issues
}

func guardIssues(report parserpkg.Report) []health.GuardIssue {
	var issues []health.GuardIssue

	if report.Keep != nil {
		issues = append(issues, health.GuardIssue{
			Source: report.Source,
			Reason: report.Keep.Reason,
			Detail: report.Keep.Summary(),
		})
	}
	for _, name := range report.Fallbacks {
		issues = append(issues, health.GuardIssue{
			Source: report.Source,
			Reason: "fallback",
			Detail: name,
		})
	}

	return issues
}

func googleDayChanges(changes []cache.DayChange) []telegrambot.GoogleDayChange {
	result := make([]telegrambot.GoogleDayChange, 0, len(changes))
	for _, change := range changes {
		kind := "group"
		if change.Kind == cache.KindTeachers {
			kind = "teacher"
		}
		result = append(result, telegrambot.GoogleDayChange{Type: kind, Value: change.Value, Date: change.Date})
	}
	return result
}

func googleOAuthHandler(cfg *config.Config, service *google.CalendarService, chats *telegrambot.Repository, bot *telegrambot.Bot, log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Auth code not provided", http.StatusBadRequest)
			return
		}

		serviceName, peerID, err := telegrambot.DecodeGoogleState(r.URL.Query().Get("state"))
		if err != nil {
			http.Error(w, "State not provided", http.StatusBadRequest)
			return
		}

		credentials, email, err := service.Exchange(r.Context(), code)
		if err != nil {
			log.Error().Err(err).Msg("google oauth exchange failed")
			http.Error(w, "Не удалось получить доступ к Google аккаунту", http.StatusBadRequest)
			return
		}

		if err := chats.SaveGoogleAccount(&telegrambot.GoogleAccount{
			Email:              email,
			RefreshToken:       credentials.RefreshToken,
			AccessToken:        credentials.AccessToken,
			AccessTokenExpires: credentials.Expiry,
		}); err != nil {
			log.Error().Err(err).Msg("failed to save google account")
			http.Error(w, "Не удалось сохранить Google аккаунт", http.StatusInternalServerError)
			return
		}

		chat, err := chats.FindOrCreate(serviceName, peerID)
		if err != nil {
			log.Error().Err(err).Msg("failed to load chat for google link")
		} else {
			chat.GoogleEmail = email
			if err := chats.Save(chat); err != nil {
				log.Error().Err(err).Msg("failed to link google account to chat")
			}
		}

		if err := bot.SendGoogleLinked(peerID, email); err != nil {
			log.Error().Err(err).Msg("failed to notify chat about google link")
		}

		w.Write([]byte("Аккаунт успешно привязан, можете вернуться обратно в чат"))
	}
}
