package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/mymmrac/telego"
)

type Bot struct {
	client    *telego.Bot
	cfg       *config.Config
	log       *logger.Logger
	i18n      *i18n.Localizer
	chatRepo  *Repository
	cache     *cache.RaspCache
	commands  map[string]Command
	callbacks map[string]Callback
}

type Update struct {
	Bot      *Bot
	Message  *telego.Message
	Callback *telego.CallbackQuery
	ChatID   int64
	UserID   int64
	Text     string
	Data     string
}

type Command interface {
	Name() string
	Description() string
	Handler(ctx context.Context, u *Update) error
}

type TextMatcher interface {
	MatchText(string) bool
}

type Callback interface {
	Prefix() string
	Handler(ctx context.Context, u *Update) error
}

func NewBot(cfg *config.Config, log *logger.Logger, loc *i18n.Localizer, chatRepo *Repository, cache *cache.RaspCache) (*Bot, error) {
	client, err := telego.NewBot(cfg.Telegram.Token, telego.WithDefaultDebugLogger())
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	b := &Bot{
		client:    client,
		cfg:       cfg,
		log:       log,
		i18n:      loc,
		chatRepo:  chatRepo,
		cache:     cache,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
	}

	b.registerAll()
	return b, nil
}

func (b *Bot) Client() *telego.Bot       { return b.client }
func (b *Bot) Config() *config.Config    { return b.cfg }
func (b *Bot) I18n() *i18n.Localizer     { return b.i18n }
func (b *Bot) Log() *logger.Logger       { return b.log }
func (b *Bot) GetRaspCache() *cache.RaspCache { return b.cache }

func (b *Bot) RegisterCommand(cmd Command) {
	b.commands[cmd.Name()] = cmd
}

func (b *Bot) RegisterCallback(cb Callback) {
	b.callbacks[cb.Prefix()] = cb
}

func (b *Bot) registerAll() {
	b.RegisterCommand(&startCmd{bot: b})
	b.RegisterCommand(&helpCmd{bot: b})
	b.RegisterCommand(&cancelCmd{bot: b})
	b.RegisterCommand(&setupCmd{bot: b})
	b.RegisterCommand(&dayCmd{bot: b})
	b.RegisterCommand(&weekCmd{bot: b})
	b.RegisterCommand(&callsCmd{bot: b})
	b.RegisterCommand(&aboutCmd{bot: b})
	b.RegisterCommand(&groupCmd{bot: b})
	b.RegisterCommand(&teacherCmd{bot: b})
	b.RegisterCommand(&settingsCmd{bot: b})
	b.RegisterCommand(&imageCmd{bot: b})

	b.RegisterCallback(&timetableCb{bot: b})
	b.RegisterCallback(&callsCb{bot: b})
	b.RegisterCallback(&imageCb{bot: b})
	b.RegisterCallback(&cancelCb{bot: b})
	b.RegisterCallback(&setupCb{bot: b})
	b.RegisterCallback(&dayCb{bot: b})
	b.RegisterCallback(&weekCb{bot: b})
	b.RegisterCallback(&aboutCb{bot: b})
	b.RegisterCallback(&groupCb{bot: b})
	b.RegisterCallback(&teacherCb{bot: b})
	b.RegisterCallback(&settingsCb{bot: b})
	b.RegisterCallback(&icsCb{bot: b})
}

func (b *Bot) Run(ctx context.Context) error {
	updates, err := b.client.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		Timeout: 30,
	})
	if err != nil {
		return fmt.Errorf("start polling: %w", err)
	}

	b.log.Info().Msg("bot started, listening for updates")

	for update := range updates {
		b.handleUpdate(ctx, update)
	}

	return nil
}

