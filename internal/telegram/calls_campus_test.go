package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func withCampusVariants(t *testing.T, b *Bot) {
	t.Helper()

	b.cache.SetCallsSiteVariants(t.Context(), []cache.CallsVariant{
		{
			Name: "Казинца",
			Schedule: cache.CallsSchedule{
				Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
				Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
			},
		},
		{
			Name: "Кнорина",
			Schedule: cache.CallsSchedule{
				Weekdays: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
				Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
			},
		},
	})
}

func containsText(rows [][]string, text string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if cell == text {
				return true
			}
		}
	}
	return false
}

func TestCallsScheduleFollowsTheChatCampus(t *testing.T) {
	b, _, c := setupTestBotWithData(t)
	withCampusVariants(t, b)

	c.SetCallsNotify(t.Context(), cache.Schedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		Saturday: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
	}, cache.Schedule{}, "site", "")

	defaultChat := &Chat{}
	if got := b.callsScheduleFor(defaultChat).Weekdays[0][0][0]; got != "08:00" {
		t.Errorf("default campus start = %s", got)
	}

	knorina := &Chat{CallsCampus: "Кнорина"}
	if got := b.callsScheduleFor(knorina).Weekdays[0][0][0]; got != "09:00" {
		t.Errorf("Кнорина start = %s", got)
	}

	unknown := &Chat{CallsCampus: "Нет такого"}
	if got := b.callsScheduleFor(unknown).Weekdays[0][0][0]; got != "08:00" {
		t.Errorf("an unknown campus must fall back to the site schedule, got %s", got)
	}
}

func TestCallsScheduleIgnoresCampusForManualSource(t *testing.T) {
	b, _, c := setupTestBotWithData(t)
	withCampusVariants(t, b)

	calls := c.GetCalls()
	calls.Manual = cache.CallsSource{
		Schedule:  cache.CallsSchedule{Weekdays: [][2][2]string{{{"10:00", "10:45"}, {"10:55", "11:40"}}}},
		UpdatedAt: 1,
	}
	calls.Active = cache.CallsActive{
		Source:   "manual",
		Schedule: calls.Manual.Schedule,
	}
	c.SetCallsFromCache(calls)

	chat := &Chat{CallsCampus: "Кнорина"}
	if got := b.callsScheduleFor(chat).Weekdays[0][0][0]; got != "10:00" {
		t.Errorf("manual source must win over the campus, got %s", got)
	}
}

func TestCallsCampusKeyboardOffersEveryCampus(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)

	plain := replyRows(b.replyCallsSettings(&Chat{}, false))
	if containsText(plain, campusButtonPrefix+"Казинца") {
		t.Error("no campus buttons before the site reports several schedules")
	}

	withCampusVariants(t, b)
	rows := replyRows(b.replyCallsSettings(&Chat{CallsCampus: "Кнорина"}, false))

	if !containsText(rows, "✅ "+campusButtonPrefix+"Кнорина") {
		t.Errorf("the selected campus must be checked: %v", rows)
	}
	if !containsText(rows, campusButtonPrefix+"Казинца") {
		t.Errorf("the other campus must be offered: %v", rows)
	}
	if !containsText(rows, campusButtonPrefix+campusAutoLabel) {
		t.Errorf("an explicit choice must be resettable: %v", rows)
	}
}

func TestCallsCampusTextCommandStoresTheChoice(t *testing.T) {
	b, repo, _ := setupTestBotWithData(t)
	withCampusVariants(t, b)

	handler := &callsCampusTextCmd{bot: b}
	if !handler.MatchText(campusButtonText("Кнорина", false)) {
		t.Fatal("the campus button must be matched")
	}
	if !handler.MatchText(campusButtonText("Кнорина", true)) {
		t.Fatal("the checked campus button must be matched")
	}
	if handler.MatchText("Источник: сайт") {
		t.Fatal("other calls buttons must not match")
	}

	chat, err := repo.FindOrCreate("telegram", 777)
	if err != nil {
		t.Fatal(err)
	}

	if label := campusButtonLabel(campusButtonText("Кнорина", true)); label != "Кнорина" {
		t.Errorf("label = %q", label)
	}

	chat.CallsCampus = "Кнорина"
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.FindOrCreate("telegram", 777)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CallsCampus != "Кнорина" {
		t.Errorf("campus was not persisted: %q", stored.CallsCampus)
	}
}

