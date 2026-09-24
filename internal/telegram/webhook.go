package telegram

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/mymmrac/telego"
)

const (
	webhookModeLongPolling = "long_polling"
	webhookModeWebhook     = "webhook"

	webhookDefaultListen         = "127.0.0.1:8082"
	webhookDefaultPath           = "/telegram/webhook"
	webhookDefaultMaxConnections = 40
	webhookDefaultBuffer         = 128
	webhookMaxBodyBytes          = 1 << 20
	webhookReadHeaderTimeout     = 10 * time.Second
	webhookReadTimeout           = 30 * time.Second
	webhookWriteTimeout          = 30 * time.Second
	webhookIdleTimeout           = 60 * time.Second
	webhookShutdownTimeout       = 5 * time.Second
)

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

type webhookSettings struct {
	Listen             string
	Path               string
	Endpoint           string
	SecretToken        string
	Certificate        string
	Key                string
	IPAddress          string
	MaxConnections     int
	Buffer             int
	DropPendingUpdates bool
	AllowedUpdates     []string
}

func ValidateWebhook(cfg *config.Config) error {
	_, err := webhookSettingsFrom(cfg)
	return err
}

func webhookSettingsFrom(cfg *config.Config) (webhookSettings, error) {
	raw := cfg.Telegram.Webhook

	settings := webhookSettings{
		Listen:             strings.TrimSpace(raw.Listen),
		Path:               strings.TrimSpace(raw.Path),
		SecretToken:        strings.TrimSpace(raw.SecretToken),
		Certificate:        strings.TrimSpace(raw.Certificate),
		Key:                strings.TrimSpace(raw.Key),
		IPAddress:          strings.TrimSpace(raw.IPAddress),
		MaxConnections:     raw.MaxConnections,
		Buffer:             raw.Buffer,
		DropPendingUpdates: raw.DropPendingUpdates,
		AllowedUpdates:     raw.AllowedUpdates,
	}

	if settings.Listen == "" {
		settings.Listen = webhookDefaultListen
	}
	if settings.Path == "" {
		settings.Path = webhookDefaultPath
	}
	if settings.Buffer <= 0 {
		settings.Buffer = webhookDefaultBuffer
	}
	if settings.MaxConnections <= 0 {
		settings.MaxConnections = webhookDefaultMaxConnections
	}
	if settings.MaxConnections > 100 {
		return webhookSettings{}, fmt.Errorf("telegram.webhook.max_connections must be between 1 and 100, got %d", settings.MaxConnections)
	}
	if !strings.HasPrefix(settings.Path, "/") {
		return webhookSettings{}, fmt.Errorf("telegram.webhook.path must start with /, got %q", settings.Path)
	}
	if settings.SecretToken != "" && !webhookSecretPattern.MatchString(settings.SecretToken) {
		return webhookSettings{}, errors.New("telegram.webhook.secret_token accepts only A-Z, a-z, 0-9, _ and -, up to 256 characters")
	}
	if (settings.Certificate == "") != (settings.Key == "") {
		return webhookSettings{}, errors.New("telegram.webhook.certificate and telegram.webhook.key must be set together")
	}
	for _, path := range []string{settings.Certificate, settings.Key} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return webhookSettings{}, fmt.Errorf("telegram.webhook file %s: %w", path, err)
		}
	}
	if settings.IPAddress != "" && net.ParseIP(settings.IPAddress) == nil {
		return webhookSettings{}, fmt.Errorf("telegram.webhook.ip_address is not a valid IP address: %q", settings.IPAddress)
	}

	endpoint, err := webhookEndpoint(raw.URL, settings.Path)
	if err != nil {
		return webhookSettings{}, err
	}
	settings.Endpoint = endpoint
	if parsed, err := url.Parse(endpoint); err == nil {
		settings.Path = parsed.Path
	}

	return settings, nil
}

func webhookEndpoint(raw, path string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("telegram.webhook.url is required when the webhook is enabled")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("telegram.webhook.url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("telegram.webhook.url must be an https URL, got %q", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("telegram.webhook.url has no host: %q", raw)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = path
	}

	return parsed.String(), nil
}

