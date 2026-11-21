package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
)

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func writeRunConfig(t *testing.T, apiBase, site string, port int, dir string) string {
	t.Helper()
	body := fmt.Sprintf(`telegram:
  token: "123456:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
  api_base_url: %q
  noticer: false
http:
  port: %d
timezone: "UTC"
cache_dir: %q
db_path: %q
chat_db_path: %q
logging:
  level: "error"
  format: "json"
parser:
  enabled: true
  update_interval:
    default: 300
  endpoints:
    timetable_group: %q
    timetable_teacher: %q
    bell_schedule: %q
    team:
      - %q
`, apiBase, port, filepath.Join(dir, "cache", "rasp"),
		filepath.Join(dir, "archive.db"), filepath.Join(dir, "chats.db"),
		site+"/groups", site+"/teachers", site+"/calls", site+"/team")

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunStartsAndStopsTheWholeProcess walks the production entry point: every
// store opens, the API listens, the bot polls the stub and the signal context
// shuts it down with exit code 0.
func TestRunStartsAndStopsTheWholeProcess(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer api.Close()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body><table></table></body></html>"))
	}))
	defer site.Close()

	dir := t.TempDir()
	cfgPath := writeRunConfig(t, api.URL, site.URL, freePort(t), dir)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(1500 * time.Millisecond)
		cancel()
	}()

	if code := Run(ctx, cfgPath, build.Info{Version: "test"}); code != 0 {
		t.Fatalf("Run returned %d, want 0 after a clean shutdown", code)
	}
	for _, name := range []string{"archive.db", "chats.db"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s must exist after Run: %v", name, err)
		}
	}
}

func TestRunReportsAMissingConfig(t *testing.T) {
	if code := Run(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"), build.Info{}); code != 1 {
		t.Fatalf("Run returned %d, want 1 for a missing config", code)
	}
}
