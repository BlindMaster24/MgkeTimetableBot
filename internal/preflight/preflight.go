package preflight

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
	"github.com/blindmaster24/MgkeTimetableBot/internal/model"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
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
	ConfigPath string    `json:"configPath"`
	Started    time.Time `json:"started"`
	ElapsedMS  int64     `json:"elapsedMs"`
	Checks     []Check   `json:"checks"`
}

type Endpoints struct {
	Groups   string
	Teachers string
	Calls    string
}

type Options struct {
	ConfigPath string
	Endpoints  Endpoints
	HTTPClient *http.Client
	ImageDir   string
	SkipSite   bool
	SkipImage  bool
	Now        func() time.Time
}

func Run(opts Options) Report {
	started := time.Now()
	if opts.Now != nil {
		started = opts.Now()
	}

	report := &Report{ConfigPath: opts.ConfigPath, Started: started}

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
	report.add(checkLocale(findRepoRoot()))
	report.add(checkStorage(cfg))
	report.add(checkTimetable(cfg))

	endpoints := endpointsFor(cfg, opts.Endpoints)
	switch {
	case opts.SkipSite:
		report.add(Check{Name: "site", Level: LevelSkip, Detail: "network checks skipped"})
		report.add(checkImageFromCache(cfg, opts))
	default:
		groups, groupCheck := checkGroups(client, endpoints.Groups)
		report.add(groupCheck)
		report.add(checkTeachers(client, endpoints.Teachers))
		report.add(checkCalls(client, endpoints.Calls))
		if groups != nil {
			report.add(checkImage(groups, opts))
		} else {
			report.add(checkImageFromCache(cfg, opts))
		}
	}

	report.finish(started)
	return *report
}

func (r *Report) add(check Check) {
	if check.Level == "" {
		check.Level = LevelOK
	}
	r.Checks = append(r.Checks, check)
}

func (r *Report) finish(started time.Time) {
	r.ElapsedMS = time.Since(started).Milliseconds()
}

func (r Report) Count(level string) int {
	total := 0
	for _, check := range r.Checks {
		if check.Level == level {
			total++
		}
	}
	return total
}

func (r Report) Failed() bool {
	return r.Count(LevelFail) > 0
}

func (r Report) Warnings() int {
	return r.Count(LevelWarn)
}

func (r Report) Summary() string {
	return fmt.Sprintf("%d ok, %d warn, %d fail, %d skipped in %s",
		r.Count(LevelOK), r.Count(LevelWarn), r.Count(LevelFail), r.Count(LevelSkip),
		(time.Duration(r.ElapsedMS) * time.Millisecond).Round(time.Millisecond))
}

