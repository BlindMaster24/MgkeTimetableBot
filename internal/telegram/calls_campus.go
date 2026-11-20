package telegram

import (
	"context"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func (b *Bot) campusNames() []string {
	var names []string
	for _, variant := range b.cache.GetCalls().SiteVariants {
		if variant.Name == "" {
			continue
		}
		names = append(names, variant.Name)
	}
	return names
}

func (b *Bot) campusSelectionAvailable() bool {
	return len(b.campusNames()) > 1
}

func (b *Bot) configuredCampus() string {
	if b.cfg == nil || b.cfg.Parser.Calls == nil {
		return ""
	}
	return strings.TrimSpace(b.cfg.Parser.Calls.Campus)
}

func (b *Bot) selectedCampus(chat *Chat) string {
	if chat != nil && chat.CallsCampus != "" {
		return chat.CallsCampus
	}
	return b.configuredCampus()
}

func (b *Bot) callsScheduleFor(chat *Chat) cache.CallsSchedule {
	fallback := cache.CallsSchedule{Weekdays: b.cfg.Timetable.Weekdays, Saturday: b.cfg.Timetable.Saturday}
	return b.cache.ActiveCallsForCampus(b.selectedCampus(chat), fallback)
}

func (b *Bot) campusButtonText(name string, selected bool) string {
	if selected {
		return b.loc("campus_selected") + b.loc("campus_prefix") + name
	}
	return b.loc("campus_prefix") + name
}

func (b *Bot) campusButtonLabel(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "✅ ")
	if !strings.HasPrefix(text, b.loc("campus_prefix")) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(text, b.loc("campus_prefix")))
}

type callsCampusTextCmd struct {
	bot *Bot
}

func (c *callsCampusTextCmd) Name() string        { return "/calls_campus_text" }
func (c *callsCampusTextCmd) Description() string { return "" }
func (c *callsCampusTextCmd) Scene() string       { return sceneSettingsCalls }

func (c *callsCampusTextCmd) MatchText(text string) bool {
	return c.bot.campusButtonLabel(text) != ""
}

func (c *callsCampusTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	name := c.bot.campusButtonLabel(u.Text)
	if name != c.bot.loc("campus_auto") {
		known := false
		for _, candidate := range c.bot.campusNames() {
			if candidate == name {
				known = true
				break
			}
		}
		if !known {
			return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.callsMenuText(chat, c.bot.isAdmin(u.UserID)), c.bot.replyCallsSettings(chat, c.bot.isAdmin(u.UserID)))
		}
	}

	chat.CallsCampus = name
	if name == c.bot.loc("campus_auto") {
		chat.CallsCampus = ""
	}
	c.bot.saveChat(chat)

	lines := []string{c.bot.campusConfirmation(chat), c.bot.callsMenuText(chat, c.bot.isAdmin(u.UserID))}
	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, strings.Join(lines, "\n\n"), c.bot.replyCallsSettings(chat, c.bot.isAdmin(u.UserID)))
}

func (b *Bot) campusConfirmation(chat *Chat) string {
	selected := b.selectedCampus(chat)
	if selected == "" {
		return b.loc("calls_campus_auto")
	}
	return b.locData("calls_campus_selected", map[string]interface{}{"Campus": selected})
}

func (b *Bot) callsCampusLine(chat *Chat) string {
	if !b.campusSelectionAvailable() {
		return ""
	}

	selected := b.selectedCampus(chat)
	if selected == "" {
		return ""
	}
	return b.locData("calls_campus_current", map[string]interface{}{"Campus": selected})
}

func (b *Bot) callsCampusMenuLines(chat *Chat) []string {
	if !b.campusSelectionAvailable() {
		return nil
	}

	selected := b.selectedCampus(chat)
	if selected == "" {
		return []string{b.loc("calls_campus_choose")}
	}
	return []string{b.locData("calls_campus_current", map[string]interface{}{"Campus": selected})}
}
