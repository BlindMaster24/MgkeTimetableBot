package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func newWiringRuntime(t *testing.T) *runtime {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.CacheDir = filepath.Join(dir, "cache", "rasp")
	cfg.DBPath = filepath.Join(dir, "timetable_archive.db")
	cfg.ChatDBPath = filepath.Join(dir, "bot_chats.db")
	cfg.EncryptKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	cfg.Accept.Private = true

	return &runtime{
		cfg:     cfg,
		log:     logger.New("error", nil),
		metrics: health.NewTracker(health.DefaultThresholds()),
	}
}

func TestOpenStoresCreatesTheCacheAndTheArchive(t *testing.T) {
	r := newWiringRuntime(t)

	if !r.openStores() {
		t.Fatal("openStores must succeed on an empty directory")
	}
	defer r.close()

	if r.cache == nil || r.archive == nil {
		t.Fatal("openStores must wire both the cache and the archive")
	}
	if _, err := os.Stat(r.cfg.ResolvedDBPath()); err != nil {
		t.Fatalf("the archive file must exist after openStores: %v", err)
	}
	stats, err := r.archive.Stats(t.Context())
	if err != nil {
		t.Fatalf("archive stats: %v", err)
	}
	if stats.Total != 0 || stats.WithGroup != 0 || stats.WithTeacher != 0 {
		t.Fatalf("a fresh archive must be empty, got %+v", stats)
	}
}

func TestOpenStoresAdoptsTheLegacyArchiveName(t *testing.T) {
	r := newWiringRuntime(t)
	legacy := filepath.Join(filepath.Dir(r.cfg.DBPath), config.LegacyDBName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}

	first := newWiringRuntime(t)
	if !first.openStores() {
		t.Fatal("openStores must open the fresh database")
	}
	first.close()
	if err := os.Rename(first.cfg.ResolvedDBPath(), legacy); err != nil {
		t.Skipf("the platform does not let the test rename the file: %v", err)
	}

	if !r.openStores() {
		t.Fatal("openStores must adopt the legacy sqlite3.db file")
	}
	defer r.close()
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("the legacy file must stay in place: %v", err)
	}
	if _, err := os.Stat(r.cfg.ResolvedDBPath()); err == nil {
		t.Fatal("openStores must not create a second archive next to the adopted one")
	}
}

func TestOpenStoresRefusesAnUnusableArchivePath(t *testing.T) {
	r := newWiringRuntime(t)
	blocker := filepath.Join(filepath.Dir(r.cfg.DBPath), "blocker")
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.cfg.DBPath = filepath.Join(blocker, "archive.db")

	if r.openStores() {
		t.Fatal("openStores must report a failure instead of half-wiring the runtime")
	}
	if r.archive != nil {
		t.Fatal("a failed openStores must leave the archive field empty")
	}
}

func TestOpenChatsPreparesTheKeyStore(t *testing.T) {
	r := newWiringRuntime(t)

	if !r.openChats() {
		t.Fatal("openChats must succeed with a valid encrypt key")
	}
	defer r.chats.Close()

	if r.chats == nil || r.keys == nil {
		t.Fatal("openChats must wire the chat repository and the key store")
	}
	if !r.keys.Enabled() {
		t.Fatal("a 32-byte key must enable the API")
	}
	if r.probeToken == "" {
		t.Fatal("openChats must mint the probe token the API probe signs its requests with")
	}
	chat, err := r.chats.FindOrCreate("telegram", 6409603928)
	if err != nil {
		t.Fatalf("find the first chat: %v", err)
	}
	if !chat.Accepted {
		t.Fatal("accept.private must reach the chat repository, a new chat has to be accepted")
	}
}

func TestOpenChatsWithoutAKeyKeepsTheAPIOff(t *testing.T) {
	r := newWiringRuntime(t)
	r.cfg.EncryptKey = strings.Repeat("x", 8)

	if !r.openChats() {
		t.Fatal("a missing key must not stop the bot, only the API")
	}
	defer r.chats.Close()

	if r.keys.Enabled() {
		t.Fatal("a short key must leave the API disabled")
	}
	if r.probeToken != "" {
		t.Fatal("a disabled key store cannot mint a probe token")
	}
}

func TestTheWiringChainReachesABot(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer stub.Close()

	r := newWiringRuntime(t)
	r.cfg.Telegram.Token = "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	r.cfg.Telegram.APIBaseURL = stub.URL
	r.build = build.Info{Version: "test"}
	r.counters = health.NewTelegramCounters()
	r.loc = i18n.New("ru")

	if !r.openStores() {
		t.Fatal("openStores")
	}
	defer r.close()
	if !r.openChats() {
		t.Fatal("openChats")
	}
	defer r.chats.Close()
	if !r.startAPI() {
		t.Fatal("startAPI")
	}
	if !r.startBot() {
		t.Fatal("startBot")
	}
	if r.bot == nil || r.incidents == nil {
		t.Fatal("startBot must wire the bot and its incident log")
	}

	r.cfg.Archive.BackupDir = filepath.Join(filepath.Dir(r.cfg.DBPath), "backups")
	r.startMaintenance(t.Context())

	maintenance := newArchiveMaintenance(r.cfg, r.archive, r.log, r.metrics)
	report, err := maintenance.Run(t.Context())
	if err != nil {
		t.Fatalf("the maintenance hook must run on a fresh archive: %v", err)
	}
	if report.Path == "" {
		t.Fatal("the backup report must name the file it wrote")
	}

	r.configureMetrics()
}
