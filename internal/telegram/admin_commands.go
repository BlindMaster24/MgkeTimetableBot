package telegram

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/health"
	"github.com/blindmaster24/MgkeTimetableBot/internal/notification"
	"github.com/mymmrac/telego"
)

const incidentHistoryLimit = 20

type regexpCmd struct{ bot *Bot }

func (c *regexpCmd) Name() string { return "/regexp" }

func (c *regexpCmd) AdminOnly() bool { return true }
func (c *regexpCmd) Description() string {
	return c.bot.loc("admcmd_1")
}
func (c *regexpCmd) MatchText(text string) bool {
	return text == "/regexp"
}
func (c *regexpCmd) Handler(ctx context.Context, u *Update) error {
	lines := make([]string, 0, len(c.bot.commands))
	for name := range c.bot.commands {
		lines = append(lines, name)
	}
	sort.Strings(lines)
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type vanishCmd struct{ bot *Bot }

func (c *vanishCmd) AdminOnly() bool { return true }

func (c *vanishCmd) Name() string        { return "/vanish" }
func (c *vanishCmd) Description() string { return c.bot.loc("admcmd_2") }
func (c *vanishCmd) MatchText(text string) bool {
	return text == "/vanish" || text == "/vacuum"
}
func (c *vanishCmd) Handler(ctx context.Context, u *Update) error {
	c.bot.chatRepo.mu.Lock()
	_, err := c.bot.chatRepo.db.Exec("VACUUM")
	c.bot.chatRepo.mu.Unlock()
	if err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("admcmd_3"), err))
	}
	return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_4"))
}

type parserLogsCmd struct{ bot *Bot }

func (c *parserLogsCmd) Name() string { return "/parserLogs" }

func (c *parserLogsCmd) AdminOnly() bool { return true }
func (c *parserLogsCmd) Description() string {
	return c.bot.loc("admcmd_5")
}
func (c *parserLogsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/parserlogs" || lower == "/updaterlogs" || lower == "/getparserlogs" || lower == "/getupdaterlogs"
}
func (c *parserLogsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_6"))
	}
	logs := c.bot.GetParseLogs()
	var lines []string
	for i, entry := range logs {
		icon := "✅"
		if !entry.success {
			icon = "❌"
		}
		lines = append(lines, fmt.Sprintf("%d. %s [%s]: %s", i+1, icon, entry.time.Format("02.01 15:04:05"), entry.msg))
	}

	if diagnostics := c.bot.parserDiagnostics(); len(diagnostics) > 0 {
		lines = append(lines, diagnostics...)
	}

	if len(lines) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("parser_logs_no_logs"))
	}
	text := strings.Join(lines, "\n")
	if len(text) > 4096 {
		text = text[len(text)-4096:]
	}
	return u.Bot.SendText(u.ChatID, text)
}

type parserHealthCmd struct{ bot *Bot }

func (c *parserHealthCmd) Name() string { return "/parserhealth" }

func (c *parserHealthCmd) AdminOnly() bool { return true }
func (c *parserHealthCmd) Description() string {
	return c.bot.loc("cmd_parserhealth")
}
func (c *parserHealthCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_7"))
	}
	kb := buttonsKeyboard(notification.HealthAlertButtons(health.AlertParserGuard, func(k string) string { return c.bot.loc(k) }))
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.parserHealthText(), kb)
}

