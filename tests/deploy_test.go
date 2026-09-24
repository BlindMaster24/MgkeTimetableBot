package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type deployService struct {
	Image       string            `yaml:"image"`
	Environment map[string]string `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	DependsOn   []string          `yaml:"depends_on"`
}

type deployCompose struct {
	Services map[string]deployService `yaml:"services"`
	Volumes  map[string]struct {
		Name string `yaml:"name"`
	} `yaml:"volumes"`
}

func readDeployCompose(t *testing.T, name string) deployCompose {
	t.Helper()

	var parsed deployCompose
	if err := yaml.Unmarshal([]byte(readFile(t, name)), &parsed); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}

func requireService(t *testing.T, compose deployCompose, name string) deployService {
	t.Helper()

	service, ok := compose.Services[name]
	if !ok {
		t.Fatalf("expected a %s service, got %v", name, compose.Services)
	}
	return service
}

func requireMount(t *testing.T, service deployService, fragment string) {
	t.Helper()

	for _, mount := range service.Volumes {
		if strings.Contains(mount, fragment) {
			return
		}
	}
	t.Errorf("expected a volume mount containing %q, got %v", fragment, service.Volumes)
}

func requirePort(t *testing.T, service deployService, port string) {
	t.Helper()

	for _, published := range service.Ports {
		if published == port {
			return
		}
	}
	t.Errorf("expected the published port %q, got %v", port, service.Ports)
}

func requireWebhookEnvironment(t *testing.T, env map[string]string) {
	t.Helper()

	if env["MGKE_TELEGRAM_WEBHOOK_ENABLED"] != "true" {
		t.Errorf("the bot service must enable the webhook, got %q", env["MGKE_TELEGRAM_WEBHOOK_ENABLED"])
	}
	if !strings.Contains(env["MGKE_TELEGRAM_WEBHOOK_URL"], "https://${BOT_DOMAIN") {
		t.Errorf("the webhook address must be built from BOT_DOMAIN over https, got %q", env["MGKE_TELEGRAM_WEBHOOK_URL"])
	}
	if env["MGKE_TELEGRAM_WEBHOOK_LISTEN"] != "0.0.0.0:8082" {
		t.Errorf("the reverse proxy can only reach the bot on a public listen address, got %q", env["MGKE_TELEGRAM_WEBHOOK_LISTEN"])
	}
	secret := env["MGKE_TELEGRAM_WEBHOOK_SECRET_TOKEN"]
	if !strings.Contains(secret, "MGKE_TELEGRAM_WEBHOOK_SECRET_TOKEN") || !strings.Contains(secret, ":?") {
		t.Errorf("the secret token must come from the environment and refuse to start when unset, got %q", secret)
	}
	for _, name := range []string{"MGKE_TELEGRAM_TOKEN", "MGKE_DB_PATH", "MGKE_CHAT_DB_PATH", "MGKE_CACHE_DIR"} {
		if env[name] == "" {
			t.Errorf("the example lost %s", name)
		}
	}
}

func TestCaddyWebhookComposeTerminatesTLSWithCaddy(t *testing.T) {
	compose := readDeployCompose(t, "docker-compose.webhook.yml")

	bot := requireService(t, compose, "bot")
	requireWebhookEnvironment(t, bot.Environment)
	requireMount(t, bot, "/data")

	caddy := requireService(t, compose, "caddy")
	if !strings.HasPrefix(caddy.Image, "caddy:") {
		t.Errorf("expected the official caddy image, got %q", caddy.Image)
	}
	if caddy.Environment["BOT_DOMAIN"] == "" || caddy.Environment["ACME_EMAIL"] == "" {
		t.Errorf("Caddy needs BOT_DOMAIN and ACME_EMAIL, got %+v", caddy.Environment)
	}
	requirePort(t, caddy, "80:80")
	requirePort(t, caddy, "443:443")
	requireMount(t, caddy, "deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro")

	for _, dependency := range caddy.DependsOn {
		if dependency == "bot" {
			return
		}
	}
	t.Errorf("Caddy must start after the bot, got %v", caddy.DependsOn)
}

func TestNginxWebhookComposeTerminatesTLSWithNginx(t *testing.T) {
	compose := readDeployCompose(t, "docker-compose.webhook-nginx.yml")

	bot := requireService(t, compose, "bot")
	requireWebhookEnvironment(t, bot.Environment)

	nginx := requireService(t, compose, "nginx")
	if !strings.HasPrefix(nginx.Image, "nginx:") {
		t.Errorf("expected the official nginx image, got %q", nginx.Image)
	}
	if nginx.Environment["BOT_DOMAIN"] == "" {
		t.Error("nginx renders its server_name from BOT_DOMAIN")
	}
	requirePort(t, nginx, "80:80")
	requirePort(t, nginx, "443:443")
	requireMount(t, nginx, "deploy/nginx:/etc/nginx/templates:ro")

	certbot := requireService(t, compose, "certbot")
	if !strings.HasPrefix(certbot.Image, "certbot/certbot") {
		t.Errorf("expected the official certbot image, got %q", certbot.Image)
	}
	requireMount(t, certbot, "/etc/letsencrypt")

	letsencrypt, ok := compose.Volumes["letsencrypt"]
	if !ok || letsencrypt.Name == "" {
		t.Fatalf("the example must pin the certificate volume by name, got %+v", compose.Volumes)
	}
	webroot, ok := compose.Volumes["certbot-webroot"]
	if !ok || webroot.Name == "" {
		t.Fatalf("the example must pin the challenge volume by name, got %+v", compose.Volumes)
	}

	requireMount(t, nginx, "letsencrypt:/etc/letsencrypt")
	requireMount(t, nginx, "certbot-webroot:/var/www/certbot")
	requireMount(t, certbot, "letsencrypt:/etc/letsencrypt")
	requireMount(t, certbot, "certbot-webroot:/var/www/certbot")

	composeText := readFile(t, "docker-compose.webhook-nginx.yml")
	for _, token := range []string{"certbot renew", "--webroot --webroot-path /var/www/certbot"} {
		if !strings.Contains(composeText, token) {
			t.Errorf("the certbot service must renew the certificate automatically, missing %q", token)
		}
	}

	for _, doc := range docs {
		content := readDoc(t, doc)
		for _, mount := range []string{
			letsencrypt.Name + ":/etc/letsencrypt",
			webroot.Name + ":/var/www/certbot",
			"--webroot -w /var/www/certbot",
		} {
			if !strings.Contains(content, mount) {
				t.Errorf("%s must bootstrap the certificate with %q, otherwise certbot never finds it", doc, mount)
			}
		}
	}
}

func TestReverseProxyExamplesOnlyExposeTheWebhookAndTheAPIStaysLocal(t *testing.T) {
	caddy := readFile(t, filepath.Join("deploy", "caddy", "Caddyfile"))
	nginx := readFile(t, filepath.Join("deploy", "nginx", "default.conf.template"))

	if !strings.Contains(caddy, "respond 404") {
		t.Error("the Caddy site must answer 404 outside the webhook path instead of proxying everything to the bot")
	}
	if !strings.Contains(nginx, "return 404") {
		t.Error("the nginx site must answer 404 outside the webhook path instead of proxying everything to the bot")
	}

	if !strings.Contains(nginx, "client_max_body_size 1m") {
		t.Error("nginx must accept the same 1 MB body the bot accepts, otherwise Telegram retries a body the bot would have taken")
	}
	if !strings.Contains(nginx, "X-Telegram-Bot-Api-Secret-Token") {
		t.Error("nginx must forward the Telegram secret header the bot compares")
	}

	for _, name := range []string{"docker-compose.webhook.yml", "docker-compose.webhook-nginx.yml"} {
		for _, port := range requireService(t, readDeployCompose(t, name), "bot").Ports {
			if strings.Contains(port, "8081") && !strings.HasPrefix(port, "127.0.0.1:") {
				t.Errorf("%s publishes the API port %q on every interface, want loopback only", name, port)
			}
		}
	}
}
