package preflight

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
)

const probeMetricsToken = "preflight-metrics-token-42"

func metricsConfig(enabled bool, token string) *config.Config {
	return &config.Config{Metrics: config.MetricsConfig{Enabled: enabled, Token: token}}
}

func requireMetricsLevel(t *testing.T, check Check, level string) {
	t.Helper()

	if check.Level != level {
		t.Fatalf("level = %s, want %s (%s %v)", check.Level, level, check.Detail, check.Hints)
	}
}

func metricsCheckMentions(check Check, text string) bool {
	return strings.Contains(check.Detail, text) || containsHint(check.Hints, text)
}

func TestMetricsCheckSkipsWhileDisabled(t *testing.T) {
	requireMetricsLevel(t, checkMetrics(metricsConfig(false, "")), LevelSkip)
	requireMetricsLevel(t, checkMetrics(nil), LevelSkip)
}

func TestMetricsCheckServesTheRealRoute(t *testing.T) {
	check := checkMetrics(metricsConfig(true, probeMetricsToken))

	requireMetricsLevel(t, check, LevelOK)
	if !strings.Contains(check.Detail, "families") {
		t.Errorf("the detail must report the served families: %q", check.Detail)
	}
	if len(check.Hints) != 0 {
		t.Errorf("a passing probe carries no hints: %v", check.Hints)
	}

	withoutToken := checkMetrics(metricsConfig(true, ""))
	requireMetricsLevel(t, withoutToken, LevelWarn)
	if !containsHint(withoutToken.Hints, "metrics.token") {
		t.Errorf("an open metrics route must be called out: %v", withoutToken.Hints)
	}
}

func TestMetricsCheckNeverPrintsTheToken(t *testing.T) {
	for name, check := range map[string]Check{
		"passing": checkMetrics(metricsConfig(true, probeMetricsToken)),
		"refused": checkMetrics(metricsConfig(true, probeMetricsToken)),
		"foreign": runMetricsCheck(metricsConfig(true, probeMetricsToken), metricsProbeOptions{
			handler: func(string, string, string) (http.Handler, error) {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					io.WriteString(w, "# HELP mgke_build_info build\n# TYPE mgke_build_info gauge\nmgke_build_info 1\n")
				}), nil
			},
		}),
	} {
		if strings.Contains(check.Detail, probeMetricsToken) {
			t.Errorf("%s: the detail carries the token", name)
		}
		if containsHint(check.Hints, probeMetricsToken) {
			t.Errorf("%s: the hints carry the token: %v", name, check.Hints)
		}
	}
}

func TestMetricsCheckFailsWhenTheRouteIgnoresTheToken(t *testing.T) {
	check := runMetricsCheck(metricsConfig(true, probeMetricsToken), metricsProbeOptions{
		handler: func(string, string, string) (http.Handler, error) {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, "# HELP mgke_build_info build\n# TYPE mgke_build_info gauge\nmgke_build_info 1\n")
			}), nil
		},
	})

	requireMetricsLevel(t, check, LevelFail)
	if !metricsCheckMentions(check, "401") {
		t.Errorf("the failure must say what the anonymous scrape answered: %s %v", check.Detail, check.Hints)
	}
}

func TestMetricsCheckFailsWhenTheRouteRefusesTheToken(t *testing.T) {
	check := runMetricsCheck(metricsConfig(true, probeMetricsToken), metricsProbeOptions{
		handler: func(string, string, string) (http.Handler, error) {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}), nil
		},
	})

	requireMetricsLevel(t, check, LevelFail)
	if !metricsCheckMentions(check, "with the token answered 401") {
		t.Errorf("the failure must name the refused scrape: %s %v", check.Detail, check.Hints)
	}
}

func TestMetricsCheckReportsAPanickingRoute(t *testing.T) {
	panicHandler := func(string, string, string) (http.Handler, error) {
		panic("bogus metrics path")
	}

	if _, err := routableMetricsHandler(panicHandler, "", "/metrics", ""); err == nil {
		t.Fatal("a handler factory that panics must come back as an error")
	} else if !strings.Contains(err.Error(), "not routable") {
		t.Errorf("unexpected error: %v", err)
	}

	check := runMetricsCheck(metricsConfig(true, ""), metricsProbeOptions{handler: panicHandler})
	requireMetricsLevel(t, check, LevelFail)
	if !containsHint(check.Hints, "not routable") {
		t.Errorf("the failure must say the path could not be served: %v", check.Hints)
	}
}

func brokenMetricsExposition(t *testing.T, rewrite func(string) string) Check {
	t.Helper()

	return brokenMetricsExpositionWith(t, metricsConfig(true, ""), rewrite)
}

func brokenMetricsExpositionWith(t *testing.T, cfg *config.Config, rewrite func(string) string) Check {
	t.Helper()

	return runMetricsCheck(cfg, metricsProbeOptions{
		handler: func(cacheDir, path, token string) (http.Handler, error) {
			inner, err := metricsAPIHandler(cfg, cacheDir, path, token)
			if err != nil {
				return nil, err
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				recorder := httptest.NewRecorder()
				inner.ServeHTTP(recorder, r)

				for name, values := range recorder.Header() {
					if strings.EqualFold(name, "Content-Length") {
						continue
					}
					for _, value := range values {
						w.Header().Add(name, value)
					}
				}
				w.WriteHeader(recorder.Code)
				io.WriteString(w, rewrite(recorder.Body.String()))
			}), nil
		},
		root: func() string { return "" },
	})
}

