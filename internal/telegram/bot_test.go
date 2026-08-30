package telegram

import (
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
)

func setupTestBot(t *testing.T) *Bot {
	t.Helper()
	cfg := &config.Config{}
	cfg.Telegram.Token = "test:token"
	cfg.Timetable.Weekdays = [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}}
	cfg.Timetable.Saturday = [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}}

	log := logger.New("error", nil)
	loc := i18n.New("ru")
	_, _ = cache.New(t.TempDir())

	b := &Bot{
		cfg:       cfg,
		log:       log,
		i18n:      loc,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
	}
	b.registerAll()
	return b
}

func TestBotRegisterCommands(t *testing.T) {
	b := setupTestBot(t)
	expected := []string{"/start", "/help", "/cancel", "/setup", "/day", "/week", "/calls", "/about", "/group", "/teacher", "/settings", "/image", "/buttons", "/formatter", "/forceparse", "/resetcache", "/eula", "/api", "/debug", "/send", "/trigger"}
	for _, name := range expected {
		if _, ok := b.commands[name]; !ok {
			t.Errorf("missing command %s", name)
		}
	}
	if len(b.commands) != len(expected) {
		t.Errorf("expected %d commands, got %d", len(expected), len(b.commands))
	}
}

func TestBotRegisterCallbacks(t *testing.T) {
	b := setupTestBot(t)
	if len(b.callbacks) == 0 {
		t.Error("expected callbacks to be registered")
	}
}

func TestBotCommandDescription(t *testing.T) {
	b := setupTestBot(t)
	for name, cmd := range b.commands {
		desc := cmd.Description()
		if desc == "" || desc == "["+name+"]" {
			t.Errorf("command %s has no translated description", name)
		}
	}
}

func TestBotCommandMatchText(t *testing.T) {
	b := setupTestBot(t)

	matcherTests := []struct {
		cmd   string
		text  string
		match bool
	}{
		{"/cancel", "Отмена", true},
		{"/cancel", "случайный текст", false},
		{"/day", "📄 На день", true},
		{"/week", "📑 На неделю", true},
		{"/calls", "🕐 Звонки", true},
		{"/about", "💡 О боте", true},
		{"/group", "👩‍🎓 Группа", true},
		{"/teacher", "👩‍🏫 Преподаватель", true},
		{"/settings", "⚙️ Настройки", true},
		{"/setup", "📚 Первоначальная настройка", true},
		{"/image", "📷 Изображение", true},
	}

	for _, tc := range matcherTests {
		cmd, ok := b.commands[tc.cmd]
		if !ok {
			t.Errorf("command %s not found", tc.cmd)
			continue
		}
		if tm, ok := cmd.(TextMatcher); ok {
			got := tm.MatchText(tc.text)
			if got != tc.match {
				t.Errorf("cmd %s MatchText(%q) = %v, want %v", tc.cmd, tc.text, got, tc.match)
			}
		}
	}
}

func TestFormatCallsSchedule(t *testing.T) {
	b := setupTestBot(t)
	result := b.formatCallsSchedule()
	if result == "" {
		t.Error("expected non-empty calls schedule")
	}
}

func TestFormatCallsScheduleEmpty(t *testing.T) {
	b := setupTestBot(t)
	b.cfg.Timetable.Weekdays = nil
	b.cfg.Timetable.Saturday = nil
	result := b.formatCallsSchedule()
	if result == "" {
		t.Error("expected non-empty fallback")
	}
}

func TestBotLoc(t *testing.T) {
	b := setupTestBot(t)
	welcome := b.loc("welcome")
	if welcome == "" || welcome == "[welcome]" {
		t.Errorf("expected translated welcome, got %s", welcome)
	}
}

func TestCleanupTempFiles(t *testing.T) {
	b := setupTestBot(t)
	dir := t.TempDir()
	b.CleanupTempFiles(dir, 0)
}

func TestBotAccessors(t *testing.T) {
	b := setupTestBot(t)
	if b.Config() == nil {
		t.Error("expected non-nil config")
	}
	if b.I18n() == nil {
		t.Error("expected non-nil i18n")
	}
	if b.Log() == nil {
		t.Error("expected non-nil logger")
	}
}
