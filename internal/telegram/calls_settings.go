package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

type callsSettingsTextCmd struct {
	bot  *Bot
	kind string
}

func (c *callsSettingsTextCmd) Name() string        { return "/calls_settings_text_" + c.kind }
func (c *callsSettingsTextCmd) Description() string { return "" }
func (c *callsSettingsTextCmd) Scene() string       { return sceneSettingsCalls }

func (c *callsSettingsTextCmd) MatchText(text string) bool {
	switch c.kind {
	case "show":
		return text == c.bot.loc("callset_1")
	case "refresh":
		return text == c.bot.loc("callset_2")
	case "edit":
		return text == c.bot.loc("callset_3")
	case "source_site":
		return text == c.bot.loc("callset_4") || text == c.bot.loc("callset_5")
	case "source_manual":
		return text == c.bot.loc("callset_6") || text == c.bot.loc("callset_7")
	case "source_config":
		return text == c.bot.loc("callset_8") || text == c.bot.loc("callset_9")
	case "source_auto":
		return text == c.bot.loc("callset_10") || text == c.bot.loc("callset_11")
	}
	return false
}

func (c *callsSettingsTextCmd) Handler(ctx context.Context, u *Update) error {
	chat, err := c.bot.chatRepo.FindOrCreate("telegram", u.UserID)
	if err != nil {
		return u.Bot.SendText(u.ChatID, c.bot.loc("data_not_loaded"))
	}

	if c.kind == "show" {
		return c.bot.sendCallsShow(u)
	}

	if !c.bot.isAdmin(u.UserID) {
		return u.Bot.SendText(u.ChatID, c.bot.loc("callset_12"))
	}

	switch c.kind {
	case "refresh":
		return c.bot.refreshCallsNow(u, chat)
	case "edit":
		chat.Scene = sceneCallsEditInput
		c.bot.chatRepo.Save(chat)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, c.bot.loc("callset_13"), c.bot.replyCancel())
	case "source_site", "source_manual", "source_config":
		source := strings.TrimPrefix(c.kind, "source_")
		c.bot.applyCallsSource(source)
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf(c.bot.loc("callset_14"), c.bot.callsSourceLabel(source), c.bot.callsMenuText(chat, true)), c.bot.replyCallsSettings(chat, true))
	case "source_auto":
		c.bot.cache.ResetCallsOverride()
		return u.Bot.SendTextWithReplyKeyboard(u.ChatID, fmt.Sprintf(c.bot.loc("callset_15"), c.bot.callsMenuText(chat, true)), c.bot.replyCallsSettings(chat, true))
	}
	return nil
}

func (b *Bot) applyCallsSource(source string) {
	b.cache.SetCallsOverride(source)
	if source != "config" {
		return
	}
	calls := b.cache.GetCalls()
	calls.Active.Schedule = cache.CallsSchedule{
		Weekdays: b.cfg.Timetable.Weekdays,
		Saturday: b.cfg.Timetable.Saturday,
	}
	b.cache.SetCallsFromCache(calls)
}

func (b *Bot) refreshCallsNow(u *Update, chat *Chat) error {
	lines := []string{b.loc("callset_16")}

	var err error
	if b.parseFunc != nil {
		err = b.parseFunc()
	}

	calls := b.cache.GetCalls()
	siteParsed := len(calls.Site.Schedule.Weekdays) > 0

	switch {
	case err != nil:
		lines = append(lines, b.loc("callset_17"))
	case !siteParsed:
		lines = append(lines, b.loc("callset_18"))
	default:
		lines = append(lines, b.loc("callset_19"))
	}

	if siteParsed {
		lines = append(lines, b.loc("callset_20"))
	} else {
		lines = append(lines, b.loc("callset_21"))
	}

	switch {
	case calls.Site.UpdatedAtRaw != "":
		lines = append(lines, fmt.Sprintf(b.loc("callset_22"), calls.Site.UpdatedAtRaw))
	case siteParsed:
		lines = append(lines, "📅 "+b.loc("call_site_date_missing"))
		if at := b.siteCallsParsedAt(); at != "" {
			lines = append(lines, "🕒 "+b.locData("call_site_parsed_at", map[string]interface{}{"At": at}))
		}
	}

	active := calls.Active.Source
	if calls.OverrideSource != "" {
		active = calls.OverrideSource
	}
	lines = append(lines, fmt.Sprintf(b.loc("callset_23"), b.callsSourceLabel(active)))

	if calls.ManualReason != "" && calls.Active.Source == "manual" {
		lines = append(lines, fmt.Sprintf(b.loc("callset_24"), calls.ManualReason))
	}

	if err != nil {
		lines = append(lines, "🪠 "+err.Error())
	}

	schedule := b.callsScheduleFor(chat)
	if len(schedule.Weekdays) > 0 {
		lines = append(lines, b.loc("callset_25"))
		lines = append(lines, formatCallsPlain(schedule.Weekdays))
		lines = append(lines, b.loc("callset_26"))
		lines = append(lines, formatCallsPlain(schedule.Saturday))
	}

	return u.Bot.SendTextWithReplyKeyboard(u.ChatID, strings.Join(lines, "\n"), b.replyCallsSettings(chat, true))
}