func TestE2E_CampusButtonStoresTheChoice(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	withCampusVariants(t, b)

	userID := int64(4100)
	open := makeUpdate(userID, "🕐 Звонки: управление")
	open.Bot = b
	b.handleMessageText(context.Background(), open)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsCalls {
		t.Fatalf("scene = %q", saved.Scene)
	}

	u := makeUpdate(userID, campusButtonText("Кнорина", false))
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.CallsCampus != "Кнорина" {
		t.Errorf("campus was not saved: %q", saved.CallsCampus)
	}

	auto := makeUpdate(userID, campusButtonPrefix+campusAutoLabel)
	auto.Bot = b
	b.handleMessageText(context.Background(), auto)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.CallsCampus != "" {
		t.Errorf("the auto entry must reset the campus, got %q", saved.CallsCampus)
	}
}

func TestCallsMenuReportsAMissingSiteDate(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)

	calls := b.cache.GetCalls()
	calls.Active.Source = "site"
	calls.Active.Schedule = cache.CallsSchedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
	}
	calls.Site.UpdatedAt = time.Date(2026, 10, 5, 8, 30, 0, 0, time.Local).UnixMilli()
	calls.Site.UpdatedAtRaw = ""
	b.cache.SetCallsFromCache(calls)

	text := b.callsMenuText(&Chat{}, true)
	if !strings.Contains(text, b.loc("call_site_date_missing")) {
		t.Errorf("the calls menu must say the site date is missing, got %q", text)
	}
	if !strings.Contains(text, "Последний разбор: 05.10.2026 08:30") {
		t.Errorf("the calls menu must fall back to the parse moment, got %q", text)
	}
}

func TestCallsShowAddsAFreshnessLineForUsers(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chat, _ := repo.FindOrCreate("telegram", 4242)
	chat.Mode = ModeStudent
	chat.Group = "63ТП"
	repo.Save(chat)

	calls := b.cache.GetCalls()
	calls.Site.UpdatedAt = b.nowTime().Add(-3 * time.Hour).UnixMilli()
	calls.Site.UpdatedAtRaw = ""
	b.cache.SetCallsFromCache(calls)

	b.showCalls(&Update{Bot: b, ChatID: 4242, UserID: 4242}, chat, false)

	text, _ := lastDeliveredMessage(t, caller)
	if !strings.Contains(text, "Звонки получены:") || !strings.Contains(text, "3 ч. назад") {
		t.Errorf("the calls view must show how old the bell schedule is, got %q", text)
	}
}

func TestCallsShowPrefersTheSiteDateInTheFreshnessLine(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4243)
	chat, _ := repo.FindOrCreate("telegram", 4243)
	chat.Mode = ModeStudent
	chat.Group = "63ТП"
	repo.Save(chat)

	calls := b.cache.GetCalls()
	calls.Site.UpdatedAt = b.nowTime().Add(-72 * time.Hour).UnixMilli()
	calls.Site.UpdatedAtRaw = "05.09.2026"
	b.cache.SetCallsFromCache(calls)

	b.showCalls(&Update{Bot: b, ChatID: 4243, UserID: 4243}, chat, false)

	text, _ := lastDeliveredMessage(t, caller)
	if !strings.Contains(text, "Звонки обновлены на сайте: 05.09.2026") {
		t.Errorf("the calls view must show the site date when it exists, got %q", text)
	}
}

func TestCallsCampusConfirmationMentionsTheCampus(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	withCampusVariants(t, b)

	text := b.campusConfirmation(&Chat{CallsCampus: "Кнорина"})
	if !strings.Contains(text, "Кнорина") {
		t.Errorf("confirmation = %q", text)
	}
	if !strings.Contains(strings.ToLower(b.callsMenuText(&Chat{CallsCampus: "Кнорина"}, false)), "кнорина") {
		t.Error("the calls menu should mention the selected campus")
	}
}
