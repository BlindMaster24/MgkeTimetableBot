package telegram

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type regexpCmd struct{ bot *Bot }

func (c *regexpCmd) Name() string { return "/regexp" }
func (c *regexpCmd) Description() string {
	return "Отобразить все команды и регулярки"
}
func (c *regexpCmd) MatchText(text string) bool {
	return text == "/regexp"
}
func (c *regexpCmd) Handler(ctx context.Context, u *Update) error {
	var lines []string
	for name := range c.bot.commands {
		lines = append(lines, name)
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type vanishCmd struct{ bot *Bot }

func (c *vanishCmd) Name() string        { return "/vanish" }
func (c *vanishCmd) Description() string { return "Почистить базу данных" }
func (c *vanishCmd) MatchText(text string) bool {
	return text == "/vanish" || text == "/vacuum"
}
func (c *vanishCmd) Handler(ctx context.Context, u *Update) error {
	c.bot.chatRepo.mu.Lock()
	c.bot.chatRepo.db.Exec("VACUUM")
	c.bot.chatRepo.mu.Unlock()
	return u.Bot.SendText(u.ChatID, "БД почищена")
}

type parserLogsCmd struct{ bot *Bot }

func (c *parserLogsCmd) Name() string { return "/parserLogs" }
func (c *parserLogsCmd) Description() string {
	return "Логи последних обновлений парсера"
}
func (c *parserLogsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/parserlogs" || lower == "/updaterlogs" || lower == "/getparserlogs" || lower == "/getupdaterlogs"
}
func (c *parserLogsCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	logs := c.bot.GetParseLogs()
	if len(logs) == 0 {
		return u.Bot.SendText(u.ChatID, "Логов нет")
	}
	var lines []string
	for i, entry := range logs {
		icon := "✅"
		if !entry.success {
			icon = "❌"
		}
		lines = append(lines, fmt.Sprintf("%d. %s [%s]: %s", i+1, icon, entry.time.Format("02.01 15:04:05"), entry.msg))
	}
	text := strings.Join(lines, "\n")
	if len(text) > 4096 {
		text = text[len(text)-4096:]
	}
	return u.Bot.SendText(u.ChatID, text)
}

type requireNewButtonsCmd struct{ bot *Bot }

func (c *requireNewButtonsCmd) Name() string { return "/requireNewButtons" }
func (c *requireNewButtonsCmd) Description() string {
	return "Обновить клавиатуру пользователям"
}
func (c *requireNewButtonsCmd) MatchText(text string) bool {
	return text == "/requireNewButtons"
}
func (c *requireNewButtonsCmd) Handler(ctx context.Context, u *Update) error {
	c.bot.chatRepo.mu.Lock()
	c.bot.chatRepo.db.Exec("UPDATE chats SET accepted = accepted WHERE accepted = 1")
	c.bot.chatRepo.mu.Unlock()
	return u.Bot.SendText(u.ChatID, "ok")
}

type createApiKeyCmd struct{ bot *Bot }

func (c *createApiKeyCmd) Name() string        { return "/createApiKey" }
func (c *createApiKeyCmd) Description() string { return "Создать API токен" }
func (c *createApiKeyCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/createapi")
}
func (c *createApiKeyCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "Создание API токена пока недоступно")
}

type decryptKeyCmd struct{ bot *Bot }

func (c *decryptKeyCmd) Name() string        { return "/decryptKey" }
func (c *decryptKeyCmd) Description() string { return "Дешифровать ключ" }
func (c *decryptKeyCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/decrypt")
}
func (c *decryptKeyCmd) Handler(ctx context.Context, u *Update) error {
	key := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(u.Text, "/decryptKey"), "/decrypt"))
	if key == "" {
		return u.Bot.SendText(u.ChatID, "Ключ не указан")
	}
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("Дешифровка ключа пока недоступна"))
}

type sqlCmd struct{ bot *Bot }

func (c *sqlCmd) Name() string        { return "/sql" }
func (c *sqlCmd) Description() string { return "Выполнить SQL запрос" }
func (c *sqlCmd) MatchText(text string) bool {
	return strings.HasPrefix(strings.ToLower(text), "/sql")
}
func (c *sqlCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	query := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(u.Text, "/sql"), "/SQL"))
	if query == "" {
		return u.Bot.SendText(u.ChatID, "Укажите SQL запрос после /sql")
	}
	c.bot.chatRepo.mu.Lock()
	defer c.bot.chatRepo.mu.Unlock()
	rows, err := c.bot.chatRepo.db.Query(query)
	if err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("❌ Ошибка: %v", err))
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
		rows.Scan(ptrs...)
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
	if len(lines) > 50 {
		lines = lines[:50]
		lines = append(lines, "... (обрезано)")
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

type restartCmd struct{ bot *Bot }

func (c *restartCmd) Name() string        { return "/restart" }
func (c *restartCmd) Description() string { return "Перезапустить бота" }
func (c *restartCmd) MatchText(text string) bool {
	return text == "/restart"
}
func (c *restartCmd) Handler(ctx context.Context, u *Update) error {
	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, "⛔ Доступ запрещён")
	}
	u.Bot.SendText(u.ChatID, "🔄 Перезапуск...")
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}
