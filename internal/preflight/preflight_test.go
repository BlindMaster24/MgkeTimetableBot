package preflight

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	imagepkg "github.com/blindmaster24/MgkeTimetableBot/internal/image"
)

const callsHTML = `<html><body>
<div class="entry"><div class="content">
<h1>Расписание звонков</h1>
<table class="table table-bordered">
<thead><tr><th colspan="2">1 смена</th></tr></thead>
<tbody>
<tr><td>1 пара</td><td>8.00 &ndash; 8.45<br/>8.55 &ndash; 9.40</td></tr>
<tr><td>2 пара</td><td>9.50 &ndash; 10.35<br/>10.45 &ndash; 11.30</td></tr>
</tbody>
</table>
</div></div>
</body></html>`

func groupHTML(date string) string {
	return `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, ` + date + `</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td class="sub">101</td></tr>
<tr><th>2</th><td>Физика<br>(Пр)<br>Петров</td><td class="sub">202</td></tr>
</table>
</div></div>
</body></html>`
}

func groupHTMLWithoutDates() string {
	return `<html><body>
<div class="entry"><div class="content">
<h2>Группа - 63ТП</h2>
<table border="1">
<tr><th rowspan="2">№</th><th colspan="2">Понедельник</th></tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>Математика<br>(Лек)<br>Иванов</td><td class="sub">101</td></tr>
</table>
</div></div>
</body></html>`
}

func teacherHTML(date string) string {
	return `<html><body>
<div class="entry"><div class="content">
<h2>Преподаватель - Иванов А.А.</h2>
<table border="1">
<tr>
<th rowspan="2">№</th>
<th colspan="2">Понедельник, ` + date + `</th>
</tr>
<tr><th class="sub">D</th><th class="sub">A</th></tr>
<tr><th>1</th><td>63ТП-Математика<br>(Лек)</td><td class="sub">101</td></tr>
</table>
</div></div>
</body></html>`
}

type siteFixtures struct {
	server       *httptest.Server
	groupsPath   string
	teachersPath string
	callsPath    string
}

