package telegram

import (
	"context"
	"strings"
)

type BackupReport struct {
	Backup        bool
	Retention     bool
	Path          string
	Removed       int
	Kept          int
	PrunedRows    int
	RetentionDays int
}

func (b *Bot) SetBackupFunc(fn func(ctx context.Context) (BackupReport, error)) {
	b.backupFunc = fn
}

type backupCmd struct{ bot *Bot }

func (c *backupCmd) Name() string { return "/backup" }

func (c *backupCmd) AdminOnly() bool { return true }

func (c *backupCmd) Description() string { return c.bot.loc("cmd_backup") }

func (c *backupCmd) MatchText(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), "/backup")
}

func (c *backupCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	if c.bot.backupFunc == nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("backup_disabled"))
	}

	report, err := c.bot.backupFunc(ctx)
	lines := c.bot.backupReportLines(report)
	if err != nil {
		lines = append(lines, c.bot.locData("backup_error", map[string]any{"Error": err.Error()}))
	}
	if len(lines) == 0 {
		return u.Bot.SendText(u.ChatID, c.bot.loc("backup_disabled"))
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

func (b *Bot) backupReportLines(report BackupReport) []string {
	var lines []string
	if report.Backup {
		lines = append(lines, b.locData("backup_done", map[string]any{
			"Path":    report.Path,
			"Removed": report.Removed,
			"Kept":    report.Kept,
		}))
	}
	if report.Retention {
		lines = append(lines, b.locData("backup_pruned", map[string]any{
			"Rows": report.PrunedRows,
			"Days": report.RetentionDays,
		}))
	}
	return lines
}
