package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
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
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, fmt.Sprintf("peer_id: %d\nuser_id: %d", u.ChatID, u.UserID))
	}
	return u.Bot.SendText(u.ChatID, fmt.Sprintf("chat_id: %d\npeer_id: %d\nuser_id: %d", chat.ID, chat.PeerID, u.UserID))
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

type indexToStrDateCmd struct{ bot *Bot }

func (c *indexToStrDateCmd) Hidden() bool { return true }

func (c *indexToStrDateCmd) Name() string { return "/indexToStrDate" }
func (c *indexToStrDateCmd) Description() string {
	return "Перевести индекс дня в дату"
}
func commandWord(text string) string {
	word := bareCommand(text)
	if idx := strings.IndexByte(word, ' '); idx >= 0 {
		return word[:idx]
	}
	return word
}

func (c *indexToStrDateCmd) MatchText(text string) bool {
	switch commandWord(text) {
	case "indextodate", "indextostr", "indextostrdate",
		"dayindextodate", "dayindextostr", "dayindextostrdate":
		return true
	}
	return false
}
func (c *indexToStrDateCmd) Handler(ctx context.Context, u *Update) error {
	index, err := strconv.Atoi(strings.TrimSpace(extractCommandArg(u.Text)))
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Индекс дня не число")
	}
	return u.Bot.SendText(u.ChatID, utils.DayIndexToDate(index).Format("02.01.2006"))
}

type strToIndexCmd struct{ bot *Bot }

func (c *strToIndexCmd) Hidden() bool { return true }

func (c *strToIndexCmd) Name() string { return "/strToIndex" }
func (c *strToIndexCmd) Description() string {
	return "Перевести дату в индекс дня"
}
func (c *strToIndexCmd) MatchText(text string) bool {
	switch commandWord(text) {
	case "datetoindex", "strtoindex", "strdatetoindex",
		"datetodayindex", "strtodayindex", "strdatetodayindex":
		return true
	}
	return false
}
func (c *strToIndexCmd) Handler(ctx context.Context, u *Update) error {
	value := strings.TrimSpace(extractCommandArg(u.Text))
	if value == "" {
		return u.Bot.SendText(u.ChatID, "День не введён")
	}

	parsed, err := time.Parse("02.01.2006", value)
	if err != nil {
		if parsed, err = time.Parse("02.01.06", value); err != nil {
			return u.Bot.SendText(u.ChatID, "День не введён")
		}
	}

	return u.Bot.SendText(u.ChatID, strconv.Itoa(utils.DayIndexFromDate(parsed)))
}
