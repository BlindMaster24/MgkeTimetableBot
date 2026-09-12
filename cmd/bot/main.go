package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/blindmaster24/MgkeTimetableBot/internal/api"
	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	parserpkg "github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
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

	cfg, err := config.Load(*cfgPath)
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
	loc := i18n.New("ru")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	raspCache, err := cache.New("./cache/rasp")
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

	initArchiveSchema(archiveRepo)

	syncArchive := func(tag string) {
		if err := archiveRepo.SyncFromCache(raspCache.GetGroups(), raspCache.GetTeachers()); err != nil {
			log.Error().Err(err).Str("tag", tag).Msg("archive sync failed")
		} else {
			log.Info().Str("tag", tag).Msg("archive synced from cache")
		}
	}

	syncArchive("startup")

	apiServer := api.NewServer(raspCache, cfg.HTTP.Port)
	go func() {
		log.Info().Int("port", cfg.HTTP.Port).Msg("API server starting")
		if err := apiServer.Run(); err != nil {
			log.Error().Err(err).Msg("API server error")
		}
	}()

	chatRepo, err := telegrambot.NewChatRepo("./bot_chats.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open chat DB")
	}
	defer chatRepo.Close()

	bot, err := telegrambot.NewBot(cfg, log, loc, chatRepo, raspCache, archiveRepo)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create bot")
	}

	adapter := &chatFinderAdapter{repo: chatRepo, adminIDs: cfg.Telegram.AdminIDs}

	if cfg.Telegram.Noticer {
		eventNotifier := notification.NewEventNotifier(raspCache, cfg, log, bot, adapter)
		scheduler := notification.NewScheduler(cfg, raspCache, log, bot, adapter)
		scheduler.Start()
		defer scheduler.Stop()
		log.Info().Msg("notification scheduler started")

		drainEvents := func(tag string) {
			events := raspCache.DrainEvents()
			if len(events) > 0 {
				log.Info().Int("count", len(events)).Str("tag", tag).Msg("draining cache events")
				eventNotifier.HandleEvents(events)
			}
		}

		drainEvents("startup")

		bot.SetParseFunc(func() error {
			groupURL := cfg.Parser.Endpoints.TimetableGroup
			teacherURL := cfg.Parser.Endpoints.TimetableTeacher
			err := parserpkg.FetchAndParse(log, raspCache, groupURL, teacherURL, cfg.Parser.Endpoints.BellSchedule)
			if err != nil {
				bot.AddParseLog(false, err.Error())
				go eventNotifier.ParserError(err)
			} else {
				bot.AddParseLog(true, fmt.Sprintf("groups=%d teachers=%d", len(raspCache.GetGroups()), len(raspCache.GetTeachers())))
				drainEvents("parse")
			}
			go syncArchive("parse")
			return err
		})
	} else {
		bot.SetParseFunc(func() error {
			groupURL := cfg.Parser.Endpoints.TimetableGroup
			teacherURL := cfg.Parser.Endpoints.TimetableTeacher
			err := parserpkg.FetchAndParse(log, raspCache, groupURL, teacherURL, cfg.Parser.Endpoints.BellSchedule)
			if err != nil {
				bot.AddParseLog(false, err.Error())
			} else {
				bot.AddParseLog(true, fmt.Sprintf("groups=%d teachers=%d", len(raspCache.GetGroups()), len(raspCache.GetTeachers())))
			}
			go syncArchive("parse")
			return err
		})
	}

	if err := bot.SetMyCommands(); err != nil {
		log.Warn().Err(err).Msg("failed to set bot commands")
	}

	go func() {
		groupURL := cfg.Parser.Endpoints.TimetableGroup
		teacherURL := cfg.Parser.Endpoints.TimetableTeacher
		log.Info().Msg("initial parse starting")
		if err := parserpkg.FetchAndParse(log, raspCache, groupURL, teacherURL, cfg.Parser.Endpoints.BellSchedule); err != nil {
			log.Error().Err(err).Msg("initial parse failed")
		} else {
			log.Info().Int("groups", len(raspCache.GetGroups())).Int("teachers", len(raspCache.GetTeachers())).Msg("initial parse done")
		}
		syncArchive("initial")
	}()

	log.Info().Msg("bot starting")
	if err := bot.Run(ctx); err != nil {
		log.Error().Err(err).Msg("bot stopped")
	}

	if err := raspCache.Save(); err != nil {
		log.Error().Err(err).Msg("failed to save cache")
	}
	log.Info().Msg("shutdown complete")
}

func initArchiveSchema(repo *archive.Repository) {
	db := repo.DB()
	db.Exec(`CREATE TABLE IF NOT EXISTS timetable_archive (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		day INTEGER NOT NULL,
		"group" TEXT,
		teacher TEXT,
		data TEXT NOT NULL,
		UNIQUE(day, "group"),
		UNIQUE(day, teacher)
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_group_day ON timetable_archive("group", day)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_teacher_day ON timetable_archive(teacher, day)`)
}

type chatFinderAdapter struct {
	repo     *telegrambot.Repository
	adminIDs []int64
}

func (a *chatFinderAdapter) toEventChats(chats []*telegrambot.Chat) []*notification.EventChat {
	result := make([]*notification.EventChat, 0, len(chats))
	for _, c := range chats {
		result = append(result, &notification.EventChat{
			ID:        c.ID,
			PeerID:    c.PeerID,
			Mode:      string(c.Mode),
			Group:     c.Group,
			Teacher:   c.Teacher,
			Formatter: c.Formatter,
		})
	}
	return result
}

func (a *chatFinderAdapter) FindChatsByGroups(service string, groups []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindChatsByGroups(service, groups, noticeChanges)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}

func (a *chatFinderAdapter) FindChatsByTeachers(service string, teachers []string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindChatsByTeachers(service, teachers, noticeChanges)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}

func (a *chatFinderAdapter) FindSubscribedChatsByGroup(service, group string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindSubscribedChatsByGroup(service, group, noticeChanges)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}

func (a *chatFinderAdapter) FindSubscribedChatsByTeacher(service, teacher string, noticeChanges bool) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindSubscribedChatsByTeacher(service, teacher, noticeChanges)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}

func (a *chatFinderAdapter) FindChatsWithNotice(service string, notice string) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindChatsWithNotice(service, notice)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}

func (a *chatFinderAdapter) FindAdminChats(service string) ([]*notification.EventChat, error) {
	chats, err := a.repo.FindAdminChats(service, a.adminIDs)
	if err != nil {
		return nil, err
	}
	return a.toEventChats(chats), nil
}