func startSite(t *testing.T, groups, teachers string) *siteFixtures {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/groups", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, groups)
	})
	mux.HandleFunc("/teachers", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, teachers)
	})
	mux.HandleFunc("/calls", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, callsHTML)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &siteFixtures{
		server:       server,
		groupsPath:   server.URL + "/groups",
		teachersPath: server.URL + "/teachers",
		callsPath:    server.URL + "/calls",
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func siteConfig(t *testing.T, site *siteFixtures) string {
	t.Helper()

	dir := t.TempDir()
	body := fmt.Sprintf(`db_path: %q
chat_db_path: %q
cache_dir: %q
http:
  port: 8081
telegram:
  token: "123456789:AAF-abcdefghijklmnopqrstuvwxyz012345"
  admin_ids: [1]
encrypt_key: "local-key"
parser:
  enabled: true
  endpoints:
    timetable_group: %q
    timetable_teacher: %q
    team: []
    bell_schedule: %q
  calls:
    enabled: true
timetable:
  weekdays:
    - [["09:00", "09:45"], ["09:55", "10:40"]]
  saturday:
    - [["09:00", "09:45"], ["09:55", "10:40"]]
`,
		filepath.Join(dir, "sqlite3.db"),
		filepath.Join(dir, "bot_chats.db"),
		filepath.Join(dir, "cache"),
		site.groupsPath,
		site.teachersPath,
		site.callsPath,
	)
	return writeConfig(t, strings.ReplaceAll(body, "\\", "\\"))
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()

	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("check %q missing from report: %s", name, report.Text())
	return Check{}
}

func todayPlus(days int) string {
	return time.Now().AddDate(0, 0, days).Format("02.01.2006")
}

func TestRun_LiveSiteChecks(t *testing.T) {
	site := startSite(t, groupHTML(todayPlus(0)), teacherHTML(todayPlus(1)))

	report := Run(Options{
		ConfigPath: siteConfig(t, site),
		HTTPClient: site.server.Client(),
	})

	for _, name := range []string{"config", "credentials", "locale", "storage", "timetable", "site:groups", "site:teachers", "site:calls"} {
		check := findCheck(t, report, name)
		if check.Level == LevelFail {
			t.Errorf("%s failed: %s %v", name, check.Detail, check.Hints)
		}
	}
	if report.Failed() {
		t.Errorf("expected a passing report, got %s", report.Text())
	}
}

func TestRun_ReportsParsedCounts(t *testing.T) {
	site := startSite(t, groupHTML(todayPlus(0)), teacherHTML(todayPlus(1)))

	report := Run(Options{ConfigPath: siteConfig(t, site), HTTPClient: site.server.Client()})

	groups := findCheck(t, report, "site:groups")
	if !strings.Contains(groups.Detail, "1 groups") || !strings.Contains(groups.Detail, "1 with lessons") {
		t.Errorf("unexpected group detail: %q", groups.Detail)
	}
	calls := findCheck(t, report, "site:calls")
	if !strings.Contains(calls.Detail, "2 weekday slots") {
		t.Errorf("unexpected calls detail: %q", calls.Detail)
	}
}

func TestRun_FailsWhenDayColumnsHaveNoDate(t *testing.T) {
	site := startSite(t, groupHTMLWithoutDates(), teacherHTML(todayPlus(1)))

	report := Run(Options{ConfigPath: siteConfig(t, site), HTTPClient: site.server.Client()})

	groups := findCheck(t, report, "site:groups")
	if groups.Level != LevelFail {
		t.Fatalf("expected a failing group check, got %s", groups.Detail)
	}
	if !containsHint(groups.Hints, "dd.MM.yyyy") {
		t.Errorf("the failure must name the date format: %v", groups.Hints)
	}
	if !report.Failed() {
		t.Error("the report must fail when dates are not parseable")
	}
}

func TestRun_FailsWhenTheSiteIsUnreachable(t *testing.T) {
	site := startSite(t, groupHTML(todayPlus(0)), teacherHTML(todayPlus(1)))
	path := siteConfig(t, site)
	site.server.Close()

	report := Run(Options{ConfigPath: path, HTTPClient: &http.Client{Timeout: 2 * time.Second}})

	if !report.Failed() {
		t.Fatalf("expected a failure for an unreachable site, got %s", report.Text())
	}
	if check := findCheck(t, report, "site:groups"); check.Level != LevelFail {
		t.Errorf("expected site:groups to fail, got %s", check.Detail)
	}
}

func TestRun_SkipSiteUsesTheCachedData(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	cached, err := cache.New(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	cached.SetGroups(map[string]any{
		"777": map[string]any{
			"group": "777",
			"days": []any{
				map[string]any{"day": todayPlus(0), "lessons": []any{
					map[string]any{"lesson": "История", "type": "Лек", "cabinet": "404"},
				}},
			},
		},
	}, "hash")
	if err := cached.Save(); err != nil {
		t.Fatal(err)
	}

	path := writeConfig(t, fmt.Sprintf(`db_path: %q
chat_db_path: %q
cache_dir: %q
http:
  port: 8081
telegram:
  token: "123456789:AAF-abcdefghijklmnopqrstuvwxyz012345"
  admin_ids: [1]
encrypt_key: "local-key"
parser:
  endpoints:
    timetable_group: "https://example.invalid/groups"
    timetable_teacher: "https://example.invalid/teachers"
    bell_schedule: "https://example.invalid/calls"
`,
		filepath.Join(dir, "sqlite3.db"),
		filepath.Join(dir, "bot_chats.db"),
		cacheDir,
	))

	report := Run(Options{ConfigPath: path, SkipSite: true})
	if report.Failed() {
		t.Fatalf("skip-site run must not fail: %s", report.Text())
	}
	if check := findCheck(t, report, "site"); check.Level != LevelSkip {
		t.Errorf("expected the site check to be skipped, got %s", check.Detail)
	}
	image := findCheck(t, report, "image")
	if image.Level == LevelFail {
		t.Fatalf("cached image render failed: %s %v", image.Detail, image.Hints)
	}
	if image.Level == LevelOK && !strings.Contains(image.Detail, "cached") {
		t.Errorf("the image check must say it used the cache: %q", image.Detail)
	}
}

func TestRun_FailsWhenARequiredKeyIsMissing(t *testing.T) {
	path := writeConfig(t, `chat_db_path: "./x.db"
http:
  port: 8081
telegram:
  token: ""
parser:
  endpoints:
    timetable_group: "https://example.invalid/groups"
`)

	report := Run(Options{ConfigPath: path, SkipSite: true})

	cfg := findCheck(t, report, "config")
	if cfg.Level != LevelFail {
		t.Fatalf("expected the config check to fail, got %s", cfg.Detail)
	}
	if !containsHint(cfg.Hints, "telegram.token") {
		t.Errorf("the failure must name the missing key: %v", cfg.Hints)
	}
	if !report.Failed() {
		t.Error("a missing token must fail the run")
	}
}

func TestRun_FailsOnAnUnreadableConfig(t *testing.T) {
	report := Run(Options{ConfigPath: filepath.Join(t.TempDir(), "absent.yaml"), SkipSite: true})

	if !report.Failed() {
		t.Fatalf("expected a failure, got %s", report.Text())
	}
	if len(report.Checks) != 1 || report.Checks[0].Name != "config" {
		t.Errorf("an unreadable config must stop after the first check: %v", report.Checks)
	}
}

func TestCheckCredentials_ServiceAccountKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))

	cfg := &config.Config{}
	cfg.Telegram.Token = "123456789:AAF-abcdefghijklmnopqrstuvwxyz012345"
	cfg.Google.ServiceAccount.ClientEmail = "calendar@project.iam.gserviceaccount.com"
	cfg.Google.ServiceAccount.PrivateKey = pemKey
	cfg.EncryptKey = "local-key"

	check := checkCredentials(cfg)
	if check.Level != LevelOK {
		t.Fatalf("expected a healthy credential check, got %s: %v", check.Detail, check.Hints)
	}
	if !strings.Contains(check.Detail, "2048-bit RSA") {
		t.Errorf("expected the key size in the detail: %q", check.Detail)
	}

	cfg.Google.ServiceAccount.PrivateKey = "not-a-pem"
	failed := checkCredentials(cfg)
	if failed.Level != LevelFail {
		t.Fatalf("expected an invalid key to fail, got %s", failed.Detail)
	}
	if !containsHint(failed.Hints, "not a PEM block") {
		t.Errorf("expected a PEM hint: %v", failed.Hints)
	}
}