func (r Report) Text() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("preflight %s config=%s\n\n", r.Started.Format("02.01.2006 15:04:05"), r.ConfigPath))

	width := 0
	for _, check := range r.Checks {
		if len(check.Name) > width {
			width = len(check.Name)
		}
	}

	for _, check := range r.Checks {
		b.WriteString(fmt.Sprintf("[%s] %-*s  %s\n", check.Level, width, check.Name, check.Detail))
		for _, hint := range check.Hints {
			b.WriteString(fmt.Sprintf("       %s\n", hint))
		}
	}

	b.WriteString("\n")
	b.WriteString(r.Summary())
	b.WriteString("\n")
	return b.String()
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func checkConfig(cfg *config.Config, path string) Check {
	type key struct {
		name  string
		value string
	}

	port := ""
	if cfg.HTTP.Port > 0 {
		port = strconv.Itoa(cfg.HTTP.Port)
	}

	keys := []key{
		{"telegram.token", cfg.Telegram.Token},
		{"parser.endpoints.timetable_group", cfg.Parser.Endpoints.TimetableGroup},
		{"parser.endpoints.timetable_teacher", cfg.Parser.Endpoints.TimetableTeacher},
		{"parser.endpoints.bell_schedule", cfg.Parser.Endpoints.BellSchedule},
		{"db_path", cfg.DBPath},
		{"http.port", port},
	}

	var missing []string
	for _, k := range keys {
		if strings.TrimSpace(k.value) == "" {
			missing = append(missing, k.name)
		}
	}

	var warnings []string
	if len(cfg.Telegram.AdminIDs) == 0 {
		warnings = append(warnings, "telegram.admin_ids is empty, nobody would receive alerts")
	}
	if !cfg.Parser.Enabled {
		warnings = append(warnings, "parser.enabled is false")
	}
	if cfg.Health != nil && cfg.Health.Disabled {
		warnings = append(warnings, "health.disabled is true")
	}
	if cfg.Parser.Calls != nil && !cfg.Parser.Calls.Enabled {
		warnings = append(warnings, "parser.calls.enabled is false")
	}
	if len(cfg.Parser.Endpoints.Team) == 0 {
		warnings = append(warnings, "parser.endpoints.team is empty, teacher names stay short")
	}

	if len(missing) > 0 {
		return Check{
			Name:   "config",
			Level:  LevelFail,
			Detail: fmt.Sprintf("%s: missing %d of %d required keys", filepath.Base(path), len(missing), len(keys)),
			Hints:  []string{"missing: " + strings.Join(missing, ", ")},
		}
	}

	level := LevelOK
	if len(warnings) > 0 {
		level = LevelWarn
	}
	hints := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		hints = append(hints, "warning: "+warning)
	}
	return Check{
		Name:   "config",
		Level:  level,
		Detail: fmt.Sprintf("%d required keys present, %d optional warnings", len(keys), len(warnings)),
		Hints:  hints,
	}
}

func checkCredentials(cfg *config.Config) Check {
	var facts []string
	var hints []string
	level := LevelOK

	escalate := func(next string) {
		if next == LevelFail || level == LevelOK {
			level = next
		}
	}

	switch {
	case strings.TrimSpace(cfg.Telegram.Token) == "":
		facts = append(facts, "telegram token missing")
		escalate(LevelFail)
	case telegramTokenRe.MatchString(cfg.Telegram.Token):
		facts = append(facts, "telegram token shape ok")
	default:
		facts = append(facts, "telegram token shape looks wrong")
		hints = append(hints, "warning: a Telegram bot token looks like 123456789:AA... (44 characters)")
		escalate(LevelWarn)
	}

	email := strings.TrimSpace(cfg.Google.ServiceAccount.ClientEmail)
	privateKey := strings.TrimSpace(cfg.Google.ServiceAccount.PrivateKey)
	switch {
	case email == "" && privateKey == "":
		facts = append(facts, "google service account not configured")
		hints = append(hints, "warning: Google Calendar sync stays off without google.service_account")
		escalate(LevelWarn)
	case privateKey == "":
		facts = append(facts, "google service account key missing")
		hints = append(hints, "warning: set google.service_account.private_key, otherwise Google Calendar sync stays off")
		escalate(LevelWarn)
	case !serviceEmailRe.MatchString(email):
		facts = append(facts, "google service account key ok, client email unusual")
		hints = append(hints, "warning: client_email "+email+" does not look like name@project.iam.gserviceaccount.com")
		escalate(LevelWarn)
	default:
		bits, err := rsaKeyBits(privateKey)
		switch {
		case err != nil:
			facts = append(facts, "google service account key is not usable")
			hints = append(hints, "error: "+err.Error())
			escalate(LevelFail)
		case bits > 0:
			facts = append(facts, fmt.Sprintf("google service account key ok (%d-bit RSA)", bits))
		default:
			facts = append(facts, "google service account key ok")
		}
	}

	oauthID := strings.TrimSpace(cfg.Google.OAuth.ClientID)
	oauthSecret := strings.TrimSpace(cfg.Google.OAuth.ClientSecret)
	if (oauthID == "") != (oauthSecret == "") {
		facts = append(facts, "google oauth pair incomplete")
		hints = append(hints, "warning: google.oauth needs both client_id and client_secret")
		escalate(LevelWarn)
	} else if oauthID != "" {
		facts = append(facts, "google oauth pair present")
	}

	if strings.TrimSpace(cfg.EncryptKey) == "" {
		facts = append(facts, "encrypt_key empty")
		hints = append(hints, "warning: encrypt_key is empty, stored OAuth tokens cannot be decrypted after a restart")
		escalate(LevelWarn)
	}

	return Check{Name: "credentials", Level: level, Detail: strings.Join(facts, "; "), Hints: hints}
}

