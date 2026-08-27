package telegram

import "context"

type timetableCb struct{ bot *Bot }

func (cb *timetableCb) Prefix() string { return "day" }
func (cb *timetableCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type callsCb struct{ bot *Bot }

func (cb *callsCb) Prefix() string { return "calls" }
func (cb *callsCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.formatCallsSchedule())
}

type imageCb struct{ bot *Bot }

func (cb *imageCb) Prefix() string { return "image" }
func (cb *imageCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type cancelCb struct{ bot *Bot }

func (cb *cancelCb) Prefix() string { return "cancel" }
func (cb *cancelCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("input_cancelled"))
}

type setupCb struct{ bot *Bot }

func (cb *setupCb) Prefix() string { return "setup" }
func (cb *setupCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("setup_started"))
}

type dayCb struct{ bot *Bot }

func (cb *dayCb) Prefix() string { return "day:" }
func (cb *dayCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type weekCb struct{ bot *Bot }

func (cb *weekCb) Prefix() string { return "week:" }
func (cb *weekCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}

type aboutCb struct{ bot *Bot }

func (cb *aboutCb) Prefix() string { return "about" }
func (cb *aboutCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("about_bot"))
}

type groupCb struct{ bot *Bot }

func (cb *groupCb) Prefix() string { return "group" }
func (cb *groupCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_group_number"))
}

type teacherCb struct{ bot *Bot }

func (cb *teacherCb) Prefix() string { return "teacher" }
func (cb *teacherCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("enter_teacher_name"))
}

type settingsCb struct{ bot *Bot }

func (cb *settingsCb) Prefix() string { return "settings" }
func (cb *settingsCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendTextWithKeyboard(u.ChatID, cb.bot.loc("settings_menu"), settingsKeyboard(cb.bot.i18n.T))
}

type icsCb struct{ bot *Bot }

func (cb *icsCb) Prefix() string { return "ics" }
func (cb *icsCb) Handler(ctx context.Context, u *Update) error {
	return u.Bot.SendText(u.ChatID, cb.bot.loc("need_group"))
}