func (s webhookSettings) setParams() (*telego.SetWebhookParams, func() error, error) {
	params := &telego.SetWebhookParams{
		URL:                s.Endpoint,
		IPAddress:          s.IPAddress,
		MaxConnections:     s.MaxConnections,
		AllowedUpdates:     s.AllowedUpdates,
		DropPendingUpdates: s.DropPendingUpdates,
		SecretToken:        s.SecretToken,
	}
	if s.Certificate == "" {
		return params, func() error { return nil }, nil
	}

	file, err := os.Open(s.Certificate)
	if err != nil {
		return nil, nil, fmt.Errorf("open telegram.webhook.certificate: %w", err)
	}
	params.Certificate = &telego.InputFile{File: file}

	return params, file.Close, nil
}

type WebhookStatus struct {
	Enabled           bool     `json:"enabled"`
	Mode              string   `json:"mode"`
	Endpoint          string   `json:"endpoint,omitempty"`
	Listen            string   `json:"listen,omitempty"`
	Path              string   `json:"path,omitempty"`
	SecretToken       bool     `json:"secretToken"`
	Certificate       bool     `json:"certificate"`
	MaxConnections    int      `json:"maxConnections,omitempty"`
	AllowedUpdates    []string `json:"allowedUpdates,omitempty"`
	URL               string   `json:"url,omitempty"`
	PendingUpdates    int      `json:"pendingUpdates"`
	CustomCertificate bool     `json:"customCertificate,omitempty"`
	LastError         string   `json:"lastError,omitempty"`
	LastErrorAt       int64    `json:"lastErrorAt,omitempty"`
	InfoError         string   `json:"infoError,omitempty"`
	CheckedAt         int64    `json:"checkedAt,omitempty"`
}

const webhookStatusTimeout = 3 * time.Second

func (b *Bot) WebhookStatus(ctx context.Context) WebhookStatus {
	status := WebhookStatus{
		Enabled: b.cfg.Telegram.Webhook.Enabled,
		Mode:    webhookModeLongPolling,
	}

	if status.Enabled {
		status.Mode = webhookModeWebhook
		if settings, err := webhookSettingsFrom(b.cfg); err == nil {
			status.Endpoint = settings.Endpoint
			status.Listen = settings.Listen
			status.Path = settings.Path
			status.MaxConnections = settings.MaxConnections
			status.AllowedUpdates = settings.AllowedUpdates
			status.SecretToken = settings.SecretToken != ""
			status.Certificate = settings.Certificate != ""
		} else {
			status.InfoError = err.Error()
		}
	}

	cached := b.cachedWebhookStatus()

	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), webhookStatusTimeout)
	defer cancel()

	info, err := b.client.GetWebhookInfo(checkCtx)
	if err != nil {
		status.URL = cached.URL
		status.PendingUpdates = cached.PendingUpdates
		status.CustomCertificate = cached.CustomCertificate
		status.LastError = cached.LastError
		status.LastErrorAt = cached.LastErrorAt
		status.CheckedAt = cached.CheckedAt
		if status.InfoError == "" {
			status.InfoError = err.Error()
		}
		return status
	}

	status.URL = info.URL
	status.PendingUpdates = info.PendingUpdateCount
	status.CustomCertificate = info.HasCustomCertificate
	status.LastError = info.LastErrorMessage
	status.LastErrorAt = info.LastErrorDate
	if len(info.AllowedUpdates) > 0 {
		status.AllowedUpdates = info.AllowedUpdates
	}
	if info.MaxConnections > 0 {
		status.MaxConnections = info.MaxConnections
	}
	status.CheckedAt = time.Now().Unix()

	b.webhookMu.Lock()
	b.webhookStatus = status
	b.webhookMu.Unlock()

	return status
}

func (b *Bot) cachedWebhookStatus() WebhookStatus {
	b.webhookMu.Lock()
	defer b.webhookMu.Unlock()

	return b.webhookStatus
}

