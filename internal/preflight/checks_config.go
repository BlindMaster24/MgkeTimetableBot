package preflight

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

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

func checkWebhook(cfg *config.Config) Check {
	if !cfg.Telegram.Webhook.Enabled {
		return Check{Name: "webhook", Level: LevelSkip, Detail: "long polling is used, no public address required"}
	}

	if err := telegram.ValidateWebhook(cfg); err != nil {
		return Check{
			Name:   "webhook",
			Level:  LevelFail,
			Detail: err.Error(),
			Hints: []string{
				"warning: set telegram.webhook.url to the public https address of this bot",
				"warning: the secret token is sent in the X-Telegram-Bot-Api-Secret-Token header, use A-Z a-z 0-9 _ - only",
			},
		}
	}

	detail := fmt.Sprintf("webhook enabled, listen %s", cfg.Telegram.Webhook.Listen)
	if strings.TrimSpace(cfg.Telegram.Webhook.SecretToken) == "" {
		return Check{
			Name:   "webhook",
			Level:  LevelWarn,
			Detail: detail,
			Hints:  []string{"warning: without telegram.webhook.secret_token anyone who knows the address can post updates"},
		}
	}

	return Check{Name: "webhook", Level: LevelOK, Detail: detail}
}

func checkTelegramAPI(cfg *config.Config) Check {
	server, err := cfg.TelegramAPIServer()
	if err != nil {
		return Check{
			Name:   "telegram-api",
			Level:  LevelFail,
			Detail: err.Error(),
			Hints:  []string{"warning: leave telegram.api_base_url empty to talk to https://api.telegram.org"},
		}
	}

	if server == "" {
		return Check{Name: "telegram-api", Level: LevelOK, Detail: "official https://api.telegram.org"}
	}

	return Check{
		Name:   "telegram-api",
		Level:  LevelWarn,
		Detail: fmt.Sprintf("telegram.api_base_url is %s", server),
		Hints:  []string{"warning: the bot talks to this Bot API server instead of https://api.telegram.org"},
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
