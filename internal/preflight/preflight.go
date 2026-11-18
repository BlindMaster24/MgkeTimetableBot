package preflight

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

const (
	LevelOK   = "ok"
	LevelWarn = "warn"
	LevelFail = "fail"
	LevelSkip = "skip"
)

const (
	localePath       = "internal/i18n/locales/ru.json"
	telegramSource   = "internal/telegram"
	noticeSource     = "internal/notification"
	defaultTimeout   = 45 * time.Second
	storageProbeName = ".preflight-probe"

	defaultCallsStaleDays = 180
)

var (
	telegramTokenRe = regexp.MustCompile(`^\d{6,}:[A-Za-z0-9_-]{30,}$`)
	serviceEmailRe  = regexp.MustCompile(`^[\w.+-]+@[\w-]+\.iam\.gserviceaccount\.com$`)
	localeKeyRe     = regexp.MustCompile(`\b(?:loc|locData)\("([A-Za-z0-9_]+)"`)
)

type Check struct {
	Name   string   `json:"name"`
	Level  string   `json:"level"`
	Detail string   `json:"detail"`
	Hints  []string `json:"hints,omitempty"`
}

type Report struct {
	Title        string    `json:"-"`
	SubjectLabel string    `json:"-"`
	Subject      string    `json:"subject,omitempty"`
	ConfigPath   string    `json:"configPath,omitempty"`
	Started      time.Time `json:"started"`
	ElapsedMS    int64     `json:"elapsedMs"`
	Incomplete   bool      `json:"incomplete,omitempty"`
	Checks       []Check   `json:"checks"`
}

type Endpoints struct {
	Groups   string
	Teachers string
	Calls    string
}

type Options struct {
	ConfigPath     string
	Endpoints      Endpoints
	HTTPClient     *http.Client
	ImageDir       string
	SkipSite       bool
	SkipImage      bool
	Now            func() time.Time
	CallsStaleDays int
}

func Run(opts Options) Report {
	started := time.Now()
	if opts.Now != nil {
		started = opts.Now()
	}

	report := &Report{ConfigPath: opts.ConfigPath, Started: started}

	staleDays := opts.CallsStaleDays
	if staleDays <= 0 {
		staleDays = defaultCallsStaleDays
	}

	cfg, err := config.LoadWithEnv(opts.ConfigPath, os.LookupEnv)
	if err != nil {
		report.add(Check{Name: "config", Level: LevelFail, Detail: fmt.Sprintf("%s: %v", opts.ConfigPath, err)})
		report.finish(started)
		return *report
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	report.add(checkConfig(cfg, opts.ConfigPath))
	report.add(checkCredentials(cfg))
	report.add(checkWebhook(cfg))
	report.add(checkTelegramAPI(cfg))
	report.add(checkLocale(findRepoRoot()))
	report.add(checkStorage(cfg))
	report.add(checkTimetable(cfg))
	report.add(checkMetrics(cfg))
	report.add(checkImageLayout())

	endpoints := endpointsFor(cfg, opts.Endpoints)
	switch {
	case opts.SkipSite:
		report.add(Check{Name: "site", Level: LevelSkip, Detail: "network checks skipped"})
		report.add(checkImageFromCache(cfg, opts))
	default:
		groups, groupCheck := checkGroups(client, endpoints.Groups)
		report.add(groupCheck)
		report.add(checkTeachers(client, endpoints.Teachers))
		report.add(checkCalls(client, endpoints.Calls, started, staleDays))
		if groups != nil {
			report.add(checkImage(groups, opts))
		} else {
			report.add(checkImageFromCache(cfg, opts))
		}
	}

	report.finish(started)
	return *report
}
