package telegram

import (
	"context"
	"fmt"
	"strings"
)

type regexpCmd struct{ bot *Bot }

func (c *regexpCmd) Name() string        { return "/regexp" }
func (c *regexpCmd) Description() string { return "Отобразить все команды и регулярки" }
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

func (c *parserLogsCmd) Name() string        { return "/parserLogs" }
func (c *parserLogsCmd) Description() string { return "Логи последних обновлений парсера" }
func (c *parserLogsCmd) MatchText(text string) bool {
	lower := strings.ToLower(text)
	return lower == "/parserlogs" || lower == "/updaterlogs" || lower == "/getparserlogs" || lower == "/getupdaterlogs"
}
func (c *parserLogsCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, "Логи парсера пока недоступны")
}

type requireNewButtonsCmd struct{ bot *Bot }

func (c *requireNewButtonsCmd) Name() string        { return "/requireNewButtons" }
func (c *requireNewButtonsCmd) Description() string { return "Обновить клавиатуру пользователям" }
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