func rsaKeyBits(privateKey string) (int, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return 0, fmt.Errorf("private_key is not a PEM block; keep the literal \\n escapes inside the YAML value")
	}

	var parsed any
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		return 0, fmt.Errorf("private_key does not parse: %w", err)
	}

	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return 0, nil
	}
	if err := rsaKey.Validate(); err != nil {
		return 0, fmt.Errorf("private_key is not valid: %w", err)
	}
	return rsaKey.N.BitLen(), nil
}

func checkLocale(root string) Check {
	if root == "" {
		return Check{Name: "locale", Level: LevelSkip, Detail: "repository sources not found, locale check skipped"}
	}

	path := filepath.Join(root, localePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Check{Name: "locale", Level: LevelFail, Detail: fmt.Sprintf("%s: %v", localePath, err)}
	}

	var messages map[string]string
	if err := json.Unmarshal(raw, &messages); err != nil {
		return Check{Name: "locale", Level: LevelFail, Detail: localePath + " is not a flat string map", Hints: []string{"error: " + err.Error()}}
	}

	var empty []string
	for key, value := range messages {
		if strings.TrimSpace(value) == "" {
			empty = append(empty, key)
		}
	}
	sort.Strings(empty)

	missing, used := missingLocaleKeys(root, messages)
	if len(missing) > 0 {
		return Check{
			Name:   "locale",
			Level:  LevelFail,
			Detail: fmt.Sprintf("%s: %d of %d keys referenced by the bot are missing", localePath, len(missing), used),
			Hints:  []string{"missing: " + strings.Join(missing, ", ")},
		}
	}

	level := LevelOK
	var hints []string
	if len(empty) > 0 {
		level = LevelWarn
		hints = append(hints, "warning: empty values: "+strings.Join(empty, ", "))
	}
	return Check{
		Name:   "locale",
		Level:  level,
		Detail: fmt.Sprintf("%d keys, %d referenced by the bot all present", len(messages), used),
		Hints:  hints,
	}
}

func missingLocaleKeys(root string, messages map[string]string) ([]string, int) {
	used := make(map[string]bool)
	for _, dir := range []string{telegramSource, noticeSource} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			source, err := os.ReadFile(filepath.Join(root, dir, entry.Name()))
			if err != nil {
				continue
			}
			for _, match := range localeKeyRe.FindAllSubmatch(source, -1) {
				used[string(match[1])] = true
			}
		}
	}

	var missing []string
	for key := range used {
		if _, ok := messages[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing, len(used)
}

func checkStorage(cfg *config.Config) Check {
	paths := []struct {
		name string
		path string
	}{
		{"db_path", cfg.DBPath},
		{"chat_db_path", cfg.ResolvedChatDBPath()},
		{"cache_dir", cfg.ResolvedCacheDir()},
	}

	var facts []string
	var hints []string
	level := LevelOK

	for _, entry := range paths {
		dir := filepath.Dir(entry.path)
		if dir == "" || dir == "." {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			facts = append(facts, entry.name+" unwritable")
			hints = append(hints, fmt.Sprintf("error: %s: %v", dir, err))
			level = LevelFail
			continue
		}

		probe := filepath.Join(dir, storageProbeName)
		if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
			facts = append(facts, entry.name+" unwritable")
			hints = append(hints, fmt.Sprintf("error: %s: %v", dir, err))
			level = LevelFail
			continue
		}
		os.Remove(probe)
		facts = append(facts, entry.path+" writable")
	}

	return Check{Name: "storage", Level: level, Detail: strings.Join(facts, ", "), Hints: hints}
}

