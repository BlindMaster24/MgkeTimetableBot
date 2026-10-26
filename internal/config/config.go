package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TimetableConfig struct {
	Weekdays              [][2][2]string `yaml:"weekdays"`
	Saturday              [][2][2]string `yaml:"saturday"`
	ReminderLeadMinutes   int            `yaml:"reminder_lead_minutes"`
	ReminderJitterSeconds int            `yaml:"reminder_jitter_seconds"`
}

type CallsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	PreferSite bool   `yaml:"prefer_site"`
	Notify     bool   `yaml:"notify"`
	Campus     string `yaml:"campus"`
}

type SQLiteConfig struct {
	BusyTimeoutMS int `yaml:"busy_timeout_ms"`
	BusyRetries   int `yaml:"busy_retries"`
}

type HealthConfig struct {
	Disabled             bool `yaml:"disabled"`
	CheckMinutes         int  `yaml:"check_minutes"`
	CooldownMinutes      int  `yaml:"cooldown_minutes"`
	BackupFailures       int  `yaml:"backup_failures"`
	ParserStaleMinutes   int  `yaml:"parser_stale_minutes"`
	ParserFailures       int  `yaml:"parser_failures"`
	ParserLayoutFailures int  `yaml:"parser_layout_failures"`
	ParserGuardFailures  int  `yaml:"parser_guard_failures"`
	CalendarStaleMinutes int  `yaml:"calendar_stale_minutes"`
	CalendarFailures     int  `yaml:"calendar_failures"`
	APIErrors            int  `yaml:"api_errors"`
	APIWindowMinutes     int  `yaml:"api_window_minutes"`
	APISlowMS            int  `yaml:"api_slow_ms"`
}

type GuardConfig struct {
	Disabled       bool `yaml:"disabled"`
	MinItems       int  `yaml:"min_items"`
	MaxDropPercent int  `yaml:"max_drop_percent"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Token   string `yaml:"token"`
}

type ArchiveConfig struct {
	BackupDir           string `yaml:"backup_dir"`
	BackupKeep          int    `yaml:"backup_keep"`
	RetentionDays       int    `yaml:"retention_days"`
	MaintenanceSchedule string `yaml:"maintenance_schedule"`
}

type Config struct {
	DBPath     string `yaml:"db_path" env:"DB_PATH"`
	ChatDBPath string `yaml:"chat_db_path"`
	CacheDir   string `yaml:"cache_dir"`

	Logging struct {
		Level  string `yaml:"level" env:"LOG_LEVEL"`
		Format string `yaml:"format"`
		File   struct {
			Enabled   bool   `yaml:"enabled"`
			Path      string `yaml:"path"`
			MaxSizeMB int    `yaml:"max_size_mb"`
			MaxFiles  int    `yaml:"max_files"`
		} `yaml:"file"`
	} `yaml:"logging"`

	HTTP struct {
		Port int `yaml:"port" env:"HTTP_PORT"`
	} `yaml:"http"`

	Telegram struct {
		Token      string  `yaml:"token" env:"TG_TOKEN"`
		APIBaseURL string  `yaml:"api_base_url"`
		AdminIDs   []int64 `yaml:"admin_ids"`
		Noticer    bool    `yaml:"noticer"`
		Webhook    struct {
			Enabled            bool     `yaml:"enabled"`
			Listen             string   `yaml:"listen"`
			URL                string   `yaml:"url"`
			Path               string   `yaml:"path"`
			SecretToken        string   `yaml:"secret_token"`
			Certificate        string   `yaml:"certificate"`
			Key                string   `yaml:"key"`
			IPAddress          string   `yaml:"ip_address"`
			MaxConnections     int      `yaml:"max_connections"`
			Buffer             int      `yaml:"buffer"`
			DropPendingUpdates bool     `yaml:"drop_pending_updates"`
			AllowedUpdates     []string `yaml:"allowed_updates"`
		} `yaml:"webhook"`
	} `yaml:"telegram"`

	API struct {
		URL string `yaml:"url"`
	} `yaml:"api"`

	Google struct {
		RedirectDomain string `yaml:"redirect_domain"`
		URL            string `yaml:"url"`
		OAuth          struct {
			ClientID     string `yaml:"client_id"`
			ClientSecret string `yaml:"client_secret"`
		} `yaml:"oauth"`
		ServiceAccount struct {
			ClientEmail string `yaml:"client_email"`
			PrivateKey  string `yaml:"private_key"`
		} `yaml:"service_account"`
	} `yaml:"google"`

	Calendar struct {
		ICS struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"ics"`
	} `yaml:"calendar"`

	Health  *HealthConfig `yaml:"health"`
	Archive ArchiveConfig `yaml:"archive"`
	Metrics MetricsConfig `yaml:"metrics"`
	SQLite  SQLiteConfig  `yaml:"sqlite"`

	Accept struct {
		Room    bool `yaml:"room"`
		Private bool `yaml:"private"`
	} `yaml:"accept"`

	Parser struct {
		Enabled   bool   `yaml:"enabled"`
		Activity  [2]int `yaml:"activity"`
		Endpoints struct {
			TimetableGroup   string   `yaml:"timetable_group"`
			TimetableTeacher string   `yaml:"timetable_teacher"`
			Team             []string `yaml:"team"`
			BellSchedule     string   `yaml:"bell_schedule"`
		} `yaml:"endpoints"`
		UpdateInterval struct {
			Default  int `yaml:"default"`
			Activity int `yaml:"activity"`
			Error    int `yaml:"error"`
			Teams    int `yaml:"teams"`
			Calls    int `yaml:"calls"`
		} `yaml:"update_interval"`
		AlertableIgnoreFilter struct {
			Group   []LessonFilter `yaml:"group"`
			Teacher []LessonFilter `yaml:"teacher"`
		} `yaml:"alertable_ignore_filter"`
		LessonIndexIfEmpty int          `yaml:"lesson_index_if_empty"`
		Calls              *CallsConfig `yaml:"calls"`
		Guard              GuardConfig  `yaml:"guard"`
		Proxy              *string      `yaml:"proxy"`
	} `yaml:"parser"`

	Timetable TimetableConfig `yaml:"timetable"`

	EncryptKey string `yaml:"encrypt_key" env:"ENCRYPT_KEY"`
}

