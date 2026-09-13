package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	"github.com/blindmaster24/MgkeTimetableBot/internal/i18n"
	"github.com/blindmaster24/MgkeTimetableBot/internal/logger"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/blindmaster24/MgkeTimetableBot/internal/parser"
	"github.com/mymmrac/telego"
)

type parseLogEntry struct {
	time    time.Time
	success bool
	msg     string
}

type Bot struct {
	client       *telego.Bot
	cfg          *config.Config
	log          *logger.Logger
	i18n         *i18n.Localizer
	chatRepo     *Repository
	cache        *cache.RaspCache
	cacheMu      sync.Mutex
	commands     map[string]Command
	commandOrder []Command
	callbacks    map[string]Callback
	parseFunc    func() error
	startTime    time.Time
	archive      any
	aliasRepo    *AliasRepository
	parseLogs    []parseLogEntry
	reportsMu    sync.Mutex
	reports      map[string]parser.Report
	textCommands []Command
	scenes       []sceneRoute
	google       googleService
	googleSyncMu sync.Mutex
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

type AdminCommand interface {
	AdminOnly() bool
}

type HiddenCommand interface {
	Hidden() bool
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

func (b *Bot) Client() *telego.Bot            { return b.client }
func (b *Bot) Config() *config.Config         { return b.cfg }
func (b *Bot) I18n() *i18n.Localizer          { return b.i18n }
func (b *Bot) Log() *logger.Logger            { return b.log }
func (b *Bot) GetRaspCache() *cache.RaspCache { return b.cache }
func (b *Bot) SetParseFunc(fn func() error)   { b.parseFunc = fn }

func (b *Bot) RegisterCommand(cmd Command) {
	if _, exists := b.commands[cmd.Name()]; !exists {
		b.commandOrder = append(b.commandOrder, cmd)
	}
	b.commands[cmd.Name()] = cmd
}

func (b *Bot) RegisterTextCommand(cmd Command) {
	b.textCommands = append(b.textCommands, cmd)
}

func (b *Bot) RegisterCallback(cb Callback) {
	b.callbacks[cb.Prefix()] = cb
}

func (b *Bot) registerAll() {
	b.scenes = b.buildSceneRoutes()
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
	b.RegisterCommand(&brovkaCmd{bot: b})
	b.RegisterCommand(&imageCmd{bot: b})
	b.RegisterCommand(&buttonsReloadCmd{bot: b})
	b.RegisterCommand(&forceParseCmd{bot: b})
	b.RegisterCommand(&resetCacheCmd{bot: b})
	b.RegisterCommand(&eulaCmd{bot: b})
	b.RegisterCommand(&apiCmd{bot: b})
	b.RegisterCommand(&devCmd{bot: b})
	b.RegisterCommand(&mathCmd{bot: b})
	b.RegisterCommand(&flushCacheCmd{bot: b})
	b.RegisterCommand(&debugCmd{bot: b})
	b.RegisterCommand(&sendCmd{bot: b})
	b.RegisterCommand(&triggerCmd{bot: b})
	b.RegisterCommand(&archiveStatsCmd{bot: b})
	b.RegisterCommand(&noticeDebugCmd{bot: b})
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
	b.registerMenus()
	b.RegisterCommand(&idCmd{bot: b})
	b.RegisterCommand(&errorCmd{bot: b})
	b.RegisterCommand(&testCmd{bot: b})
	b.RegisterCommand(&sqlCmd{bot: b})
	b.RegisterCommand(&restartCmd{bot: b})

	b.RegisterCallback(&callsFullCb{bot: b})
	b.RegisterCallback(&imageCb{bot: b})
	b.RegisterCallback(&cancelCb{bot: b})
	b.RegisterCallback(&answerCb{bot: b})
	b.RegisterCallback(&timetableGroupCb{bot: b})
	b.RegisterCallback(&timetableTeacherCb{bot: b})
	b.RegisterCallback(&googleCalCb{bot: b})
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

	bestPrefix, bestHandler := b.findCallback(cb.Data)
	if bestHandler != nil {
		if err := bestHandler.Handler(ctx, u); err != nil {
			b.log.Error().Err(err).Str("prefix", bestPrefix).Msg("callback error")
		}
	}
}

func (b *Bot) findCallback(data string) (string, Callback) {
	bestPrefix := ""
	var bestHandler Callback
	for prefix, handler := range b.callbacks {
		if strings.HasPrefix(data, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			bestHandler = handler
		}
	}
	return bestPrefix, bestHandler
}

func (b *Bot) SetMyCommands() error {
	params := &telego.SetMyCommandsParams{
		Commands: b.botCommands(false),
		Scope:    &telego.BotCommandScopeDefault{Type: "default"},
	}

	var errs []error
	if err := b.client.SetMyCommands(context.Background(), params); err != nil {
		errs = append(errs, err)
	}

	adminCommands := b.botCommands(true)
	for _, adminID := range b.cfg.Telegram.AdminIDs {
		scope := &telego.BotCommandScopeChat{
			Type:   "chat",
			ChatID: telego.ChatID{ID: adminID},
		}
		if err := b.client.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
			Commands: adminCommands,
			Scope:    scope,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (b *Bot) commandByName(name string) Command {
	name = strings.ToLower(strings.TrimPrefix(name, "/"))
	for _, cmd := range b.commandOrder {
		if strings.ToLower(strings.TrimPrefix(cmd.Name(), "/")) == name {
			return cmd
		}
	}
	return nil
}

func (b *Bot) botCommands(includeAdmin bool) []telego.BotCommand {
	cmds := make([]telego.BotCommand, 0, len(b.commandOrder))
	for _, cmd := range b.commandOrder {
		if hidden, ok := cmd.(HiddenCommand); ok && hidden.Hidden() {
			continue
		}

		admin := false
		if ac, ok := cmd.(AdminCommand); ok {
			admin = ac.AdminOnly()
		}
		if admin && !includeAdmin {
			continue
		}

		description := cmd.Description()
		if admin {
			description = "[адм] " + description
		}

		cmds = append(cmds, telego.BotCommand{
			Command:     strings.ToLower(strings.TrimPrefix(cmd.Name(), "/")),
			Description: description,
		})
	}
	return cmds
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

type namedBytes struct {
	name   string
	data   []byte
	offset int64
}

func (n *namedBytes) Name() string { return n.name }
func (n *namedBytes) Read(p []byte) (int, error) {
	if n.offset >= int64(len(n.data)) {
		return 0, io.EOF
	}
	c := copy(p, n.data[n.offset:])
	n.offset += int64(c)
	return c, nil
}

func namedBytesReader(name string, data []byte) *namedBytes {
	return &namedBytes{name: name, data: data}
}

func (b *Bot) SendDocument(chatID int64, filename string, data []byte, caption string) error {
	params := &telego.SendDocumentParams{
		ChatID:   telego.ChatID{ID: chatID},
		Document: telego.InputFile{File: namedBytesReader(filename, data)},
	}
	if caption != "" {
		params.Caption = caption
		params.ParseMode = "HTML"
	}
	_, err := b.client.SendDocument(context.Background(), params)
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

func (b *Bot) SendTextWithReplyKeyboard(chatID int64, text string, kb *telego.ReplyKeyboardMarkup) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: kb,
	})
	return err
}

func (b *Bot) SendTextWithButtons(chatID int64, text string, buttons []notification.KeyboardButton) error {
	var kb *telego.InlineKeyboardMarkup
	if len(buttons) > 0 {
		rows := make([][]telego.InlineKeyboardButton, 0, len(buttons))
		for _, btn := range buttons {
			rows = append(rows, []telego.InlineKeyboardButton{
				{Text: btn.Text, CallbackData: btn.Data},
			})
		}
		kb = &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
	}
	return b.SendTextWithKeyboard(chatID, text, kb)
}

func (b *Bot) RemoveReplyKeyboard(chatID int64) error {
	_, err := b.client.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   ".",
		ReplyMarkup: &telego.ReplyKeyboardRemove{
			RemoveKeyboard: true,
		},
	})
	return err
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

func (b *Bot) sendOrEdit(chatID int64, text string, chat *Chat, inlineKb *telego.InlineKeyboardMarkup) error {
	if chat.LastMsgID > 0 {
		err := b.EditMessageText(chatID, int(chat.LastMsgID), text, inlineKb)
		if err == nil {
			return nil
		}
	}
	var msg *telego.Message
	var err error
	if inlineKb != nil {
		msg, err = b.client.SendMessage(context.Background(), &telego.SendMessageParams{
			ChatID:      telego.ChatID{ID: chatID},
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: inlineKb,
		})
	} else {
		msg, err = b.client.SendMessage(context.Background(), &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: chatID},
			Text:      text,
			ParseMode: "HTML",
		})
	}
	if err != nil {
		return err
	}
	chat.LastMsgID = int64(msg.MessageID)
	b.chatRepo.Save(chat)
	return nil
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

func (b *Bot) RecordParserReport(report parser.Report) {
	b.reportsMu.Lock()
	defer b.reportsMu.Unlock()

	if b.reports == nil {
		b.reports = make(map[string]parser.Report)
	}
	b.reports[report.Source] = report
}

func (b *Bot) ParserReports() []parser.Report {
	b.reportsMu.Lock()
	defer b.reportsMu.Unlock()

	sources := make([]string, 0, len(b.reports))
	for source := range b.reports {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	reports := make([]parser.Report, 0, len(sources))
	for _, source := range sources {
		reports = append(reports, b.reports[source])
	}
	return reports
}

func (b *Bot) parserDiagnostics() []string {
	reports := b.ParserReports()
	if len(reports) == 0 {
		return nil
	}

	issues := 0
	lines := make([]string, 0, len(reports)+1)
	for _, report := range reports {
		failing := report.Failing()
		issues += len(failing) + len(report.Warnings)

		summary := fmt.Sprintf("%s: items=%d", report.Source, report.Items)
		if len(failing) > 0 {
			selectors := make([]string, 0, len(failing))
			for _, probe := range failing {
				selectors = append(selectors, fmt.Sprintf("%s (found %d)", probe.Selector, probe.Found))
			}
			summary += "\n   " + b.loc("parser_logs_no_data") + strings.Join(selectors, ", ")
		}
		for _, warning := range report.Warnings {
			summary += "\n   " + b.loc("parser_logs_warning_prefix") + warning
		}
		for _, fallback := range report.Fallbacks {
			summary += "\n   " + b.loc("parser_logs_fallback_prefix") + fallback
		}
		if report.URL != "" {
			summary += "\n   " + report.URL
		}
		lines = append(lines, summary)
	}

	if issues == 0 {
		lines = append(lines, b.loc("parser_logs_clean"))
	} else {
		lines = append([]string{b.loc("parser_logs_diagnostics")}, lines...)
	}

	return lines
}

func (b *Bot) AddParseLog(success bool, msg string) {
	entry := parseLogEntry{time: time.Now(), success: success, msg: msg}
	b.parseLogs = append(b.parseLogs, entry)
	if len(b.parseLogs) > 50 {
		b.parseLogs = b.parseLogs[len(b.parseLogs)-50:]
	}
}

func (b *Bot) GetParseLogs() []parseLogEntry {
	return b.parseLogs
}