func (b *Bot) ResetWebhook(ctx context.Context) (WebhookStatus, error) {
	if !b.cfg.Telegram.Webhook.Enabled {
		return WebhookStatus{}, errors.New("telegram.webhook.enabled is off")
	}

	settings, err := webhookSettingsFrom(b.cfg)
	if err != nil {
		return WebhookStatus{}, err
	}
	params, closeCertificate, err := settings.setParams()
	if err != nil {
		return WebhookStatus{}, err
	}
	defer func() { _ = closeCertificate() }()

	if err := b.client.SetWebhook(ctx, params); err != nil {
		return WebhookStatus{}, err
	}

	return b.WebhookStatus(ctx), nil
}

func webhookHTTPHandler(handler telego.WebhookHandler, secretToken string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secretToken != "" && subtle.ConstantTimeCompare(
			[]byte(request.Header.Get(telego.WebhookSecretTokenHeader)), []byte(secretToken),
		) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, webhookMaxBodyBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(writer, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(writer, "cannot read body", http.StatusBadRequest)
			return
		}
		if err := handler(context.WithoutCancel(request.Context()), body); err != nil {
			http.Error(writer, "cannot accept update", http.StatusInternalServerError)
			return
		}

		writer.WriteHeader(http.StatusOK)
	}
}

func (b *Bot) runPolling(ctx context.Context) error {
	if err := b.client.DeleteWebhook(ctx, &telego.DeleteWebhookParams{}); err != nil {
		b.log.Warn().Err(err).Msg("failed to delete the webhook before long polling")
	}

	updates, err := b.client.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		Timeout: 30,
	})
	if err != nil {
		return fmt.Errorf("start polling: %w", err)
	}

	b.log.Info().Msg("bot started, listening for updates via long polling")

	return b.consumeUpdates(ctx, updates)
}

func (b *Bot) runWebhook(ctx context.Context) error {
	settings, err := webhookSettingsFrom(b.cfg)
	if err != nil {
		return fmt.Errorf("telegram webhook: %w", err)
	}

	params, closeCertificate, err := settings.setParams()
	if err != nil {
		return fmt.Errorf("telegram webhook: %w", err)
	}
	defer func() { _ = closeCertificate() }()

	listener, err := net.Listen("tcp", settings.Listen)
	if err != nil {
		return fmt.Errorf("listen telegram webhook on %s: %w", settings.Listen, err)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: webhookReadHeaderTimeout,
		ReadTimeout:       webhookReadTimeout,
		WriteTimeout:      webhookWriteTimeout,
		IdleTimeout:       webhookIdleTimeout,
	}

	register := func(handler telego.WebhookHandler) error {
		mux.HandleFunc(settings.Path, webhookHTTPHandler(handler, settings.SecretToken))
		return nil
	}

	updates, err := b.client.UpdatesViaWebhook(ctx, register,
		telego.WithWebhookBuffer(uint(settings.Buffer)),
		telego.WithWebhookSet(ctx, params),
	)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("start webhook: %w", err)
	}

	b.logWebhookInfo(ctx)

	serveErr := make(chan error, 1)
	go func() {
		var err error
		if settings.Certificate != "" {
			err = server.ServeTLS(listener, settings.Certificate, settings.Key)
		} else {
			err = server.Serve(listener)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), webhookShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			b.log.Warn().Err(err).Msg("failed to shut the webhook server down")
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("telegram webhook server: %w", err)
	default:
	}

	return b.consumeUpdates(ctx, updates)
}

const webhookResetCallback = "webhook:reset"

type webhookCmd struct{ bot *Bot }

func (c *webhookCmd) Name() string { return "/webhook" }

func (c *webhookCmd) AdminOnly() bool { return true }
func (c *webhookCmd) Description() string {
	return c.bot.loc("cmd_webhook")
}
func (c *webhookCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}

	status := c.bot.WebhookStatus(ctx)

	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.webhookStatusText(status), c.bot.webhookStatusKeyboard())
}

