package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
)

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

func (r *AliasRepository) EnsureTable() {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.db.Exec(`CREATE TABLE IF NOT EXISTS aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		telegram_user_id INTEGER NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		UNIQUE(telegram_user_id, key)
	)`)
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
	return result, nil
}

func (r *AliasRepository) Add(userID int64, key, value string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
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
func (c *aliasCmd) Description() string { return "Настройка алиасов" }
func (c *aliasCmd) MatchText(text string) bool {
	return text == "Алиасы" || text == "Настройка алиасов"
}
func (c *aliasCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}
	chat.Scene = "settings_alias"
	c.bot.chatRepo.Save(chat)
	return c.bot.showAliasMenu(u)
}

func (b *Bot) showAliasMenu(u *Update) error {
	kb := &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{{Text: "Список", CallbackData: "alias:list"}, {Text: "Добавить", CallbackData: "alias:add"}},
			{{Text: "Удалить", CallbackData: "alias:remove"}},
			{{Text: "Отчистить все", CallbackData: "alias:clear"}},
			{{Text: "Меню настроек", CallbackData: "settings"}, {Text: "Главное меню", CallbackData: "main_menu"}},
		},
	}
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Меню настройки алиасов.", kb)
}

type aliasCb struct{ bot *Bot }

func (cb *aliasCb) Prefix() string { return "alias:" }
func (cb *aliasCb) Handler(ctx context.Context, u *Update) error {
	cb.bot.AnswerCallback(u.Callback.ID, "")
	chat, err := cb.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, cb.bot.loc("data_not_loaded"))
	}

	action := strings.TrimPrefix(u.Data, "alias:")

	switch action {
	case "list":
		return cb.bot.showAliasList(u, u.UserID)
	case "add":
		chat.Scene = "alias_add"
		cb.bot.chatRepo.Save(chat)
		return u.Bot.SendText(u.ChatID, "Введите алиас в формате: оригинальное_название = замена")
	case "remove":
		return cb.bot.showAliasRemoveList(u, u.UserID)
	case "clear":
		cb.bot.aliasRepo.Clear(u.UserID)
		return cb.bot.showAliasMenu(u)
	}

	return cb.bot.showAliasMenu(u)
}

func (b *Bot) showAliasList(u *Update, userID int64) error {
	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения алиасов")
	}
	if len(aliases) == 0 {
		return u.Bot.SendText(u.ChatID, "У вас нет алиасов")
	}
	var lines []string
	lines = append(lines, "Ваши алиасы:")
	for i, a := range aliases {
		lines = append(lines, fmt.Sprintf("%d. %s → %s", i+1, a.Key, a.Value))
	}
	return u.Bot.SendText(u.ChatID, strings.Join(lines, "\n"))
}

func (b *Bot) showAliasRemoveList(u *Update, userID int64) error {
	aliases, err := b.aliasRepo.List(userID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, "Ошибка чтения алиасов")
	}
	if len(aliases) == 0 {
		return u.Bot.SendText(u.ChatID, "У вас нет алиасов для удаления")
	}
	var rows [][]telego.InlineKeyboardButton
	for _, a := range aliases {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: fmt.Sprintf("❌ %s → %s", a.Key, a.Value), CallbackData: fmt.Sprintf("alias:del:%s", a.Key)},
		})
	}
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "Назад", CallbackData: "alias:menu"},
	})
	return u.Bot.SendTextWithKeyboard(u.ChatID, "Выберите алиас для удаления:", &telego.InlineKeyboardMarkup{InlineKeyboard: rows})
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
	b.chatRepo.Save(chat)

	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		b.SendText(u.ChatID, "Неверный формат. Используйте: оригинальное_название = замена")
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		b.SendText(u.ChatID, "Неверный формат. Используйте: оригинальное_название = замена")
		return
	}

	if err := b.aliasRepo.Add(u.UserID, key, value); err != nil {
		b.SendText(u.ChatID, "Ошибка сохранения алиаса")
		return
	}

	b.SendText(u.ChatID, fmt.Sprintf("Алиас добавлен: %s → %s", key, value))
}