func (b *Bot) handleUpdate(ctx context.Context, update telego.Update) {
	if update.Message != nil {
		b.handleMessage(ctx, update.Message)
	}
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update.CallbackQuery)
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *telego.Message) {
	text := msg.Text
	if text == "" {
		return
	}

	u := &Update{
		Bot:     b,
		Message: msg,
		ChatID:  msg.Chat.ID,
		UserID:  msg.From.ID,
		Text:    text,
	}

	b.handleMessageText(ctx, u)

	if cmd, ok := b.commands[text]; ok {
		if err := cmd.Handler(ctx, u); err != nil {
			b.log.Error().Err(err).Str("cmd", text).Msg("command error")
		}
		return
	}

	if len(text) > 1 && text[0] == '/' {
		cmdName := text[1:]
		if idx := strings.IndexByte(cmdName, ' '); idx >= 0 {
			cmdName = cmdName[:idx]
		}
		if cmd, ok := b.commands["/"+cmdName]; ok {
			if err := cmd.Handler(ctx, u); err != nil {
				b.log.Error().Err(err).Str("cmd", cmdName).Msg("command error")
			}
			return
		}
	}

	for _, cmd := range b.commands {
		if tm, ok := cmd.(TextMatcher); ok {
			if tm.MatchText(u.Text) {
				if err := cmd.Handler(ctx, u); err != nil {
					b.log.Error().Err(err).Msg("text match error")
				}
				return
			}
		}
	}
}

func (b *Bot) handleCallback(ctx context.Context, cb *telego.CallbackQuery) {
	u := &Update{
		Bot:      b,
		Callback: cb,
		UserID:   cb.From.ID,
		Data:     cb.Data,
	}
	if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
		u.ChatID = msg.Chat.ID
	}

	for prefix, handler := range b.callbacks {
		if strings.HasPrefix(cb.Data, prefix) {
			if err := handler.Handler(ctx, u); err != nil {
				b.log.Error().Err(err).Str("prefix", prefix).Msg("callback error")
			}
			return
		}
	}
}

func (b *Bot) SetMyCommands() error {
	var cmds []telego.BotCommand
	for _, cmd := range b.commands {
		cmds = append(cmds, telego.BotCommand{
			Command:     cmd.Name(),
			Description: cmd.Description(),
		})
	}
	return b.client.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
		Commands: cmds,
	})
}

func (b *Bot) SendText(chatID int64, text string) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   text,
	})
	return err
}

func (b *Bot) SendTextWithKeyboard(chatID int64, text string, kb *telego.InlineKeyboardMarkup) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ReplyMarkup: kb,
	})
	return err
}

func (b *Bot) SendPhoto(chatID int64, filePath string, caption string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	params := &telego.SendPhotoParams{
		ChatID: telego.ChatID{ID: chatID},
		Photo:  telego.InputFile{File: f},
	}
	if caption != "" {
		params.Caption = caption
	}
	_, err = b.client.SendPhoto(context.Background(), params)
	return err
}

func (b *Bot) AnswerCallback(callbackID string, text string) error {
	return b.client.AnswerCallbackQuery(context.Background(), &telego.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
}

func (b *Bot) EditMessageText(chatID int64, messageID int, text string, kb *telego.InlineKeyboardMarkup) error {
	params := &telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: messageID,
		Text:      text,
	}
	if kb != nil {
		params.ReplyMarkup = kb
	}
	_, err := b.client.EditMessageText(context.Background(), params)
	return err
}

func (b *Bot) CleanupTempFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func (b *Bot) formatCallsSchedule() string {
	timetable := b.cfg.Timetable
	var sb strings.Builder
	if len(timetable.Weekdays) > 0 {
		sb.WriteString(b.loc("calls_weekdays"))
		sb.WriteString("\n")
		for i, slot := range timetable.Weekdays {
			sb.WriteString(fmt.Sprintf("  %s-%s / %s-%s", slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
			if i < len(timetable.Weekdays)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if len(timetable.Saturday) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(b.loc("calls_saturday"))
		sb.WriteString("\n")
		for i, slot := range timetable.Saturday {
			sb.WriteString(fmt.Sprintf("  %s-%s / %s-%s", slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
			if i < len(timetable.Saturday)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if sb.Len() == 0 {
		return b.loc("calls_not_configured")
	}
	return sb.String()
}