func formatCallsPlain(slots [][2][2]string) string {
	var lines []string
	for i, slot := range slots {
		lines = append(lines, fmt.Sprintf("%d. %s - %s | %s - %s", i+1, slot[0][0], slot[0][1], slot[1][0], slot[1][1]))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) callsSourceLabel(source string) string {
	switch source {
	case "site":
		return b.loc("callset_27")
	case "manual":
		return b.loc("callset_28")
	default:
		return b.loc("callset_29")
	}
}

func (b *Bot) callsFreshnessLine() string {
	if b.cache == nil {
		return ""
	}
	calls := b.cache.GetCalls()
	if calls.Site.UpdatedAt <= 0 {
		return ""
	}
	if calls.Site.UpdatedAtRaw != "" {
		return b.locData("calls_freshness_site", map[string]interface{}{"Date": calls.Site.UpdatedAtRaw})
	}
	at := time.UnixMilli(calls.Site.UpdatedAt)
	return b.locData("calls_freshness_parsed", map[string]interface{}{
		"At":  at.Format("02.01.2006 15:04"),
		"Ago": b.callsAgeText(at),
	})
}

func (b *Bot) callsAgeText(at time.Time) string {
	age := b.nowTime().Sub(at)
	switch {
	case age < time.Minute:
		return b.loc("calls_freshness_just_now")
	case age < time.Hour:
		return b.locData("calls_freshness_minutes", map[string]interface{}{"Minutes": int(age.Minutes())})
	case age < 24*time.Hour:
		return b.locData("calls_freshness_hours", map[string]interface{}{"Hours": int(age.Hours())})
	default:
		return b.locData("calls_freshness_days", map[string]interface{}{"Days": int(age.Hours() / 24)})
	}
}

func (b *Bot) siteCallsParsedAt() string {
	if b.cache == nil {
		return ""
	}
	calls := b.cache.GetCalls()
	if calls.Site.UpdatedAtRaw != "" || calls.Site.UpdatedAt <= 0 {
		return ""
	}
	return time.UnixMilli(calls.Site.UpdatedAt).Format("02.01.2006 15:04")
}

func (b *Bot) callsMenuText(chat *Chat, admin bool) string {
	calls := b.cache.GetCalls()

	lines := []string{b.loc("callset_30")}
	lines = append(lines, b.callsCampusMenuLines(chat)...)
	if admin {
		if calls.OverrideSource != "" {
			lines = append(lines, fmt.Sprintf(b.loc("callset_31"), b.callsSourceLabel(calls.OverrideSource)))
		} else {
			lines = append(lines, fmt.Sprintf(b.loc("callset_32"), b.callsSourceLabel(calls.Active.Source)))
		}
	}

	if calls.OverrideSource == "" && calls.Active.Source != "site" && calls.SiteEmptyNotifiedAt > 0 {
		lines = append(lines, b.loc("callset_33"))
	}

	switch {
	case calls.Active.Source == "site" && calls.Site.UpdatedAtRaw != "":
		lines = append(lines, fmt.Sprintf(b.loc("callset_34"), calls.Site.UpdatedAtRaw))
	case calls.Active.Source == "site":
		lines = append(lines, b.loc("call_site_date_missing"))
		if at := b.siteCallsParsedAt(); at != "" {
			lines = append(lines, b.locData("call_site_parsed_at", map[string]interface{}{"At": at}))
		}
	case calls.Active.Source == "manual" && calls.ManualReason != "":
		lines = append(lines, fmt.Sprintf(b.loc("callset_35"), calls.ManualReason))
	case calls.Active.Source == "config" && len(b.cfg.Timetable.Weekdays) > 0:
		lines = append(lines, b.loc("callset_36"))
	}

	return strings.Join(lines, "\n")
}

func (b *Bot) showCallsSettingsReply(u *Update, chat *Chat) error {
	admin := b.isAdmin(u.UserID)
	return b.SendTextWithReplyKeyboard(u.ChatID, b.callsMenuText(chat, admin), b.replyCallsSettings(chat, admin))
}
