package tgstub

import (
	"strings"
	"testing"
)

func TestLoopbackBaseURLKeepsTheStubLocal(t *testing.T) {
	cases := []struct {
		listen string
		want   string
	}{
		{"127.0.0.1:18090", "http://127.0.0.1:18090"},
		{"localhost:18090", "http://localhost:18090"},
		{":18090", "http://127.0.0.1:18090"},
		{"[::1]:18090", "http://[::1]:18090"},
	}

	for _, c := range cases {
		got, err := LoopbackBaseURL(c.listen)
		if err != nil {
			t.Errorf("LoopbackBaseURL(%q): %v", c.listen, err)
			continue
		}
		if got != c.want {
			t.Errorf("LoopbackBaseURL(%q) = %q, want %q", c.listen, got, c.want)
		}
	}
}

func TestDevEnvironmentOverridesTheRealTransport(t *testing.T) {
	base, err := LoopbackBaseURL("127.0.0.1:18090")
	if err != nil {
		t.Fatalf("LoopbackBaseURL: %v", err)
	}

	env := DevEnvironment(base)
	if env["MGKE_TELEGRAM_API_BASE_URL"] != base {
		t.Errorf("api base = %q, want %q", env["MGKE_TELEGRAM_API_BASE_URL"], base)
	}
	if env["MGKE_TELEGRAM_TOKEN"] != DevToken || DevToken == "" {
		t.Errorf("token = %q, want the stub token", env["MGKE_TELEGRAM_TOKEN"])
	}
	if env["MGKE_TELEGRAM_WEBHOOK_ENABLED"] != "false" {
		t.Errorf("webhook = %q, want it disabled", env["MGKE_TELEGRAM_WEBHOOK_ENABLED"])
	}
	for name, value := range env {
		if strings.Contains(value, "api.telegram.org") {
			t.Errorf("%s = %q points at the real Bot API", name, value)
		}
	}
}

func TestLoopbackBaseURLRefusesPublicAddresses(t *testing.T) {
	for _, listen := range []string{
		"api.telegram.org:443",
		"0.0.0.0:18090",
		"192.168.0.5:18090",
		"10.0.0.1:18090",
		"[2001:db8::1]:18090",
		"127.0.0.1",
		"127.0.0.1:0",
		"127.0.0.1:99999",
		"127.0.0.1:http",
		"",
	} {
		if got, err := LoopbackBaseURL(listen); err == nil {
			t.Errorf("LoopbackBaseURL(%q) = %q, want an error", listen, got)
		}
	}
}