func checkTimetable(cfg *config.Config) Check {
	problems := append(slotProblems("weekdays", cfg.Timetable.Weekdays), slotProblems("saturday", cfg.Timetable.Saturday)...)

	if len(cfg.Timetable.Weekdays) == 0 {
		problems = append(problems, "timetable.weekdays is empty, autoskip and calls would not work")
	}

	if len(problems) > 0 {
		hints := make([]string, 0, len(problems))
		for _, problem := range problems {
			hints = append(hints, "warning: "+problem)
		}
		return Check{
			Name:   "timetable",
			Level:  LevelWarn,
			Detail: fmt.Sprintf("%d weekday slots, %d saturday slots, %d problems", len(cfg.Timetable.Weekdays), len(cfg.Timetable.Saturday), len(problems)),
			Hints:  hints,
		}
	}

	return Check{
		Name:   "timetable",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%d weekday slots, %d saturday slots, all ordered", len(cfg.Timetable.Weekdays), len(cfg.Timetable.Saturday)),
	}
}

func slotProblems(name string, slots [][2][2]string) []string {
	var problems []string
	previousEnd := ""
	for i, slot := range slots {
		firstStart, firstEnd := slot[0][0], slot[0][1]
		secondStart, secondEnd := slot[1][0], slot[1][1]
		label := fmt.Sprintf("timetable.%s slot %d", name, i+1)

		singleBlock := firstStart == secondStart && firstEnd == secondEnd
		if !timeOrdered(firstStart, firstEnd) {
			problems = append(problems, fmt.Sprintf("%s runs from %s to %s", label, firstStart, firstEnd))
		}
		if !singleBlock {
			if !timeOrdered(secondStart, secondEnd) {
				problems = append(problems, fmt.Sprintf("%s second half runs from %s to %s", label, secondStart, secondEnd))
			}
			if timeOrdered(firstStart, firstEnd) && timeOrdered(secondStart, secondEnd) && !timeOrdered(firstEnd, secondStart) {
				problems = append(problems, fmt.Sprintf("%s second half starts at %s before the first ends at %s", label, secondStart, firstEnd))
			}
		}
		if previousEnd != "" && !timeOrdered(previousEnd, firstStart) {
			problems = append(problems, fmt.Sprintf("%s starts at %s before the previous slot ends at %s", label, firstStart, previousEnd))
		}
		previousEnd = secondEnd
	}
	return problems
}

func timeOrdered(from, to string) bool {
	fromMinutes, okFrom := clockMinutes(from)
	toMinutes, okTo := clockMinutes(to)
	if !okFrom || !okTo {
		return false
	}
	return fromMinutes < toMinutes
}

func clockMinutes(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	var hours, minutes int
	if _, err := fmt.Sscanf(parts[0], "%d", &hours); err != nil {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minutes); err != nil {
		return 0, false
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, false
	}
	return hours*60 + minutes, true
}

