package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	cacheMu   sync.Mutex
	commands  map[string]Command
	callbacks map[string]Callback
	parseFunc func() error
	startTime time.Time
	archive   any
	aliasRepo *AliasRepository
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

func NewBot(cfg *config.Config, log *logger.Logger, loc *i18n.Localizer, chatRepo *Repository, cache *cache.RaspCache, archive any) (*Bot, error) {
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
		archive:   archive,
		commands:  make(map[string]Command),
		callbacks: make(map[string]Callback),
		startTime: time.Now(),
	}

	b.aliasRepo = NewAliasRepository(chatRepo)
	b.aliasRepo.EnsureTable()
	b.registerAll()
	return b, nil
}

func (b *Bot) Client() *telego.Bot       { return b.client }
func (b *Bot) Config() *config.Config    { return b.cfg }
func (b *Bot) I18n() *i18n.Localizer     { return b.i18n }
func (b *Bot) Log() *logger.Logger       { return b.log }
func (b *Bot) GetRaspCache() *cache.RaspCache { return b.cache }
func (b *Bot) SetParseFunc(fn func() error) { b.parseFunc = fn }

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
	b.RegisterCommand(&getGroupWeekCmd{bot: b})
	b.RegisterCommand(&getGroupImageCmd{bot: b})
	b.RegisterCommand(&getTeacherWeekCmd{bot: b})
	b.RegisterCommand(&getTeacherImageCmd{bot: b})
	b.RegisterCommand(&setGroupCmd{bot: b})
	b.RegisterCommand(&setTeacherCmd{bot: b})
	b.RegisterCommand(&settingsCmd{bot: b})
	b.RegisterCommand(&imageCmd{bot: b})
	b.RegisterCommand(&buttonsCmd{bot: b})
	b.RegisterCommand(&formatterCmd{bot: b})
	b.RegisterCommand(&forceParseCmd{bot: b})
	b.RegisterCommand(&resetCacheCmd{bot: b})
	b.RegisterCommand(&eulaCmd{bot: b})
	b.RegisterCommand(&apiCmd{bot: b})
	b.RegisterCommand(&diffCmd{bot: b})
	b.RegisterCommand(&noticeCmd{bot: b})
	b.RegisterCommand(&viewCmd{bot: b})
	b.RegisterCommand(&devCmd{bot: b})
	b.RegisterCommand(&mathCmd{bot: b})
	b.RegisterCommand(&flushCacheCmd{bot: b})
	b.RegisterCommand(&debugCmd{bot: b})
	b.RegisterCommand(&sendCmd{bot: b})
	b.RegisterCommand(&triggerCmd{bot: b})
	b.RegisterCommand(&historyCmd{bot: b})
	b.RegisterCommand(&aliasCmd{bot: b})
	b.RegisterCommand(&statsCmd{bot: b})
	b.RegisterCommand(&googleCalendarCmd{bot: b})
	b.RegisterCommand(&regexpCmd{bot: b})
	b.RegisterCommand(&vanishCmd{bot: b})
	b.RegisterCommand(&parserLogsCmd{bot: b})
	b.RegisterCommand(&requireNewButtonsCmd{bot: b})
	b.RegisterCommand(&createApiKeyCmd{bot: b})
	b.RegisterCommand(&decryptKeyCmd{bot: b})
	b.RegisterCommand(&getCabinetCmd{bot: b})
	b.RegisterCommand(&getGroupsCmd{bot: b})
	b.RegisterCommand(&getTeachersCmd{bot: b})
	b.RegisterCommand(&compareGroupsCmd{bot: b})
	b.RegisterCommand(&pingCmd{bot: b})
	b.RegisterCommand(&icsCmd{bot: b})
	b.RegisterCommand(&subscriptionsTestCmd{bot: b})
	b.RegisterCommand(&archiveCmd{bot: b})
	b.RegisterCommand(&endingsCmd{bot: b})
	b.RegisterCommand(&chatCmd{bot: b})
	b.RegisterCommand(&idCmd{bot: b})
	b.RegisterCommand(&errorCmd{bot: b})
	b.RegisterCommand(&testCmd{bot: b})

	b.RegisterCallback(&dayCb{bot: b})
	b.RegisterCallback(&weekCb{bot: b})
	b.RegisterCallback(&callsCb{bot: b})
	b.RegisterCallback(&callsFullCb{bot: b})
	b.RegisterCallback(&imageCb{bot: b})
	b.RegisterCallback(&imageGroupCb{bot: b})
	b.RegisterCallback(&imageTeacherCb{bot: b})
	b.RegisterCallback(&cancelCb{bot: b})
	b.RegisterCallback(&setupCb{bot: b})
	b.RegisterCallback(&aboutCb{bot: b})
	b.RegisterCallback(&groupCb{bot: b})
	b.RegisterCallback(&teacherCb{bot: b})
	b.RegisterCallback(&settingsCb{bot: b})
	b.RegisterCallback(&icsCb{bot: b})
	b.RegisterCallback(&btnToggleCb{bot: b})
	b.RegisterCallback(&btnMenuCb{bot: b})
	b.RegisterCallback(&fmtMenuCb{bot: b})
	b.RegisterCallback(&fmtSelectCb{bot: b})
	b.RegisterCallback(&noticeMenuCb{bot: b})
	b.RegisterCallback(&viewMenuCb{bot: b})
	b.RegisterCallback(&noticeToggleCb{bot: b})
	b.RegisterCallback(&viewToggleCb{bot: b})
	b.RegisterCallback(&mainMenuCb{bot: b})
	b.RegisterCallback(&diffMenuCb{bot: b})
	b.RegisterCallback(&diffAdvancedCb{bot: b})
	b.RegisterCallback(&diffToggleCb{bot: b})
	b.RegisterCallback(&callsMenuCb{bot: b})
	b.RegisterCallback(&callsShowCb{bot: b})
	b.RegisterCallback(&callsRefreshCb{bot: b})
	b.RegisterCallback(&callsSourceCb{bot: b})
	b.RegisterCallback(&callsSourceResetCb{bot: b})
	b.RegisterCallback(&schedulesMenuCb{bot: b})
	b.RegisterCallback(&currentSettingsCb{bot: b})
	b.RegisterCallback(&subsMenuCb{bot: b})
	b.RegisterCallback(&subsAddGroupCb{bot: b})
	b.RegisterCallback(&subsAddTeacherCb{bot: b})
	b.RegisterCallback(&subsListCb{bot: b})
	b.RegisterCallback(&subsRemoveCb{bot: b})
	b.RegisterCallback(&subsCheckCb{bot: b})
	b.RegisterCallback(&subsCheckFullCb{bot: b})
	b.RegisterCallback(&answerCb{bot: b})
	b.RegisterCallback(&timetableGroupCb{bot: b})
	b.RegisterCallback(&timetableTeacherCb{bot: b})
	b.RegisterCallback(&historyCb{bot: b})
	b.RegisterCallback(&googleCalCb{bot: b})
	b.RegisterCallback(&callsEditCb{bot: b})
	b.RegisterCallback(&aliasCb{bot: b})
	b.RegisterCallback(&aliasDelCb{bot: b})
	b.RegisterCallback(&aliasMenuCb{bot: b})
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
		ChatID:    telego.ChatID{ID: chatID},
		Text:      text,
		ParseMode: "HTML",
	})
	return err
}

func (b *Bot) SendTextWithKeyboard(chatID int64, text string, kb *telego.InlineKeyboardMarkup) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ParseMode:   "HTML",
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
		params.ParseMode = "HTML"
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

func withChat(b *Bot, u *Update, fn func(*Chat) error) error {
	chat, err := b.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("data_not_loaded"))
	}
	return fn(chat)
}

func (b *Bot) EditMessageText(chatID int64, messageID int, text string, kb *telego.InlineKeyboardMarkup) error {
	params := &telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
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
