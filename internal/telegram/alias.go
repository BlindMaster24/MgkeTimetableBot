package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
)

const MAX_ALIASES = 50

var ErrAliasLimit = errors.New("alias limit reached")

type Alias struct {
	ID    int64
	Key   string
	Value string
}

type AliasRepository struct {
	db *Repository
}

func NewAliasRepository(chatRepo *Repository) *AliasRepository {
	return &AliasRepository{db: chatRepo}
}

func (r *AliasRepository) List(userID int64) ([]Alias, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	rows, err := r.db.db.Query(`SELECT id, key, value FROM aliases WHERE telegram_user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Alias
	for rows.Next() {
		var a Alias
		if err := rows.Scan(&a.ID, &a.Key, &a.Value); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *AliasRepository) Add(userID int64, key, value string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	existing := 0
	if err := r.db.db.QueryRow(`SELECT COUNT(*) FROM aliases WHERE telegram_user_id = ? AND key = ?`, userID, key).Scan(&existing); err != nil {
		return err
	}
	if existing == 0 {
		total := 0
		if err := r.db.db.QueryRow(`SELECT COUNT(*) FROM aliases WHERE telegram_user_id = ?`, userID).Scan(&total); err != nil {
			return err
		}
		if total >= MAX_ALIASES {
			return ErrAliasLimit
		}
	}

	_, err := r.db.db.Exec(`INSERT OR REPLACE INTO aliases (chat_id, telegram_user_id, key, value) VALUES (?, ?, ?, ?)`,
		userID, userID, key, value)
	return err
}

func (r *AliasRepository) Remove(userID int64, key string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	_, err := r.db.db.Exec(`DELETE FROM aliases WHERE telegram_user_id = ? AND key = ?`, userID, key)
	return err
}

func (r *AliasRepository) Clear(userID int64) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	_, err := r.db.db.Exec(`DELETE FROM aliases WHERE telegram_user_id = ?`, userID)
	return err
}

type aliasCmd struct{ bot *Bot }

func (c *aliasCmd) Name() string        { return "/alias" }
func (c *aliasCmd) Description() string { return c.bot.loc("aliastext_1") }
func (c *aliasCmd) MatchText(text string) bool {
	return text == c.bot.loc("aliastext_2") || text == c.bot.loc("aliastext_3")
}
func (c *aliasCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	chat.Scene = sceneSettingsAlias
	c.bot.saveChat(chat)
	return c.bot.showAliasMenu(u)
}

func (b *Bot) showAliasMenu(u *Update) error {
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, b.loc("aliastext_4"), b.replySettingsAliases())
}

func (b *Bot) showAliasList(u *Update, userID int64) error {
	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("aliastext_5"))
	}
	if len(aliases) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("aliastext_6"))
	}
	var lines []string
	lines = append(lines, b.loc("aliastext_7"))
	for i, a := range aliases {
		lines = append(lines, fmt.Sprintf("%d. %s → %s", i+1, utils.EscapeHTML(a.Key), utils.EscapeHTML(a.Value)))
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

func (b *Bot) showAliasRemoveList(u *Update, userID int64) error {
	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, b.loc("aliastext_8"))
	}
	if len(aliases) == 0 {
		return u.Bot.SendText(u.ChatID, b.loc("aliastext_9"))
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, b.loc("aliastext_10"), aliasRemoveKeyboard(b, aliases))
}

func aliasMenuLabel(b *Bot) string {
	if b == nil {
		return ""
	}
	return b.loc("aliastext_11")
}

func aliasRemoveKeyboard(b *Bot, aliases []Alias) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton
	for _, a := range aliases {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: fmt.Sprintf("❌ %s → %s", a.Key, a.Value), CallbackData: fmt.Sprintf("alias:del:%s", a.Key)},
		})
	}
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: aliasMenuLabel(b), CallbackData: "alias:menu"},
	})
	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}

type aliasDelCb struct{ bot *Bot }

func (cb *aliasDelCb) Prefix() string { return "alias:del:" }
func (cb *aliasDelCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	key := strings.TrimPrefix(u.Data, "alias:del:")
	cb.bot.aliasRepo.Remove(u.UserID, key)
	return cb.bot.showAliasRemoveList(u, u.UserID)
}

type aliasMenuCb struct{ bot *Bot }

func (cb *aliasMenuCb) Prefix() string { return "alias:menu" }
func (cb *aliasMenuCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	return cb.bot.showAliasMenu(u)
}

func (b *Bot) handleAliasAdd(ctx context.Context, u *Update, chat *Chat) {
	input := strings.TrimSpace(u.Text)
	chat.Scene = ""
	b.saveChat(chat)

	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		b.SendText(u.ChatID, b.loc("aliastext_12"))
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		b.SendText(u.ChatID, b.loc("aliastext_13"))
		return
	}

	if err := b.aliasRepo.Add(u.UserID, key, value); err != nil {
		if err == ErrAliasLimit {
			b.SendText(u.ChatID, fmt.Sprintf(b.loc("alias_limit"), MAX_ALIASES))
			return
		}
		b.SendText(u.ChatID, b.loc("aliastext_14"))
		return
	}

	b.SendText(u.ChatID, fmt.Sprintf(b.loc("aliastext_15"), utils.EscapeHTML(key), utils.EscapeHTML(value)))
}
