package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func TestE2E_CallsSettingsReplyMenuLayout(t *testing.T) {
	b, _ := setupE2EBot(t, 999)
	chat := &Chat{Mode: ModeStudent}

	admin := flattenReplyKeyboardTexts(b.replyCallsSettings(chat, true))
	for _, want := range []string{"📊 Показать", "✅ Обновить с сайта", "✏️ Изменить вручную", "Источник: сайт", "Источник: вручную", "Источник: конфиг", "Источник: авто", "Меню настроек", "Главное меню"} {
		if !strings.Contains(admin, want) {
			t.Errorf("admin calls menu missing %q, got %q", want, admin)
		}
	}

	guest := flattenReplyKeyboardTexts(b.replyCallsSettings(chat, false))
	if !strings.Contains(guest, "📊 Показать") {
		t.Errorf("calls menu should always show Показать, got %q", guest)
	}
	for _, unwanted := range []string{"Обновить с сайта", "Изменить вручную", "Источник: сайт"} {
		if strings.Contains(guest, unwanted) {
			t.Errorf("non-admin calls menu must not contain %q, got %q", unwanted, guest)
		}
	}
}

func TestE2E_CallsSettingsMenuTextMatchesTS(t *testing.T) {
	b, _ := setupE2EBot(t, 999)
	chat := &Chat{Mode: ModeStudent}

	text := b.callsMenuText(chat, true)
	if !strings.HasPrefix(text, "Управление расписанием звонков.") {
		t.Errorf("calls menu header mismatch: %q", text)
	}
	if !strings.Contains(text, "Источник: авто (сейчас: сайт)") {
		t.Errorf("auto source line mismatch: %q", text)
	}
	if !strings.Contains(text, "Обновлено на сайте:") {
		t.Errorf("expected site updated line, got %q", text)
	}
	if strings.Contains(text, "<b>") {
		t.Errorf("calls menu must not contain html bold: %q", text)
	}
}

func TestE2E_CallsSettingsSceneAndTextCommands(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	userID := int64(3001)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Scene = sceneSettings
	repo.Save(chat)

	u := makeUpdate(userID, "🗓️ Управление расписаниями")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsSchedules {
		t.Fatalf("scene after schedules command: got %q", saved.Scene)
	}

	u = makeUpdate(userID, "🕐 Звонки: управление")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsCalls {
		t.Fatalf("scene after calls command: got %q", saved.Scene)
	}
}

func TestE2E_CallsSourceTextCommandsSwitchOverride(t *testing.T) {
	b, repo := setupE2EBot(t, 3002)
	userID := int64(3002)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Scene = sceneSettingsCalls
	repo.Save(chat)

	manual := &callsSettingsTextCmd{bot: b, kind: "source_manual"}
	if !manual.MatchText("Источник: вручную") || !manual.MatchText("✅ Источник: вручную") {
		t.Error("source_manual must match both plain and checked texts")
	}
	u := makeUpdate(userID, "Источник: вручную")
	u.Bot = b
	if err := manual.Handler(context.Background(), u); err != nil {
		t.Fatalf("source_manual: %v", err)
	}

	calls := b.cache.GetCalls()
	if calls.OverrideSource != "manual" {
		t.Errorf("override source: got %q", calls.OverrideSource)
	}
	if calls.Active.Source == "site" {
		t.Errorf("active source should leave site when overridden, got %q", calls.Active.Source)
	}

	auto := &callsSettingsTextCmd{bot: b, kind: "source_auto"}
	if !auto.MatchText("✅ Источник: авто") {
		t.Error("source_auto must match checked text")
	}
	if err := auto.Handler(context.Background(), u); err != nil {
		t.Fatalf("source_auto: %v", err)
	}

	calls = b.cache.GetCalls()
	if calls.OverrideSource != "" {
		t.Errorf("override should clear, got %q", calls.OverrideSource)
	}
	if calls.Active.Source != "site" {
		t.Errorf("active should fall back to site, got %q", calls.Active.Source)
	}
}