func TestCheckCredentials_WarnsWhenGoogleIsHalfConfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.Token = "123456789:AAF-abcdefghijklmnopqrstuvwxyz012345"
	cfg.EncryptKey = "local-key"
	cfg.Google.ServiceAccount.ClientEmail = "calendar@project.iam.gserviceaccount.com"

	check := checkCredentials(cfg)
	if check.Level != LevelWarn {
		t.Fatalf("expected a warning, got %s: %v", check.Detail, check.Hints)
	}
	if !containsHint(check.Hints, "private_key") {
		t.Errorf("expected the hint to name the missing key: %v", check.Hints)
	}

	emptyCfg, err := config.Load(writeConfig(t, "telegram:\n  token: \"123456789:AAF-abcdefghijklmnopqrstuvwxyz012345\"\nencrypt_key: \"local-key\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	empty := checkCredentials(emptyCfg)
	if empty.Level != LevelWarn {
		t.Fatalf("an unconfigured service account must warn, got %s", empty.Detail)
	}
}

func TestCheckCredentials_WarnsOnAPartialOAuthPair(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.Token = "123456789:AAF-abcdefghijklmnopqrstuvwxyz012345"
	cfg.EncryptKey = "local-key"
	cfg.Google.OAuth.ClientID = "client-id.apps.googleusercontent.com"

	check := checkCredentials(cfg)
	if check.Level != LevelWarn {
		t.Fatalf("expected a warning, got %s", check.Detail)
	}
	if !containsHint(check.Hints, "client_secret") {
		t.Errorf("expected the missing half in the hints: %v", check.Hints)
	}
}

func TestCheckLocale_FindsMissingKeys(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(localePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	locale := `{"welcome": "Привет", "empty": "   "}`
	if err := os.WriteFile(filepath.Join(root, localePath), []byte(locale), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, telegramSource), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package telegram\n\nfunc x() { loc(\"welcome\"); loc(\"gone_key\") }\n"
	if err := os.WriteFile(filepath.Join(root, telegramSource, "bot.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	check := checkLocale(root)
	if check.Level != LevelFail {
		t.Fatalf("expected a failure for a missing key, got %s", check.Detail)
	}
	if !containsHint(check.Hints, "gone_key") {
		t.Errorf("expected the missing key in the hints: %v", check.Hints)
	}
}

func TestCheckLocale_WarnsOnEmptyValues(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(localePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, localePath), []byte(`{"welcome": "Привет", "empty": "   "}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, telegramSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, telegramSource, "bot.go"), []byte("package telegram\n\nfunc x() { loc(\"welcome\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	check := checkLocale(root)
	if check.Level != LevelWarn {
		t.Fatalf("expected a warning, got %s", check.Detail)
	}
	if !containsHint(check.Hints, "empty") {
		t.Errorf("expected the empty key in the hints: %v", check.Hints)
	}
}

func TestCheckLocale_SkipsWithoutSources(t *testing.T) {
	check := checkLocale("")
	if check.Level != LevelSkip {
		t.Fatalf("expected a skip, got %s", check.Detail)
	}
}

func TestSlotProblems(t *testing.T) {
	tests := []struct {
		name  string
		slots [][2][2]string
		want  int
	}{
		{"ordered", [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}, {{"10:50", "11:35"}, {"11:45", "12:30"}}}, 0},
		{"backwards", [][2][2]string{{{"09:45", "09:00"}, {"09:55", "10:40"}}}, 1},
		{"overlapping", [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}, {{"10:00", "10:45"}, {"10:55", "11:40"}}}, 1},
		{"garbage", [][2][2]string{{{"утро", "09:45"}, {"09:55", "10:40"}}}, 1},
		{"halves mixed up", [][2][2]string{{{"09:55", "10:40"}, {"09:00", "09:45"}}}, 1},
		{"single blocks", [][2][2]string{{{"08:00", "09:20"}, {"08:00", "09:20"}}, {{"09:30", "10:50"}, {"09:30", "10:50"}}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(slotProblems("weekdays", tt.slots)); got != tt.want {
				t.Errorf("expected %d problems, got %d", tt.want, got)
			}
		})
	}
}

func TestDayQuality(t *testing.T) {
	entries := []dayEntry{
		{owner: "63ТП", label: todayPlus(1), lessons: 2, hasOwner: true},
		{owner: "63ТП", label: todayPlus(0), lessons: 0, hasOwner: true},
		{owner: "64ИС", label: "Понедельник, 01.09.2026", lessons: 1, hasOwner: true},
	}

	dated, bad, unordered, withLessons := dayQuality(entries)
	if dated != 2 {
		t.Errorf("expected 2 dated columns, got %d", dated)
	}
	if len(bad) != 1 || !strings.Contains(bad[0], "64ИС") {
		t.Errorf("expected the long label to be reported: %v", bad)
	}
	if len(unordered) != 1 {
		t.Errorf("expected one out-of-order day, got %v", unordered)
	}
	if withLessons != 1 {
		t.Errorf("expected a single owner with lessons, got %d", withLessons)
	}
}

func TestReportSummaryAndText(t *testing.T) {
	report := Report{ConfigPath: "configs/config.yaml", Started: time.Now(), ElapsedMS: 1200}
	report.add(Check{Name: "config", Level: LevelOK, Detail: "fine"})
	report.add(Check{Name: "site:groups", Level: LevelFail, Detail: "no groups", Hints: []string{"error: no data"}})
	report.add(Check{Name: "image", Level: LevelSkip, Detail: "skipped"})

	if !report.Failed() {
		t.Error("a failing check must fail the report")
	}
	if report.Warnings() != 0 {
		t.Errorf("expected no warnings, got %d", report.Warnings())
	}

	text := report.Text()
	for _, want := range []string{"[ok] config", "[fail] site:groups", "error: no data", "1 ok, 0 warn, 1 fail, 1 skipped"} {
		if !strings.Contains(text, want) {
			t.Errorf("report text is missing %q:\n%s", want, text)
		}
	}

	data, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var restored Report
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Checks) != 3 {
		t.Errorf("expected the JSON report to carry every check, got %d", len(restored.Checks))
	}
}

func TestRenderCheckReportsAFailureWithoutFonts(t *testing.T) {
	if imagepkg.FontPath() == "" {
		t.Skip("no font on this machine, the image check cannot pass here")
	}

	check := renderCheck("777", []map[string]any{{
		"day":     todayPlus(0),
		"lessons": []any{map[string]any{"lesson": "История", "type": "Лек", "cabinet": "404"}},
	}}, Options{})

	if check.Level != LevelOK {
		t.Fatalf("expected a rendered PNG, got %s: %v", check.Detail, check.Hints)
	}
	if !strings.Contains(check.Detail, "px") {
		t.Errorf("expected the image size in the detail: %q", check.Detail)
	}
}

func webhookCheck(t *testing.T, enabled bool, url, secret string) Check {
	t.Helper()

	site := startSite(t, groupHTML(todayPlus(0)), teacherHTML(todayPlus(1)))
	path := siteConfig(t, site)

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block := fmt.Sprintf("  webhook:\n    enabled: %v\n    url: %q\n    secret_token: %q\n", enabled, url, secret)
	patched := strings.Replace(string(body), "  admin_ids: [1]\n", "  admin_ids: [1]\n"+block, 1)
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(Options{ConfigPath: path, SkipSite: true, SkipImage: true})
	return findCheck(t, report, "webhook")
}

func TestRun_SkipsTheWebhookCheckForLongPolling(t *testing.T) {
	check := webhookCheck(t, false, "", "")
	if check.Level != LevelSkip {
		t.Fatalf("expected a skip, got %s: %s", check.Level, check.Detail)
	}
}

func TestRun_FailsTheWebhookCheckOnABrokenConfiguration(t *testing.T) {
	check := webhookCheck(t, true, "http://mgke.example.com", "secret")
	if check.Level != LevelFail {
		t.Fatalf("expected a failure, got %s: %s", check.Level, check.Detail)
	}
	if !strings.Contains(check.Detail, "https") {
		t.Errorf("expected the https requirement in the detail: %q", check.Detail)
	}
}

func TestRun_WarnsAboutAMissingWebhookSecret(t *testing.T) {
	check := webhookCheck(t, true, "https://mgke.example.com", "")
	if check.Level != LevelWarn {
		t.Fatalf("expected a warning, got %s: %s", check.Level, check.Detail)
	}
	if !containsHint(check.Hints, "secret_token") {
		t.Errorf("expected a secret token hint, got %v", check.Hints)
	}
}

func TestRun_AcceptsACompleteWebhookConfiguration(t *testing.T) {
	check := webhookCheck(t, true, "https://mgke.example.com", "webhook-secret")
	if check.Level != LevelOK {
		t.Fatalf("expected a pass, got %s: %s %v", check.Level, check.Detail, check.Hints)
	}
}

func containsHint(hints []string, needle string) bool {
	for _, hint := range hints {
		if strings.Contains(hint, needle) {
			return true
		}
	}
	return false
}
