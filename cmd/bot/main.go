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

	adapter := &chatFinderAdapter{repo: chatRepo}
	notifier := notification.NewChangeNotifier(raspCache, log, bot, adapter)

	bot.SetParseFunc(func() error {
		oldGroupsHash := raspCache.GetGroupsHash()
		oldTeachersHash := raspCache.GetTeachersHash()

		groupURL := cfg.Parser.Endpoints.TimetableGroup
		teacherURL := cfg.Parser.Endpoints.TimetableTeacher
		err := parserpkg.FetchAndParse(log, raspCache, groupURL, teacherURL, cfg.Parser.Endpoints.BellSchedule)
		if err == nil {
			go notifier.NotifyChanges(oldGroupsHash, oldTeachersHash)
		}
		return err
	})

	if err := bot.SetMyCommands(); err != nil {
		log.Warn().Err(err).Msg("failed to set bot commands")
	}

	scheduler := notification.NewScheduler(cfg, raspCache, log, bot, adapter)
	scheduler.Start()
	log.Info().Msg("notification scheduler started")

	go func() {
		groupURL := cfg.Parser.Endpoints.TimetableGroup
		teacherURL := cfg.Parser.Endpoints.TimetableTeacher
		log.Info().Msg("initial parse starting")
		if err := parserpkg.FetchAndParse(log, raspCache, groupURL, teacherURL, cfg.Parser.Endpoints.BellSchedule); err != nil {
			log.Error().Err(err).Msg("initial parse failed")
		} else {
			log.Info().Int("groups", len(raspCache.GetGroups())).Int("teachers", len(raspCache.GetTeachers())).Msg("initial parse done")
		}
	}()

	log.Info().Msg("bot starting")
	if err := bot.Run(ctx); err != nil {
		log.Error().Err(err).Msg("bot stopped")
	}

	scheduler.Stop()

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
	repo *telegrambot.Repository
}

func (a *chatFinderAdapter) FindAllWithNotifications(service string) ([]*notification.ChatInfo, error) {
	chats, err := a.repo.FindAllWithNotifications(service)
	if err != nil {
		return nil, err
	}
	var result []*notification.ChatInfo
	for _, c := range chats {
		result = append(result, &notification.ChatInfo{
			ID:            c.PeerID,
			Mode:          string(c.Mode),
			Group:         c.Group,
			Teacher:       c.Teacher,
			NoticeChanges: c.NoticeChanges,
		})
	}
	return result, nil
}




