package telegram

import (
	"context"
	"fmt"
)

type chatCmd struct{ bot *Bot }

func (c *chatCmd) Hidden() bool { return true }

func (c *chatCmd) Name() string        { return "/chat" }
func (c *chatCmd) Description() string { return "Просмотр информации о чате" }
func (c *chatCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка")
	}
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("<pre>%+v</pre>", chat))
}

type idCmd struct{ bot *Bot }

func (c *idCmd) Hidden() bool { return true }

func (c *idCmd) Name() string        { return "/id" }
func (c *idCmd) Description() string { return "ID чата и пользователя" }
func (c *idCmd) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("chat_id: %d\nuser_id: %d", u.ChatID, u.UserID))
}

type errorCmd struct{ bot *Bot }

func (c *errorCmd) Hidden() bool { return true }

func (c *errorCmd) Name() string        { return "/error" }
func (c *errorCmd) Description() string { return "Тестовая ошибка" }
func (c *errorCmd) Handler(ctx context.Context, u *Update) error {
	return fmt.Errorf("test error")
}

type testCmd struct{ bot *Bot }

func (c *testCmd) Hidden() bool { return true }

func (c *testCmd) Name() string        { return "/test" }
func (c *testCmd) Description() string { return "Тестовая команда" }
func (c *testCmd) Handler(ctx context.Context, u *Update) error {
	return nil
}
