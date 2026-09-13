package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type TimetableConfig struct {
	Weekdays [][2][2]string `yaml:"weekdays"`
	Saturday [][2][2]string `yaml:"saturday"`
}

type CallsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	PreferSite bool   `yaml:"prefer_site"`
	Notify     bool   `yaml:"notify"`
	Campus     string `yaml:"campus"`
}

type HealthConfig struct {
	Disabled             bool `yaml:"disabled"`
	CheckMinutes         int  `yaml:"check_minutes"`
	CooldownMinutes      int  `yaml:"cooldown_minutes"`
	ParserStaleMinutes   int  `yaml:"parser_stale_minutes"`
	ParserFailures       int  `yaml:"parser_failures"`
	ParserLayoutFailures int  `yaml:"parser_layout_failures"`
	ParserGuardFailures  int  `yaml:"parser_guard_failures"`
	CalendarStaleMinutes int  `yaml:"calendar_stale_minutes"`
	CalendarFailures     int  `yaml:"calendar_failures"`
	APIErrors            int  `yaml:"api_errors"`
	APIWindowMinutes     int  `yaml:"api_window_minutes"`
}

type GuardConfig struct {
	Disabled       bool `yaml:"disabled"`
	MinItems       int  `yaml:"min_items"`
	MaxDropPercent int  `yaml:"max_drop_percent"`
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
		Token    string  `yaml:"token" env:"TG_TOKEN"`
		AdminIDs []int64 `yaml:"admin_ids"`
		Noticer  bool    `yaml:"noticer"`
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

	Health *HealthConfig `yaml:"health"`

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
	DefaultChatDBPath = "./bot_chats.db"
	DefaultCacheDir   = "./cache/rasp"
)

func (c *Config) ResolvedChatDBPath() string {
	return pathOr(c.ChatDBPath, DefaultChatDBPath)
}

func (c *Config) ResolvedCacheDir() string {
	return pathOr(c.CacheDir, DefaultCacheDir)
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
