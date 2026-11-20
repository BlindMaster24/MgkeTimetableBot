package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	parserpkg "github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

type fakeProbe struct {
	timetable error
	calls     error
	team      error

	timetableCalls int
	callsCalls     int
	teamCalls      int
}

func (p *fakeProbe) Timetable(context.Context, string, string) error {
	p.timetableCalls++
	return p.timetable
}

func (p *fakeProbe) Calls(context.Context, string) error {
	p.callsCalls++
	return p.calls
}

func (p *fakeProbe) Team(context.Context, []string) error {
	p.teamCalls++
	return p.team
}

func newParseRuntime(t *testing.T) *runtime {
	t.Helper()

	cfg := &config.Config{}
	cfg.Parser.Endpoints.TimetableGroup = "http://example.test/groups"
	cfg.Parser.Endpoints.TimetableTeacher = "http://example.test/teachers"
	cfg.Parser.Endpoints.BellSchedule = "http://example.test/calls"
	cfg.Parser.Endpoints.Team = []string{"http://example.test/team"}
	cfg.Telegram.Token = "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	// Keep the bot offline: every Bot API call goes to this stub.
	apiStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	t.Cleanup(apiStub.Close)
	cfg.Telegram.APIBaseURL = apiStub.URL

	chats, err := telegrambot.New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chats.Close() })

	loc := i18n.New("ru")
	log := logger.New("error", nil)
	cacheDir := t.TempDir()
	cfg.CacheDir = cacheDir
	rasp, err := cache.New(cacheDir)
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	bot, err := telegrambot.NewBot(cfg, log, loc, chats, rasp, nil)
	if err != nil {
		t.Fatal(err)
	}

	return &runtime{
		cfg:     cfg,
		log:     log,
		metrics: health.NewTracker(health.DefaultThresholds()),
		cache:   rasp,
		chats:   chats,
		bot:     bot,
		loc:     loc,
	}
}

func TestParseOnceReportsAFailedCycle(t *testing.T) {
	r := newParseRuntime(t)
	probe := &fakeProbe{timetable: errors.New("site is down"), calls: errors.New("calls are down")}
	loop := parserpkg.LoopConfigFrom(r.cfg)

	err := r.parseOnce(t.Context(), probe, loop, true)
	if err == nil {
		t.Fatal("a failed parse must return an error")
	}
	if probe.timetableCalls != 1 || probe.callsCalls != 1 || probe.teamCalls != 1 {
		t.Fatalf("forced parse must hit every source, got %+v", probe)
	}
	if r.cache.Stats().SuccessUpdate {
		t.Fatal("the cache must remember the failure")
	}
}

func TestParseOnceSkipsSourcesThatAreNotDue(t *testing.T) {
	r := newParseRuntime(t)
	probe := &fakeProbe{}
	loop := parserpkg.LoopConfigFrom(r.cfg)

	r.cache.SetCalls(t.Context(), cache.Schedule{}, cache.Schedule{}, "manual")
	r.cache.SetTeam(t.Context(), map[string]string{"Иванов": "http://example.test/team/1"}, []string{"hash"})

	if err := r.parseOnce(t.Context(), probe, loop, false); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if probe.callsCalls != 0 {
		t.Fatalf("fresh calls must not be fetched again, got %d", probe.callsCalls)
	}
	if probe.teamCalls != 0 {
		t.Fatalf("a fresh team dump must not be fetched again, got %d", probe.teamCalls)
	}
	if probe.timetableCalls != 1 {
		t.Fatalf("the timetable is due every cycle, got %d calls", probe.timetableCalls)
	}

	if err := r.parseOnce(t.Context(), probe, loop, true); err != nil {
		t.Fatalf("forced parse: %v", err)
	}
	if probe.callsCalls != 1 || probe.teamCalls != 1 {
		t.Fatalf("a forced parse must refresh calls and team, got %+v", probe)
	}
}