func dropLines(prefix string) func(string) string {
	return func(body string) string {
		var kept []string
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, prefix) {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
}

func TestMetricsCheckFailsOnAMissingFamily(t *testing.T) {
	check := brokenMetricsExposition(t, func(body string) string {
		var kept []string
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "mgke_parser_lag_seconds") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	})

	requireMetricsLevel(t, check, LevelFail)
	if !containsHint(check.Hints, "mgke_parser_lag_seconds is missing from the scrape") {
		t.Errorf("the failure must name the dropped family: %v", check.Hints)
	}
	for _, hint := range []string{"metrics.enabled", "/api/health"} {
		if !containsHint(check.Hints, hint) {
			t.Errorf("the failure must point at %s: %v", hint, check.Hints)
		}
	}
}

func TestMetricsCheckFailsOnBrokenExpositions(t *testing.T) {
	cases := map[string]struct {
		rewrite func(string) string
		want    string
	}{
		"missing help": {
			rewrite: dropLines("# HELP mgke_uptime_seconds "),
			want:    "mgke_uptime_seconds is served without a HELP line",
		},
		"untyped family": {
			rewrite: dropLines("# TYPE mgke_uptime_seconds "),
			want:    "mgke_uptime_seconds is served as UNTYPED",
		},
		"repeated series": {
			rewrite: func(body string) string {
				return strings.Replace(body, "mgke_uptime_seconds 0\n", "mgke_uptime_seconds 0\nmgke_uptime_seconds 0\n", 1)
			},
			want: "repeats the series mgke_uptime_seconds",
		},
		"not an exposition": {
			rewrite: func(string) string { return "the route answered something else\n" },
			want:    "not a valid prometheus exposition",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			check := brokenMetricsExposition(t, tc.rewrite)
			requireMetricsLevel(t, check, LevelFail)
			if !metricsCheckMentions(check, tc.want) {
				t.Errorf("%s %v does not mention %q", check.Detail, check.Hints, tc.want)
			}
		})
	}
}

func TestMetricsCheckReadsTheMonitoringDeploy(t *testing.T) {
	root := t.TempDir()
	rulesDir := filepath.Join(root, "deploy", "monitoring", "prometheus", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(rulesDir, "mgke.yml")

	writeRules := func(body string) {
		t.Helper()
		if err := os.WriteFile(rules, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func() Check {
		return runMetricsCheck(metricsConfig(true, probeMetricsToken), metricsProbeOptions{root: func() string { return root }})
	}

	writeRules("expr: mgke_parser_lag_seconds > 900\n")
	if got := check(); got.Level != LevelOK {
		t.Fatalf("a rule that uses a served family must pass: %s %v", got.Detail, got.Hints)
	}

	writeRules("expr: mgke_archive_rows > 0\n")
	if got := check(); got.Level != LevelOK {
		t.Fatalf("a source-dependent family is allowed to be absent: %s %v", got.Detail, got.Hints)
	}

	writeRules("expr: mgke_missing_family_total > 0\n")
	broken := check()
	requireMetricsLevel(t, broken, LevelFail)
	if !containsHint(broken.Hints, "mgke_missing_family_total") {
		t.Errorf("the failure must name the unknown family: %v", broken.Hints)
	}
}

func TestMetricsCheckFailsWhenAConfiguredSourceStopsRendering(t *testing.T) {
	dir := t.TempDir()
	cfg := metricsConfig(true, probeMetricsToken)
	cfg.DBPath = filepath.Join(dir, "sqlite3.db")
	if err := os.WriteFile(cfg.DBPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Telegram.Noticer = true

	for _, dropped := range []string{
		"mgke_reminders_tracked",
		"mgke_scheduler_entries",
		"mgke_archive_rows",
		"mgke_archive_first_day_index",
		"mgke_webhook_pending_updates",
	} {
		check := brokenMetricsExpositionWith(t, cfg, dropLines(dropped))
		requireMetricsLevel(t, check, LevelFail)
		if !containsHint(check.Hints, dropped) {
			t.Errorf("dropping %s must be reported by name: %v", dropped, check.Hints)
		}
	}
}

func TestMetricsCheckExcusesAFamilyOnlyWhileItsSourceIsAbsent(t *testing.T) {
	cfg := metricsConfig(true, probeMetricsToken)

	for _, dropped := range []string{"mgke_reminders_tracked", "mgke_scheduler_entries", "mgke_archive_rows"} {
		check := brokenMetricsExpositionWith(t, cfg, dropLines(dropped))
		requireMetricsLevel(t, check, LevelOK)
	}
}

func TestRunProbesMetricsWithoutTheSite(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, fmt.Sprintf(`db_path: %q
chat_db_path: %q
cache_dir: %q
metrics:
  enabled: true
  token: %q
`,
		filepath.Join(dir, "sqlite3.db"),
		filepath.Join(dir, "bot_chats.db"),
		filepath.Join(dir, "cache"),
		probeMetricsToken,
	))

	report := Run(Options{ConfigPath: path, SkipSite: true, SkipImage: true})
	check := findCheck(t, report, "metrics")

	requireMetricsLevel(t, check, LevelOK)
	if containsHint(check.Hints, probeMetricsToken) {
		t.Errorf("the report must never carry the token: %v", check.Hints)
	}
}

func TestRunSkipsMetricsWhileDisabled(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, fmt.Sprintf(`db_path: %q
chat_db_path: %q
cache_dir: %q
`, filepath.Join(dir, "sqlite3.db"), filepath.Join(dir, "bot_chats.db"), filepath.Join(dir, "cache")))

	report := Run(Options{ConfigPath: path, SkipSite: true, SkipImage: true})
	requireMetricsLevel(t, findCheck(t, report, "metrics"), LevelSkip)
}