func (b *Bot) webhookStatusKeyboard() *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: b.loc("webhook_reset_button"), CallbackData: webhookResetCallback}},
		},
	}
}

func (b *Bot) webhookStatusText(status WebhookStatus) string {
	lines := []string{b.loc("webhook_header")}

	if status.Enabled {
		lines = append(lines, b.loc("webhook_mode_webhook"))
		lines = append(lines, b.locData("webhook_endpoint", map[string]interface{}{"Endpoint": status.Endpoint}))
		lines = append(lines, b.locData("webhook_listen", map[string]interface{}{"Listen": status.Listen}))
	} else {
		lines = append(lines, b.loc("webhook_mode_long_polling"))
	}

	if status.SecretToken {
		lines = append(lines, b.loc("webhook_secret_set"))
	} else if status.Enabled {
		lines = append(lines, b.loc("webhook_secret_missing"))
	}
	if status.Certificate {
		lines = append(lines, b.loc("webhook_certificate_set"))
	}

	if status.InfoError != "" {
		lines = append(lines, b.locData("webhook_info_unavailable", map[string]interface{}{"Error": status.InfoError}))
	} else {
		lines = append(lines, b.locData("webhook_pending", map[string]interface{}{"Count": status.PendingUpdates}))
		lines = append(lines, b.locData("webhook_delivered_to", map[string]interface{}{"URL": status.URL}))
	}

	if status.LastError != "" {
		lines = append(lines, b.locData("webhook_last_error", map[string]interface{}{
			"Error": status.LastError,
			"At":    webhookMoment(status.LastErrorAt),
		}))
	} else if status.InfoError == "" {
		lines = append(lines, b.loc("webhook_no_error"))
	}

	if status.MaxConnections > 0 {
		lines = append(lines, b.locData("webhook_connections", map[string]interface{}{"Count": status.MaxConnections}))
	}
	if len(status.AllowedUpdates) > 0 {
		lines = append(lines, b.locData("webhook_updates", map[string]interface{}{"List": strings.Join(status.AllowedUpdates, ", ")}))
	}

	if !status.Enabled {
		lines = append(lines, "", b.loc("webhook_disabled_hint"))
	}

	return strings.Join(lines, "\n")
}

func webhookMoment(at int64) string {
	if at <= 0 {
		return ""
	}
	return time.Unix(at, 0).Format("02.01 15:04")
}

type webhookResetCb struct{ bot *Bot }

func (cb *webhookResetCb) Prefix() string { return "webhook" }

func (cb *webhookResetCb) Handler(ctx context.Context, u *Update) error {
	if u.Callback != nil {
		cb.bot.AnswerCallback(u.Callback.ID, "")
	}
	if !cb.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if u.Data != webhookResetCallback {
		return nil
	}

	if _, err := cb.bot.ResetWebhook(ctx); err != nil {
		cb.bot.log.Error().Err(err).Msg("manual webhook reset failed")
		return u.Bot.SendText(u.ChatID, cb.bot.locData("webhook_reset_failed", map[string]interface{}{"Error": err.Error()}))
	}

	status := cb.bot.WebhookStatus(ctx)
	return u.Bot.SendText(u.ChatID, cb.bot.locData("webhook_reset_done", map[string]interface{}{"Endpoint": status.Endpoint}))
}

func (b *Bot) logWebhookInfo(ctx context.Context) {
	status := b.WebhookStatus(ctx)

	logger := b.log.Info().
		Str("endpoint", status.Endpoint).
		Str("listen", status.Listen).
		Int("max_connections", status.MaxConnections).
		Bool("custom_certificate", status.Certificate).
		Bool("secret_token", status.SecretToken)

	if status.InfoError != "" {
		logger.Str("info_error", status.InfoError).Msg("bot started, listening for updates via webhook, webhook info unavailable")
		return
	}

	logger.
		Int("pending_updates", status.PendingUpdates).
		Str("webhook_url", status.URL).
		Strs("allowed_updates", status.AllowedUpdates).
		Str("last_error", status.LastError).
		Msg("bot started, listening for updates via webhook")
}