func (b *Bot) parserHealthText() string {
	if b.health == nil {
		return b.loc("admcmd_8")
	}

	snapshot := b.health.Snapshot()
	parser := snapshot.Parser

	lines := []string{b.loc("admcmd_9")}
	state := b.loc("admcmd_10")
	if parser.ConsecutiveFailures > 0 {
		state = fmt.Sprintf(b.loc("admcmd_11"), parser.ConsecutiveFailures)
	}
	lines = append(lines, b.loc("admcmd_12")+state)
	lines = append(lines, fmt.Sprintf(b.loc("admcmd_13"), parser.Runs, parser.Errors))
	lines = append(lines, b.loc("admcmd_14")+b.healthMoment(parser.LastSuccessAt, parser.LagSeconds))
	if parser.LastErrorAt != "" {
		lines = append(lines, fmt.Sprintf(b.loc("admcmd_15"), healthTimestamp(parser.LastErrorAt), parser.LastError))
	}
	lines = append(lines, fmt.Sprintf(b.loc("admcmd_16"), parser.LastDurationMS))

	if len(parser.Layout) > 0 {
		lines = append(lines, "", fmt.Sprintf(b.loc("admcmd_17"), parser.LayoutFailures))
		for _, issue := range parser.Layout {
			if issue.Advisory {
				lines = append(lines, b.locData("parser_health_layout_advisory", map[string]interface{}{
					"Source":   issue.Source,
					"Selector": issue.Selector,
				}))
				if issue.Source == "calls" {
					if at := b.siteCallsParsedAt(); at != "" {
						lines = append(lines, b.locData("parser_health_calls_parsed_at", map[string]interface{}{"At": at}))
					}
				}
				continue
			}
			if issue.Missing > 0 {
				lines = append(lines, b.locData("parser_health_layout_skipped", map[string]interface{}{
					"Source":   issue.Source,
					"Selector": issue.Selector,
					"Count":    issue.Missing,
				}))
				continue
			}
			lines = append(lines, fmt.Sprintf(b.loc("admcmd_18"), issue.Source, issue.Selector, issue.Found))
		}
	}

	if len(parser.Guard) > 0 {
		lines = append(lines, "", fmt.Sprintf(b.loc("admcmd_19"), parser.GuardFailures))
		for _, issue := range parser.Guard {
			lines = append(lines, fmt.Sprintf("%s: %s: %s", issue.Source, issue.Reason, issue.Detail))
		}
	}

	if len(snapshot.Alerts) == 0 {
		lines = append(lines, "", b.loc("admcmd_20"))
	} else {
		lines = append(lines, "", b.loc("admcmd_21"))
		for _, alert := range snapshot.Alerts {
			lines = append(lines, fmt.Sprintf("⚠️ %s: %s", alert.Key, alert.Detail))
		}
	}

	if b.cache != nil {
		lines = append(lines, "", b.loc("admcmd_22"), fmt.Sprintf(b.loc("admcmd_23"), len(b.cache.GetGroups()), len(b.cache.GetTeachers())))
	}

	lines = append(lines, "", b.loc("admcmd_24"))
	return strings.Join(lines, "\n")
}

type incidentsCmd struct{ bot *Bot }

func (c *incidentsCmd) Name() string { return "/incidents" }

func (c *incidentsCmd) AdminOnly() bool { return true }
func (c *incidentsCmd) Description() string {
	return c.bot.loc("cmd_incidents")
}
func (c *incidentsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_25"))
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, c.bot.incidentsText(), c.bot.incidentButtons())
}

func (b *Bot) incidentsText() string {
	if b.incidents == nil {
		return b.loc("incidents_unavailable")
	}

	records := b.incidents.Recent(incidentHistoryLimit)
	lines := []string{b.locData("incidents_header", map[string]interface{}{"Count": incidentHistoryLimit})}
	if diagnostics := b.apiDiagnostics(); diagnostics != "" {
		lines = append(lines, diagnostics)
	}
	if len(records) == 0 {
		return strings.Join(append(lines, b.loc("incidents_empty")), "\n")
	}

	now := time.Now()
	for _, record := range records {
		lines = append(lines, b.incidentLine(record, now))
		if record.Detail != "" {
			lines = append(lines, indentBlock(record.Detail))
		}
		lines = append(lines, "   "+b.loc("incidents_outcome")+": "+b.incidentOutcome(record))
	}

	return strings.Join(lines, "\n")
}