type LessonFilter struct {
	Lesson string `yaml:"lesson"`
	Type   string `yaml:"type"`
}

const (
	DefaultDBPath              = "./timetable_archive.db"
	LegacyDBName               = "sqlite3.db"
	DefaultChatDBPath          = "./bot_chats.db"
	DefaultCacheDir            = "./cache/rasp"
	DefaultBackupKeep          = 7
	DefaultMaintenanceSchedule = "0 30 4 * * *"
	DefaultMetricsPath         = "/metrics"
	MinimumMetricsToken        = 8
	DefaultSQLiteBusyTimeoutMS = 5000
	DefaultSQLiteBusyRetries   = 1
)

func (c *Config) ResolvedSQLiteBusyTimeout() time.Duration {
	if c.SQLite.BusyTimeoutMS <= 0 {
		return time.Duration(DefaultSQLiteBusyTimeoutMS) * time.Millisecond
	}
	return time.Duration(c.SQLite.BusyTimeoutMS) * time.Millisecond
}

func (c *Config) ResolvedSQLiteBusyRetries() int {
	switch {
	case c.SQLite.BusyRetries < 0:
		return 0
	case c.SQLite.BusyRetries == 0:
		return DefaultSQLiteBusyRetries
	}
	return c.SQLite.BusyRetries
}

func (c *Config) ResolvedDBPath() string {
	return pathOr(c.DBPath, DefaultDBPath)
}

func (c *Config) ResolvedChatDBPath() string {
	return pathOr(c.ChatDBPath, DefaultChatDBPath)
}

func AdoptLegacyDBPath(path string) (string, bool) {
	configured := strings.TrimSpace(path)
	if configured == "" || filepath.Base(configured) != filepath.Base(DefaultDBPath) {
		return path, false
	}

	legacy := filepath.Join(filepath.Dir(configured), LegacyDBName)
	if filepath.Clean(legacy) == filepath.Clean(configured) {
		return path, false
	}
	if _, err := os.Stat(configured); !errors.Is(err, fs.ErrNotExist) {
		return path, false
	}
	if _, err := os.Stat(legacy); err != nil {
		return path, false
	}
	return legacy, true
}

func (c *Config) ResolvedCacheDir() string {
	return pathOr(c.CacheDir, DefaultCacheDir)
}

func (c *Config) ResolvedBackupKeep() int {
	if c.Archive.BackupKeep < 0 {
		return 0
	}
	if c.Archive.BackupKeep == 0 {
		return DefaultBackupKeep
	}
	return c.Archive.BackupKeep
}

func (c *Config) ResolvedMaintenanceSchedule() string {
	return pathOr(strings.TrimSpace(c.Archive.MaintenanceSchedule), DefaultMaintenanceSchedule)
}

func (c *Config) BackupEnabled() bool {
	return strings.TrimSpace(c.Archive.BackupDir) != ""
}

func (c *Config) RetentionEnabled() bool {
	return c.Archive.RetentionDays > 0
}

func (c *Config) MaintenanceEnabled() bool {
	return c.BackupEnabled() || c.RetentionEnabled()
}

func (c *Config) ResolvedMetricsPath() string {
	return pathOr(strings.TrimSpace(c.Metrics.Path), DefaultMetricsPath)
}

func (c *Config) TelegramAPIServer() (string, error) {
	raw := strings.TrimSpace(c.Telegram.APIBaseURL)
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("telegram.api_base_url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("telegram.api_base_url must be an http or https URL, got %q", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("telegram.api_base_url has no host: %q", raw)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func pathOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