func TestE2E_CallsSourceConfigUsesTimetable(t *testing.T) {
	b, repo := setupE2EBot(t, 3003)
	userID := int64(3003)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSettingsCalls
	repo.Save(chat)

	cfgSource := &callsSettingsTextCmd{bot: b, kind: "source_config"}
	u := makeUpdate(userID, "Источник: конфиг")
	u.Bot = b
	if err := cfgSource.Handler(context.Background(), u); err != nil {
		t.Fatalf("source_config: %v", err)
	}

	calls := b.cache.GetCalls()
	if calls.Active.Source != "config" {
		t.Fatalf("active source: got %q", calls.Active.Source)
	}
	if len(calls.Active.Schedule.Weekdays) != len(b.cfg.Timetable.Weekdays) {
		t.Errorf("config schedule not applied: %d", len(calls.Active.Schedule.Weekdays))
	}
}

func TestE2E_CallsSourceRequiresAdmin(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	userID := int64(3004)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSettingsCalls
	repo.Save(chat)

	refresh := &callsSettingsTextCmd{bot: b, kind: "refresh"}
	u := makeUpdate(userID, "✅ Обновить с сайта")
	u.Bot = b
	if err := refresh.Handler(context.Background(), u); err != nil {
		t.Fatalf("refresh for non-admin: %v", err)
	}

	if b.cache.GetCalls().OverrideSource != "" {
		t.Error("non-admin must not change calls state")
	}
}

func TestE2E_CallsManualEditThreeSteps(t *testing.T) {
	b, repo := setupE2EBot(t, 3005)
	userID := int64(3005)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Scene = sceneSettingsCalls
	repo.Save(chat)

	edit := &callsSettingsTextCmd{bot: b, kind: "edit"}
	u := makeUpdate(userID, "✏️ Изменить вручную")
	u.Bot = b
	if err := edit.Handler(context.Background(), u); err != nil {
		t.Fatalf("edit step 1: %v", err)
	}

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != "calls_edit_input" {
		t.Fatalf("scene after edit: got %q", saved.Scene)
	}

	schedule := "Будни\n1 08:30 09:15 09:25 10:10\n2 10:20 11:05 11:15 12:00\nСуббота\n1 09:00 09:45 09:55 10:40"
	u = makeUpdate(userID, schedule)
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.Scene != "calls_edit_reason" {
		t.Fatalf("scene after schedule input: got %q", saved.Scene)
	}
	if saved.CallsEditInput == "" {
		t.Error("schedule input should be stored on the chat")
	}

	u = makeUpdate(userID, "пропустить")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.Scene != "calls_edit_confirm" {
		t.Fatalf("scene after reason: got %q", saved.Scene)
	}

	u = makeUpdate(userID, "Не отправлять")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Fatalf("scene should clear after confirm, got %q", saved.Scene)
	}
	if saved.CallsEditInput != "" || saved.CallsEditReason != "" {
		t.Error("edit draft fields should be cleared")
	}

	calls := b.cache.GetCalls()
	if calls.Active.Source != "manual" {
		t.Errorf("manual edit should activate manual source, got %q", calls.Active.Source)
	}
	if calls.ManualReason != "" {
		t.Errorf("skipped reason should stay empty, got %q", calls.ManualReason)
	}
	if len(calls.Active.Schedule.Weekdays) != 2 {
		t.Errorf("expected 2 weekday slots, got %d", len(calls.Active.Schedule.Weekdays))
	}
	if len(calls.Active.Schedule.Saturday) != 1 {
		t.Errorf("expected 1 saturday slot, got %d", len(calls.Active.Schedule.Saturday))
	}
}

func TestE2E_CallsManualEditRejectsBadFormat(t *testing.T) {
	b, repo := setupE2EBot(t, 3006)
	userID := int64(3006)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "calls_edit_input"
	repo.Save(chat)

	u := makeUpdate(userID, "не расписание")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != "calls_edit_input" {
		t.Errorf("scene should stay on input after bad format, got %q", saved.Scene)
	}
}