func (b *Bot) apiDiagnostics() string {
	if b.health == nil {
		return ""
	}

	api := b.health.Snapshot().API
	if len(api.Endpoints) == 0 && len(api.LastErrors) == 0 {
		return ""
	}

	lines := []string{b.loc("api_diag_header")}
	for index, endpoint := range api.Endpoints {
		if index == apiDiagnosticsLimit {
			break
		}
		line := b.locData("api_diag_endpoint", map[string]interface{}{
			"Label":    apiEndpointLabel(endpoint.Method, endpoint.Path),
			"Requests": endpoint.Requests,
			"Errors":   endpoint.Errors,
			"Avg":      b.apiDuration(time.Duration(endpoint.AvgMillis) * time.Millisecond),
			"Max":      b.apiDuration(time.Duration(endpoint.MaxMillis) * time.Millisecond),
		})
		if endpoint.Slow {
			line = "🐌 " + line
		}
		if endpoint.Message != "" {
			line += " — " + endpoint.Message
		}
		lines = append(lines, line)
	}

	if len(api.LastErrors) > 0 {
		lines = append(lines, b.loc("api_diag_last"))
		for index, sample := range api.LastErrors {
			if index == apiDiagnosticsLimit {
				break
			}
			line := fmt.Sprintf("%s %s %d", apiSampleClock(sample.At), apiEndpointLabel(sample.Method, sample.Path), sample.Status)
			if sample.Message != "" {
				line += " — " + sample.Message
			}
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func apiEndpointLabel(method, path string) string {
	if method == "" {
		return path
	}
	return method + " " + path
}

func apiSampleClock(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.Local().Format("15:04:05")
}

func indentBlock(text string) string {
	parts := strings.Split(text, "\n")
	for index, part := range parts {
		parts[index] = "   " + part
	}
	return strings.Join(parts, "\n")
}

func (b *Bot) incidentLine(record health.Incident, now time.Time) string {
	icon := "✅"
	switch {
	case record.Resolution == health.ResolutionManual:
		icon = "🔧"
	case record.Open():
		icon = "⚠️"
	}

	window := b.loc("incidents_since") + " " + healthClock(record.StartedAt)
	if !record.Open() {
		window = healthClock(record.StartedAt) + "–" + healthClock(record.EndedAt)
	}

	return fmt.Sprintf("%s %s (%s)\n   %s — %s", icon, window, record.Duration(now).Truncate(time.Second), notification.HealthAlertTitle(record.Key, func(k string) string { return b.loc(k) }), record.Key)
}

func (b *Bot) incidentOutcome(record health.Incident) string {
	if record.Note != "" {
		return record.Note
	}
	if record.Open() {
		return b.loc("incidents_outcome_open")
	}
	return b.loc("incidents_outcome_auto")
}

func (b *Bot) incidentButtons() *telego.InlineKeyboardMarkup {
	open := b.incidents.Open()
	var buttons []notification.KeyboardButton
	seen := map[string]bool{}
	for _, record := range open {
		for _, button := range notification.HealthAlertButtons(record.Key, func(k string) string { return b.loc(k) }) {
			if seen[button.Data] {
				continue
			}
			seen[button.Data] = true
			buttons = append(buttons, button)
		}
	}
	return buttonsKeyboard(buttons)
}

const apiDiagnosticsLimit = 5

func healthClock(value time.Time) string {
	return value.Local().Format("02.01 15:04:05")
}

func healthTimestamp(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.Local().Format("02.01 15:04:05")
}

func (b *Bot) healthMoment(value string, lagSeconds int64) string {
	if value == "" {
		return b.loc("admcmd_26")
	}
	if lagSeconds < 0 {
		return healthTimestamp(value)
	}
	return fmt.Sprintf(b.loc("admcmd_27"), healthTimestamp(value), time.Duration(lagSeconds)*time.Second)
}

type requireNewButtonsCmd struct{ bot *Bot }

func (c *requireNewButtonsCmd) Name() string { return "/requireNewButtons" }

func (c *requireNewButtonsCmd) AdminOnly() bool { return true }
func (c *requireNewButtonsCmd) Description() string {
	return c.bot.loc("admcmd_28")
}
func (c *requireNewButtonsCmd) MatchText(text string) bool {
	return text == "/requireNewButtons"
}
func (c *requireNewButtonsCmd) Handler(ctx context.Context, u *Update) error {
	c.bot.chatRepo.mu.Lock()
	c.bot.chatRepo.db.Exec("UPDATE bot_chats SET need_update_buttons = 1 WHERE accepted = 1")
	c.bot.chatRepo.mu.Unlock()
	return u.Bot.SendText(u.ChatID, "ok")
}

type createApiKeyCmd struct{ bot *Bot }

func (c *createApiKeyCmd) AdminOnly() bool { return true }

func (c *createApiKeyCmd) Name() string        { return "/createApiKey" }
func (c *createApiKeyCmd) Description() string { return c.bot.loc("cmd_createapikey") }
func (c *createApiKeyCmd) MatchText(text string) bool {
	normalized := bareCommand(text)
	return strings.HasPrefix(normalized, "createapikey") || strings.HasPrefix(normalized, "createapitoken")
}
func (c *createApiKeyCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_29"))
	}

	fields := strings.Fields(strings.TrimSpace(u.Text))
	usage := c.bot.locData("api_create_usage", map[string]interface{}{"Command": "/createApiKey"})
	if len(fields) == 0 {
		return u.Bot.SendText(u.ChatID, usage)
	}

	usage = c.bot.locData("api_create_usage", map[string]interface{}{"Command": fields[0]})
	args := fields[1:]
	if len(args) == 0 || len(args) > 2 {
		return u.Bot.SendText(u.ChatID, usage)
	}

	chatID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return u.Bot.SendText(u.ChatID, usage)
	}

	limit := 0
	hasLimit := false
	if len(args) == 2 {
		limit, err = strconv.Atoi(args[1])
		if err != nil || limit < 0 {
			return u.Bot.SendText(u.ChatID, usage)
		}
		hasLimit = true
	}

	if c.bot.keys == nil || !c.bot.keys.Enabled() {
		return u.Bot.SendText(u.ChatID, c.bot.loc("api_disabled"))
	}

	_, created, err := c.bot.keys.FindOrCreate(chatID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.apiKeyError(err))
	}

	if !created {
		if err := c.bot.keys.Rotate(chatID); err != nil {
			return u.Bot.SendText(u.ChatID, c.bot.apiKeyError(err))
		}
	}

	if hasLimit {
		if err := c.bot.keys.SetLimit(chatID, limit); err != nil {
			return u.Bot.SendText(u.ChatID, c.bot.apiKeyError(err))
		}
	}

	key, err := c.bot.keys.ByChatID(chatID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.apiKeyError(err))
	}

	token, err := c.bot.keys.Token(key)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.apiKeyError(err))
	}

	state := "api_create_added"
	if !created {
		state = "api_create_updated"
	}

	lines := []string{
		c.bot.locData("api_create_done", map[string]interface{}{"State": c.bot.loc(state)}),
		c.bot.locData("api_create_id", map[string]interface{}{"ID": key.ID}),
		c.bot.locData("api_create_key", map[string]interface{}{"Key": token}),
		c.bot.locData("api_create_limit", map[string]interface{}{"Limit": key.LimitPerSec}),
		c.bot.locData("api_create_iv", map[string]interface{}{"IV": base64.RawURLEncoding.EncodeToString(key.IV)}),
	}

	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

