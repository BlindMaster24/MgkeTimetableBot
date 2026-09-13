package telegram

import (
	"context"
	"strings"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

const campusButtonPrefix = "🏫 Корпус: "
const campusAutoLabel = "авто"

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
	active := b.cache.GetCalls().Active

	weekdays := active.Schedule.Weekdays
	saturday := active.Schedule.Saturday

	if active.Source != "site" {
		return b.withFallbackCalls(weekdays, saturday)
	}

	if name := b.selectedCampus(chat); name != "" {
		if variant, ok := b.cache.GetCallsVariant(name); ok {
			return b.withFallbackCalls(variant.Weekdays, variant.Saturday)
		}
	}

	return b.withFallbackCalls(weekdays, saturday)
}

func (b *Bot) withFallbackCalls(weekdays, saturday [][2][2]string) cache.CallsSchedule {
	if len(weekdays) == 0 {
		weekdays = b.cfg.Timetable.Weekdays
		saturday = b.cfg.Timetable.Saturday
	}
	if len(saturday) == 0 {
		saturday = weekdays
	}
	return cache.CallsSchedule{Weekdays: weekdays, Saturday: saturday}
}

func campusButtonText(name string, selected bool) string {
	if selected {
		return "✅ " + campusButtonPrefix + name
	}
	return campusButtonPrefix + name
}

func campusButtonLabel(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "✅ ")
	if !strings.HasPrefix(text, campusButtonPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(text, campusButtonPrefix))
}

type callsCampusTextCmd struct {
	bot *Bot
}

func (c *callsCampusTextCmd) Name() string        { return "/calls_campus_text" }
func (c *callsCampusTextCmd) Description() string { return "" }
func (c *callsCampusTextCmd) Scene() string       { return sceneSettingsCalls }

func (c *callsCampusTextCmd) MatchText(text string) bool {
	return campusButtonLabel(text) != ""
}

func (c *callsCampusTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	name := campusButtonLabel(u.Text)
	if name != campusAutoLabel {
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
	if name == campusAutoLabel {
		chat.CallsCampus = ""
	}
	if err := c.bot.chatRepo.Save(chat); err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

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