func TestE2E_SubscriptionsTestTwoStepFlow(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(3007)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Scene = sceneSettings
	repo.Save(chat)

	if _, err := repo.AddSubscription(userID, "group", "100"); err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	testCmd := &subscriptionsTestCmd{bot: b}
	u := makeUpdate(userID, "🧪 Проверить")
	u.Bot = b
	if err := testCmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("test command: %v", err)
	}

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != "sub_test_pick" {
		t.Fatalf("scene after test command: got %q", saved.Scene)
	}

	u = makeUpdate(userID, "1")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if !strings.HasPrefix(saved.Scene, "sub_test_mode:group:100") {
		t.Fatalf("scene after subscription pick: got %q", saved.Scene)
	}

	u = makeUpdate(userID, "3")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.Scene != "" {
		t.Errorf("scene should clear after mode pick, got %q", saved.Scene)
	}
}

func TestE2E_SubscriptionsTestRejectsUnknownNumber(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(3008)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = "sub_test_pick"
	repo.Save(chat)

	if _, err := repo.AddSubscription(userID, "group", "100"); err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	u := makeUpdate(userID, "42")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != "sub_test_pick" {
		t.Errorf("scene should stay on pick after bad number, got %q", saved.Scene)
	}
}

func TestE2E_NeedUpdateButtonsResetsOnNextMessage(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(3009)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Mode = ModeStudent
	chat.Group = "100"
	chat.Accepted = true
	chat.NeedUpdateButtons = true
	chat.Scene = sceneSettings
	repo.Save(chat)

	u := makeUpdate(userID, "абракадабра")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.NeedUpdateButtons {
		t.Error("needUpdateButtons should be cleared after a message")
	}
	if saved.Scene != "" {
		t.Errorf("scene should be reset together with the keyboard, got %q", saved.Scene)
	}
}

func TestE2E_RequireNewButtonsSetsFlag(t *testing.T) {
	b, repo := setupE2EBot(t, 999)

	chat, _ := repo.FindOrCreate("telegram", int64(3010))
	chat.Accepted = true
	repo.Save(chat)

	cmd := &requireNewButtonsCmd{bot: b}
	u := makeUpdate(999, "/requireNewButtons")
	u.Bot = b
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("requireNewButtons: %v", err)
	}

	saved, _ := repo.FindOrCreate("telegram", int64(3010))
	if !saved.NeedUpdateButtons {
		t.Error("requireNewButtons should set the flag for accepted chats")
	}
}

func TestE2E_AliasMenuUsesReplyKeyboard(t *testing.T) {
	b, repo := setupE2EBot(t)
	userID := int64(3011)

	chat, _ := repo.FindOrCreate("telegram", userID)
	chat.Scene = sceneSettings
	repo.Save(chat)

	u := makeUpdate(userID, "Алиасы")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsAlias {
		t.Errorf("scene after alias menu: got %q", saved.Scene)
	}

	kb := b.replySettingsAliases()
	texts := flattenReplyKeyboardTexts(kb)
	for _, want := range []string{"Список", "Добавить", "Удалить", "Отчистить все", "Меню настроек", "Главное меню"} {
		if !strings.Contains(texts, want) {
			t.Errorf("alias reply menu missing %q", want)
		}
	}
}

func TestE2E_CallsMenuHasNoHtmlBoldInDisplay(t *testing.T) {
	b, _ := setupE2EBot(t)
	chat := &Chat{Mode: ModeStudent, Group: "100"}

	kb := b.replyCallsSettings(chat, true)
	if kb == nil || len(kb.Keyboard) < 4 {
		t.Fatalf("calls reply menu rows: %d", len(kb.Keyboard))
	}

	calls := b.cache.GetCalls()
	if calls.Active.Source != "site" {
		t.Errorf("initial active source should be site, got %q", calls.Active.Source)
	}
	if calls.OverrideSource != "" {
		t.Errorf("initial override should be empty, got %q", calls.OverrideSource)
	}

	var _ cache.CallsCache = calls
}
