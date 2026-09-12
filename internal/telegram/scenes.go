package telegram

import (
	"context"
	"strings"
)

const (
	sceneSetup             = "setup"
	sceneSettings          = "settings"
	sceneSettingsSchedules = "settings_schedules"
	sceneSettingsCalls     = "settings_calls"
	sceneSettingsAlias     = "settings_alias"

	sceneSetGroup         = "set_group"
	sceneSetTeacher       = "set_teacher"
	sceneGetGroup         = "get_group"
	sceneGetTeacher       = "get_teacher"
	sceneSubAddGroup      = "sub_add_group"
	sceneSubAddTeacher    = "sub_add_teacher"
	sceneSubRemove        = "sub_remove"
	sceneHistoryTeacher   = "history_teacher"
	sceneHistoryWeek      = "history_week"
	sceneSubTestPick      = "sub_test_pick"
	sceneSubTestMode      = "sub_test_mode"
	sceneCallsEditInput   = "calls_edit_input"
	sceneCallsEditReason  = "calls_edit_reason"
	sceneCallsEditConfirm = "calls_edit_confirm"
	sceneAliasAdd         = "alias_add"
	sceneCompareStepA     = "compare_groups_a"
	sceneCompareInput     = "compare_groups_input"
)

type sceneHandler func(ctx context.Context, u *Update, chat *Chat) error

type sceneRoute struct {
	match  func(scene string) bool
	handle sceneHandler
}

func sceneEquals(names ...string) func(string) bool {
	return func(scene string) bool {
		for _, name := range names {
			if scene == name {
				return true
			}
		}
		return false
	}
}

func scenePrefixed(prefixes ...string) func(string) bool {
	return func(scene string) bool {
		for _, prefix := range prefixes {
			if prefix != "" && strings.HasPrefix(scene, prefix) {
				return true
			}
		}
		return false
	}
}

func (b *Bot) buildSceneRoutes() []sceneRoute {
	void := func(fn func(context.Context, *Update, *Chat)) sceneHandler {
		return func(ctx context.Context, u *Update, chat *Chat) error {
			fn(ctx, u, chat)
			return nil
		}
	}

	return []sceneRoute{
		{match: sceneEquals(sceneSetGroup), handle: void(b.handleSetGroup)},
		{match: sceneEquals(sceneSetTeacher), handle: void(b.handleSetTeacher)},
		{match: sceneEquals(sceneSubAddGroup), handle: void(b.handleSubAddGroup)},
		{match: sceneEquals(sceneSubAddTeacher), handle: void(b.handleSubAddTeacher)},
		{match: sceneEquals(sceneSubRemove), handle: void(b.handleSubRemove)},
		{match: sceneEquals(sceneAliasAdd), handle: void(b.handleAliasAdd)},
		{match: sceneEquals(sceneHistoryTeacher), handle: (&historyTeacherScene{bot: b}).Handle},
		{match: sceneEquals(sceneHistoryWeek), handle: (&historyWeekScene{bot: b}).Handle},
		{match: sceneEquals(sceneSubTestPick), handle: (&subTestPickScene{bot: b}).Handle},
		{match: sceneEquals(sceneSubTestMode), handle: (&subTestModeScene{bot: b}).Handle},
		{match: scenePrefixed(sceneSubTestMode + ":"), handle: (&subTestModeScene{bot: b}).Handle},
		{match: sceneEquals(sceneCallsEditInput), handle: (&callsEditInputScene{bot: b}).Handle},
		{match: sceneEquals(sceneCallsEditReason), handle: (&callsEditReasonScene{bot: b}).Handle},
		{match: sceneEquals(sceneCallsEditConfirm), handle: (&callsEditConfirmScene{bot: b}).Handle},
		{match: sceneEquals(sceneCompareStepA), handle: (&compareGroupsStepA{bot: b}).Handle},
		{match: scenePrefixed(sceneCompareInput), handle: (&compareGroupsInputScene{bot: b}).Handle},
		{match: scenePrefixed(sceneGetGroup + ":"), handle: func(ctx context.Context, u *Update, chat *Chat) error {
			kind := strings.TrimPrefix(chat.Scene, sceneGetGroup+":")
			return b.resolveGroupInput(u, chat, strings.TrimSpace(u.Text), kind)
		}},
		{match: scenePrefixed(sceneGetTeacher + ":"), handle: func(ctx context.Context, u *Update, chat *Chat) error {
			kind := strings.TrimPrefix(chat.Scene, sceneGetTeacher+":")
			return b.resolveTeacherInput(u, chat, strings.TrimSpace(u.Text), kind)
		}},
	}
}

func (b *Bot) dispatchInputScene(ctx context.Context, u *Update, chat *Chat) bool {
	for _, route := range b.scenes {
		if !route.match(chat.Scene) {
			continue
		}
		if err := route.handle(ctx, u, chat); err != nil {
			b.log.Error().Err(err).Str("scene", chat.Scene).Msg("scene handler error")
		}
		return true
	}
	return false
}