func checkGroups(client *http.Client, url string) (model.Groups, Check) {
	if strings.TrimSpace(url) == "" {
		return nil, Check{Name: "site:groups", Level: LevelSkip, Detail: "parser.endpoints.timetable_group is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return nil, Check{Name: "site:groups", Level: LevelFail, Detail: err.Error()}
	}

	parsed := parser.NewGroupParser(doc)
	groups, err := parsed.Run()
	report := parsed.Report()

	if err != nil {
		return nil, Check{Name: "site:groups", Level: LevelFail, Detail: "parser: " + err.Error()}
	}
	if len(groups) == 0 {
		check := Check{Name: "site:groups", Level: LevelFail, Detail: url + ": the page produced no groups", Hints: probeHints(report)}
		return nil, check
	}

	dated, bad, unordered, withLessons := dayQuality(groupDays(groups))
	hints := probeHints(report)
	level := LevelOK

	if dated == 0 {
		level = LevelFail
		hints = append(hints, "error: no day column carried a dd.MM.yyyy date")
	}
	if len(bad) > 0 {
		level = LevelFail
		hints = append(hints, "error: day labels are not dd.MM.yyyy: "+strings.Join(limit(bad, 3), "; "))
	}
	if len(unordered) > 0 {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: days out of order: "+strings.Join(limit(unordered, 3), "; "))
	}
	if withLessons == 0 {
		level = LevelFail
		hints = append(hints, "error: no group had a single lesson")
	}
	if rangeHint := dateRangeHint(groupDays(groups)); rangeHint != "" {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: "+rangeHint)
	}
	if len(report.Fallbacks) > 0 && level == LevelOK {
		level = LevelWarn
	}

	return groups, Check{
		Name:   "site:groups",
		Level:  level,
		Detail: fmt.Sprintf("%d groups, %d with lessons, %d dated day columns", len(groups), withLessons, dated),
		Hints:  hints,
	}
}

func checkTeachers(client *http.Client, url string) Check {
	if strings.TrimSpace(url) == "" {
		return Check{Name: "site:teachers", Level: LevelSkip, Detail: "parser.endpoints.timetable_teacher is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: err.Error()}
	}

	parsed := parser.NewTeacherParser(doc)
	teachers, err := parsed.Run()
	report := parsed.Report()

	if err != nil {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: "parser: " + err.Error()}
	}
	if len(teachers) == 0 {
		return Check{Name: "site:teachers", Level: LevelFail, Detail: url + ": the page produced no teachers", Hints: probeHints(report)}
	}

	dated, bad, unordered, withLessons := dayQuality(teacherDays(teachers))
	hints := probeHints(report)
	level := LevelOK

	if dated == 0 {
		level = LevelFail
		hints = append(hints, "error: no day column carried a dd.MM.yyyy date")
	}
	if len(bad) > 0 {
		level = LevelFail
		hints = append(hints, "error: day labels are not dd.MM.yyyy: "+strings.Join(limit(bad, 3), "; "))
	}
	if len(unordered) > 0 && level == LevelOK {
		level = LevelWarn
		hints = append(hints, "warning: days out of order: "+strings.Join(limit(unordered, 3), "; "))
	}
	if withLessons == 0 {
		level = LevelFail
		hints = append(hints, "error: no teacher had a single lesson")
	}

	return Check{
		Name:   "site:teachers",
		Level:  level,
		Detail: fmt.Sprintf("%d teachers, %d with lessons, %d dated day columns", len(teachers), withLessons, dated),
		Hints:  hints,
	}
}

func checkCalls(client *http.Client, url string) Check {
	if strings.TrimSpace(url) == "" {
		return Check{Name: "site:calls", Level: LevelSkip, Detail: "parser.endpoints.bell_schedule is empty"}
	}

	doc, err := parser.FetchDocument(client, url)
	if err != nil {
		return Check{Name: "site:calls", Level: LevelFail, Detail: err.Error()}
	}

	variants, report := parser.ParseCallsVariants(doc)
	hints := probeHints(report)
	if len(variants) == 0 || len(variants[0].Schedule.Weekdays) == 0 {
		hints = append(hints, "error: no bell schedule slot was recognised")
		return Check{Name: "site:calls", Level: LevelFail, Detail: url + ": the page produced no bell schedule", Hints: hints}
	}

	weekdays := variants[0].Schedule.Weekdays
	saturday := variants[0].Schedule.Saturday

	problems := append(slotProblems("site weekdays", weekdays), slotProblems("site saturday", saturday)...)

	level := LevelOK
	if len(problems) > 0 {
		level = LevelFail
		for _, problem := range problems {
			hints = append(hints, "error: "+problem)
		}
	}
	if len(saturday) == 0 {
		if level == LevelOK {
			level = LevelWarn
		}
		hints = append(hints, "warning: the site published no saturday slots")
	}

	detail := fmt.Sprintf("%d variant(s), %d weekday slots, %d saturday slots", len(variants), len(weekdays), len(saturday))
	if len(variants) > 1 {
		names := make([]string, 0, len(variants))
		for _, variant := range variants {
			if strings.TrimSpace(variant.Name) != "" {
				names = append(names, variant.Name)
			}
		}
		if len(names) > 0 {
			detail += " (" + strings.Join(names, ", ") + ")"
		}
	}

	return Check{Name: "site:calls", Level: level, Detail: detail, Hints: hints}
}

func checkImage(groups model.Groups, opts Options) Check {
	if opts.SkipImage {
		return Check{Name: "image", Level: LevelSkip, Detail: "image rendering skipped"}
	}
	if len(groups) == 0 {
		return Check{Name: "image", Level: LevelSkip, Detail: "no freshly parsed groups to render"}
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		days := entryDays(groups[name])
		if len(days) == 0 {
			continue
		}
		return renderCheck(name, days, opts)
	}
	return Check{Name: "image", Level: LevelSkip, Detail: "no group carried a day to render"}
}

func checkImageFromCache(cfg *config.Config, opts Options) Check {
	if opts.SkipImage {
		return Check{Name: "image", Level: LevelSkip, Detail: "image rendering skipped"}
	}

	cached, err := cache.New(cfg.ResolvedCacheDir())
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "cache: " + err.Error()}
	}

	groups := cached.GetGroups()
	if len(groups) == 0 {
		return Check{Name: "image", Level: LevelSkip, Detail: "no cached groups to render"}
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		days := entryDays(groups[name])
		if len(days) == 0 {
			continue
		}
		check := renderCheck(name, days, opts)
		if check.Level == LevelOK {
			check.Detail = "cached " + check.Detail
		}
		return check
	}
	return Check{Name: "image", Level: LevelSkip, Detail: "no cached group carried a day to render"}
}

