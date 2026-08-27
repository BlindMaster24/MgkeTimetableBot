package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type CallSlot [2][2]string

type TimetableConfig struct {
	Weekdays    []CallSlot   `yaml:"weekdays"`
	Saturday    []CallSlot   `yaml:"saturday"`
	Shortened1h [][2]string  `yaml:"shortened_1h"`
}

type Config struct {
	Dev    bool   `yaml:"dev" env:"DEV"`
	DBPath string `yaml:"db_path" env:"DB_PATH"`

	Logging struct {
		Level  string `yaml:"level" env:"LOG_LEVEL"`
		Format string `yaml:"format"`
		File   struct {
			Enabled    bool   `yaml:"enabled"`
			Path       string `yaml:"path"`
			MaxSizeMB  int    `yaml:"max_size_mb"`
			MaxFiles   int    `yaml:"max_files"`
		} `yaml:"file"`
	} `yaml:"logging"`

	HTTP struct {
		ServerName string `yaml:"server_name" env:"HTTP_SERVER_NAME"`
		Port       int    `yaml:"port" env:"HTTP_PORT"`
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
		CalendarOwners []string `yaml:"calendar_owners"`
	} `yaml:"google"`

	Calendar struct {
		ICS struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"ics"`
	} `yaml:"calendar"`

	Accept struct {
		Room    bool `yaml:"room"`
		Private bool `yaml:"private"`
	} `yaml:"accept"`

	Parser struct {
		Enabled    bool `yaml:"enabled"`
		SyncMode   bool `yaml:"sync_mode"`
		LocalMode  bool `yaml:"local_mode"`
		IgnoreHash bool `yaml:"ignore_hash"`
		V2         *struct {
			Enabled           bool   `yaml:"enabled"`
			FallbackToV1      bool   `yaml:"fallback_to_v1"`
			WeekPolicy        string `yaml:"week_policy"`
			AllowTwoTables    bool   `yaml:"allow_two_tables"`
			Strict            bool   `yaml:"strict"`
			DiffLog           bool   `yaml:"diff_log"`
			DiffLogLimit      int    `yaml:"diff_log_limit"`
			HeaderScanRows    int    `yaml:"header_scan_rows"`
			MinDaysInTable    int    `yaml:"min_days_in_table"`
			MaxLessonsPerDay  int    `yaml:"max_lessons_per_day"`
			ValidationSample  int    `yaml:"validation_sample"`
			HashMode          string `yaml:"hash_mode"`
			FetchRetry        int    `yaml:"fetch_retry"`
			WeekJumpThreshold int    `yaml:"week_jump_threshold"`
			SundayHoldCurrent bool   `yaml:"sunday_hold_current"`
			Quarantine        *struct {
				Enabled    bool `yaml:"enabled"`
				MinLessons int `yaml:"min_lessons"`
			} `yaml:"quarantine"`
			Metrics *struct {
				Enabled bool   `yaml:"enabled"`
				Dir     string `yaml:"dir"`
			} `yaml:"metrics"`
		} `yaml:"v2"`
		EndHour   int       `yaml:"end_hour"`
		Activity  [2]int    `yaml:"activity"`
		Endpoints struct {
			TimetableGroup  string   `yaml:"timetable_group"`
			TimetableTeacher string `yaml:"timetable_teacher"`
			Team            []string `yaml:"team"`
			BellSchedule    string   `yaml:"bell_schedule"`
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
		LessonIndexIfEmpty int    `yaml:"lesson_index_if_empty"`
		Calls              *struct {
			Enabled    bool `yaml:"enabled"`
			PreferSite bool `yaml:"prefer_site"`
			Notify     bool `yaml:"notify"`
		} `yaml:"calls"`
		Proxy *string `yaml:"proxy"`
	} `yaml:"parser"`

	Timetable TimetableConfig `yaml:"timetable"`

	EncryptKey string `yaml:"encrypt_key" env:"ENCRYPT_KEY"`

	GlobalNoticer bool `yaml:"global_noticer"`
	GlobalAdblock bool `yaml:"global_adblock"`
}

type LessonFilter struct {
	Lesson string `yaml:"lesson"`
	Type   string `yaml:"type"`
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
