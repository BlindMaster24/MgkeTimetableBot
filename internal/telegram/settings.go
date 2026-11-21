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
		fmt.Sprintf(b.loc("settings_show_daily"), b.yesNo(chat.ShowDaily)),
		fmt.Sprintf(b.loc("settings_show_weekly"), b.yesNo(chat.ShowWeekly)),
		fmt.Sprintf(b.loc("settings_show_calls"), b.yesNo(chat.ShowCalls)),
		fmt.Sprintf(b.loc("settings_show_about"), b.yesNo(chat.ShowAbout)),
		fmt.Sprintf(b.loc("settings_show_group"), b.yesNo(chat.ShowFastGroup)),
		fmt.Sprintf(b.loc("settings_show_teacher"), b.yesNo(chat.ShowFastTeacher)),
		fmt.Sprintf(b.loc("settings_show_gcal"), b.yesNo(chat.ShowGoogleCalendar)),
		fmt.Sprintf(b.loc("settings_show_ics"), b.yesNo(chat.ShowICS)),
		"",
		fmt.Sprintf(b.loc("settings_hide_past"), b.yesNo(chat.HidePastDays)),
		fmt.Sprintf(b.loc("settings_parser_time"), b.yesNo(chat.ShowParserTime)),
		fmt.Sprintf(b.loc("settings_hints"), b.yesNo(chat.ShowHints)),
		fmt.Sprintf(b.loc("settings_delete_last_msg"), b.yesNo(chat.DeleteLastMsg)),
		fmt.Sprintf(b.loc("settings_delete_user_msg"), b.yesNo(chat.DeleteUserMsg)),
		fmt.Sprintf(b.loc("settings_diff"), b.yesNo(chat.DiffEnabled)),
		fmt.Sprintf(b.loc("settings_diff_week"), b.yesNo(chat.DiffAutoInWeek)),
		fmt.Sprintf(b.loc("settings_diff_updates"), b.yesNo(chat.DiffAutoInUpdates)),
		fmt.Sprintf(b.loc("settings_diff_before_after"), b.yesNo(chat.DiffShowBeforeAfter)),
		fmt.Sprintf(b.loc("settings_diff_limit"), chat.DiffMaxLines),
		"",
		fmt.Sprintf(b.loc("settings_notice_day"), b.yesNo(chat.NoticeChanges)),
		fmt.Sprintf(b.loc("settings_notice_week"), b.yesNo(chat.NoticeNextWeek)),
		fmt.Sprintf(b.loc("settings_notice_calls"), b.yesNo(chat.NoticeCalls)),
		fmt.Sprintf(b.loc("settings_notice_parser"), b.yesNo(chat.NoticeParserErrors)),
		"",
		b.loc("settings_system_header"),
		fmt.Sprintf(b.loc("settings_chat_mode"), string(chat.Mode), b.yesNo(chat.DeactivateSecondaryCheck)),
		fmt.Sprintf(b.loc("settings_selected_group"), b.orNone(chat.Group)),
		fmt.Sprintf(b.loc("settings_selected_teacher"), b.orNone(chat.Teacher)),
		fmt.Sprintf(b.loc("settings_last_message_id"), chat.LastMsgID),
		fmt.Sprintf(b.loc("settings_last_message_time"), chat.LastMsgTime),
		fmt.Sprintf(b.loc("settings_allow_messages"), b.yesNo(chat.AllowSendMess)),
		fmt.Sprintf(b.loc("settings_subscription"), b.yesNo(chat.SubscribeDistribution)),
		fmt.Sprintf(b.loc("settings_buttons_update"), b.yesNo(chat.NeedUpdateButtons)),
	}
	return strings.Join(lines, "\n")
}