func renderCheck(name string, days []map[string]any, opts Options) Check {
	font := imagepkg.FontPath()
	if font == "" {
		return Check{Name: "image", Level: LevelFail, Detail: "no loadable font found on this system", Hints: []string{"error: install font-dejavu in the runtime image"}}
	}

	dir := opts.ImageDir
	remove := false
	if dir == "" {
		temp, err := os.MkdirTemp("", "preflight-image")
		if err != nil {
			return Check{Name: "image", Level: LevelFail, Detail: "temp dir: " + err.Error()}
		}
		dir = temp
		remove = true
	}
	if remove {
		defer os.RemoveAll(dir)
	}

	path, err := imagepkg.RenderGroupDays(name, days, dir)
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "render: " + err.Error()}
	}

	file, err := os.Open(path)
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "open rendered image: " + err.Error()}
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "stat rendered image: " + err.Error()}
	}

	config, err := png.DecodeConfig(file)
	if err != nil {
		return Check{Name: "image", Level: LevelFail, Detail: "the rendered file is not a readable PNG: " + err.Error()}
	}
	if config.Width <= 0 || config.Height <= 0 || info.Size() == 0 {
		return Check{Name: "image", Level: LevelFail, Detail: "the rendered PNG is empty"}
	}

	return Check{
		Name:   "image",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%q -> %dx%d px, %d KB, font %s", name, config.Width, config.Height, info.Size()/1024, filepath.Base(font)),
	}
}

func groupDays(groups model.Groups) []dayEntry {
	entries := make([]dayEntry, 0, len(groups))
	for name, group := range groups {
		if group == nil {
			continue
		}
		for _, day := range group.Days {
			entries = append(entries, dayEntry{
				owner:    name,
				label:    day.Day,
				lessons:  len(day.Lessons),
				hasOwner: true,
			})
		}
	}
	return entries
}