var readOnlyQueryPrefixes = map[string]bool{
	"select":  true,
	"with":    true,
	"pragma":  true,
	"explain": true,
	"values":  true,
}

func readOnlyQuery(query string) bool {
	// One statement only: the driver hands the text to SQLite as is, so a
	// trailing "; DELETE ..." would run after the read.
	if strings.Contains(query, ";") {
		return false
	}
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return false
	}
	return readOnlyQueryPrefixes[strings.ToLower(fields[0])]
}

func trimSqlPrefix(text string) string {
	lower := strings.ToLower(text)
	for _, prefix := range []string{"/sql_run", "/sqlrun", "/db_run", "/dbrun", "/sql", "/db"} {
		if strings.HasPrefix(lower, prefix) {
			return text[len(prefix):]
		}
	}
	return text
}

type sqlCmd struct{ bot *Bot }

func (c *sqlCmd) AdminOnly() bool { return true }

func (c *sqlCmd) Name() string        { return "/sql" }
func (c *sqlCmd) Description() string { return c.bot.loc("admcmd_30") }
func (c *sqlCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return strings.HasPrefix(lower, "/sql") || strings.HasPrefix(lower, "/db")
}
func (c *sqlCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_31"))
	}
	query := strings.TrimSpace(trimSqlPrefix(u.Text))
	if query == "" {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_32"))
	}
	if !readOnlyQuery(query) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("sql_read_only"))
	}

	c.bot.chatRepo.mu.Lock()
	defer c.bot.chatRepo.mu.Unlock()

	tx, err := c.bot.chatRepo.db.BeginTx(ctx, nil)
	if err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("admcmd_33"), err))
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("admcmd_34"), err))
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var lines []string
	lines = append(lines, strings.Join(cols, " | "))
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("admcmd_35"), err))
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			switch val := v.(type) {
			case []byte:
				parts[i] = string(val)
			case nil:
				parts[i] = "NULL"
			default:
				parts[i] = fmt.Sprintf("%v", val)
			}
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	if err := rows.Err(); err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf(c.bot.loc("admcmd_36"), err))
	}
	if len(lines) > 50 {
		lines = lines[:50]
		lines = append(lines, c.bot.loc("admcmd_37"))
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type restartCmd struct {
	bot   *Bot
	delay time.Duration
	exit  func(int)
}

func (c *restartCmd) AdminOnly() bool { return true }

func (c *restartCmd) Name() string { return "/restart" }
func (c *restartCmd) Description() string {
	return c.bot.loc("admcmd_38")
}
func (c *restartCmd) MatchText(text string) bool {
	return text == "/restart"
}
func (c *restartCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_39"))
	}
	u.Bot.SendText(u.ChatID, c.bot.loc("admcmd_40"))
	if c.bot.shutdown != nil {
		c.bot.shutdown()
	}
	go func() {
		time.Sleep(c.wait())
		c.quit()(0)
	}()
	return nil
}

func (c *restartCmd) wait() time.Duration {
	if c.delay > 0 {
		return c.delay
	}
	return 500 * time.Millisecond
}

func (c *restartCmd) quit() func(int) {
	if c.exit != nil {
		return c.exit
	}
	return os.Exit
}
