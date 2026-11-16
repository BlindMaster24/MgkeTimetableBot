package telegram

import (
	"fmt"
	"strings"
)

func (b *Bot) sendCallsShow(u *Update) error {
	chat, _ := b.chatRepo.FindOrCreate("telegram", u.UserID)
	b.showCalls(u, chat, true)
	return nil
}

func (b *Bot) currentSettingsText(chat *Chat) string {
	lines := []string{
		fmt.Sprintf(b.loc("settings_show_daily"), yesNo(chat.ShowDaily)),
		fmt.Sprintf(b.loc("settings_show_weekly"), yesNo(chat.ShowWeekly)),
		fmt.Sprintf(b.loc("settings_show_calls"), yesNo(chat.ShowCalls)),
		fmt.Sprintf(b.loc("settings_show_about"), yesNo(chat.ShowAbout)),
		fmt.Sprintf(b.loc("settings_show_group"), yesNo(chat.ShowFastGroup)),
		fmt.Sprintf(b.loc("settings_show_teacher"), yesNo(chat.ShowFastTeacher)),
		"",
		fmt.Sprintf(b.loc("settings_hide_past"), yesNo(chat.HidePastDays)),
		fmt.Sprintf(b.loc("settings_parser_time"), yesNo(chat.ShowParserTime)),
		fmt.Sprintf(b.loc("settings_hints"), yesNo(chat.ShowHints)),
		fmt.Sprintf(b.loc("settings_diff"), yesNo(chat.DiffEnabled)),
		fmt.Sprintf(b.loc("settings_diff_week"), yesNo(chat.DiffAutoInWeek)),
		fmt.Sprintf(b.loc("settings_diff_updates"), yesNo(chat.DiffAutoInUpdates)),
		fmt.Sprintf(b.loc("settings_diff_before_after"), yesNo(chat.DiffShowBeforeAfter)),
		fmt.Sprintf(b.loc("settings_diff_limit"), chat.DiffMaxLines),
		"",
		fmt.Sprintf(b.loc("settings_notice_day"), yesNo(chat.NoticeChanges)),
		fmt.Sprintf(b.loc("settings_notice_week"), yesNo(chat.NoticeNextWeek)),
		fmt.Sprintf(b.loc("settings_notice_calls"), yesNo(chat.NoticeCalls)),
		fmt.Sprintf(b.loc("settings_notice_parser"), yesNo(chat.NoticeParserErrors)),
		"",
		b.loc("settings_system_header"),
		fmt.Sprintf(b.loc("settings_chat_mode"), string(chat.Mode), yesNo(chat.DeactivateSecondaryCheck)),
		fmt.Sprintf(b.loc("settings_selected_group"), orNone(chat.Group)),
		fmt.Sprintf(b.loc("settings_selected_teacher"), orNone(chat.Teacher)),
		fmt.Sprintf(b.loc("settings_last_message_id"), chat.LastMsgID),
		fmt.Sprintf(b.loc("settings_last_message_time"), chat.LastMsgTime),
		fmt.Sprintf(b.loc("settings_allow_messages"), yesNo(chat.AllowSendMess)),
		fmt.Sprintf(b.loc("settings_subscription"), yesNo(chat.SubscribeDistribution)),
		fmt.Sprintf(b.loc("settings_buttons_update"), yesNo(chat.NeedUpdateButtons)),
	}
	return strings.Join(lines, "\n")
}