func teacherDays(teachers model.Teachers) []dayEntry {
	entries := make([]dayEntry, 0, len(teachers))
	for name, teacher := range teachers {
		if teacher == nil {
			continue
		}
		for _, day := range teacher.Days {
			entries = append(entries, dayEntry{
				owner:    name,
				label:    day.Day,
				lessons:  len(day.Lessons),
				hasOwner: true,
			})
		}
	}
	return entries
}

type dayEntry struct {
	owner    string
	label    string
	lessons  int
	hasOwner bool
}

func dayQuality(entries []dayEntry) (dated int, bad []string, unordered []string, withLessons int) {
	owners := make(map[string]bool)

	for _, entry := range entries {
		if _, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label)); err != nil {
			bad = append(bad, fmt.Sprintf("%s: %q", entry.owner, entry.label))
			continue
		}
		dated++
		if entry.lessons > 0 {
			owners[entry.owner] = true
		}
	}
	withLessons = len(owners)

	for _, owner := range ownersOf(entries) {
		previous := time.Time{}
		for _, entry := range entries {
			if entry.owner != owner {
				continue
			}
			parsed, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label))
			if err != nil {
				continue
			}
			if !previous.IsZero() && parsed.Before(previous) {
				unordered = append(unordered, fmt.Sprintf("%s: %s before %s", owner, parsed.Format("02.01.2006"), previous.Format("02.01.2006")))
			}
			previous = parsed
		}
	}
	return dated, bad, unordered, withLessons
}

func ownersOf(entries []dayEntry) []string {
	seen := make(map[string]bool)
	var owners []string
	for _, entry := range entries {
		if entry.hasOwner && !seen[entry.owner] {
			seen[entry.owner] = true
			owners = append(owners, entry.owner)
		}
	}
	return owners
}

func dateRangeHint(entries []dayEntry) string {
	var first, last time.Time
	for _, entry := range entries {
		parsed, err := time.Parse("02.01.2006", strings.TrimSpace(entry.label))
		if err != nil {
			continue
		}
		if first.IsZero() || parsed.Before(first) {
			first = parsed
		}
		if parsed.After(last) {
			last = parsed
		}
	}
	if first.IsZero() {
		return ""
	}

	today := time.Now()
	todayIndex := utils.DayIndexFromDate(today)
	if utils.DayIndexFromDate(last) < todayIndex {
		return fmt.Sprintf("the whole timetable ends at %s, the site has not published the current week", last.Format("02.01.2006"))
	}
	if utils.DayIndexFromDate(first) > todayIndex+30 {
		return fmt.Sprintf("the timetable starts at %s, far in the future", first.Format("02.01.2006"))
	}
	return ""
}

func probeHints(report parser.Report) []string {
	var hints []string
	for _, probe := range report.Failing() {
		hints = append(hints, fmt.Sprintf("error: no data for %q (expected %s)", probe.Selector, probe.Expected))
	}
	for _, fallback := range report.Fallbacks {
		hints = append(hints, "warning: fallback used: "+fallback)
	}
	for _, warning := range report.Warnings {
		hints = append(hints, "warning: "+warning)
	}
	return hints
}

func entryDays(data any) []map[string]any {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil
	}

	days, _ := entry["days"].([]any)
	result := make([]map[string]any, 0, len(days))
	for _, day := range days {
		if m, ok := day.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func endpointsFor(cfg *config.Config, override Endpoints) Endpoints {
	endpoints := Endpoints{
		Groups:   cfg.Parser.Endpoints.TimetableGroup,
		Teachers: cfg.Parser.Endpoints.TimetableTeacher,
		Calls:    cfg.Parser.Endpoints.BellSchedule,
	}
	if override.Groups != "" {
		endpoints.Groups = override.Groups
	}
	if override.Teachers != "" {
		endpoints.Teachers = override.Teachers
	}
	if override.Calls != "" {
		endpoints.Calls = override.Calls
	}
	return endpoints
}

func limit(values []string, max int) []string {
	if len(values) <= max {
		return values
	}
	return values[:max]
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, localePath)); err == nil {
				return dir
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
